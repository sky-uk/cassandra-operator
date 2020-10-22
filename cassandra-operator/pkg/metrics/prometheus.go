package metrics

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreV1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
)

type clusterMetrics struct {
	cassandraNodeStatusGauge *prometheus.GaugeVec
	clusterSizeGauge         *prometheus.GaugeVec
}

type clusterTopology struct {
	nodesToRack map[string]string
}

func (t *clusterTopology) nodeCount() float64 {
	return float64(len(t.nodesToRack))
}

// PrometheusMetrics reports on the status of cluster nodes and exposes the information to Prometheus
type PrometheusMetrics struct {
	podsGetter                coreV1.PodsGetter
	gatherer                  Gatherer
	clustersMetrics           *clusterMetrics
	lastKnownClustersTopology *clusterTopologyMap
}

type clusterTopologyMap struct {
	topologyMap *sync.Map
}

func (m *clusterTopologyMap) Get(key string) (*clusterTopology, bool) {
	if raw, ok := m.topologyMap.Load(key); ok {
		ct := raw.(*clusterTopology)
		return ct, true
	}

	return nil, false
}

func (m *clusterTopologyMap) Set(key string, value *clusterTopology) {
	m.topologyMap.Store(key, value)
}

func (m *clusterTopologyMap) Delete(key string) {
	m.topologyMap.Delete(key)
}

type randomisingJolokiaURLProvider struct {
	podsGetter coreV1.PodsGetter
	random     *rand.Rand
}

// NewMetrics creates a new Prometheus metric reporter
func NewMetrics(podsGetter coreV1.PodsGetter, config *Config) *PrometheusMetrics {
	return &PrometheusMetrics{
		podsGetter:                podsGetter,
		gatherer:                  NewGatherer(&randomisingJolokiaURLProvider{podsGetter: podsGetter, random: rand.New(rand.NewSource(time.Now().UnixNano()))}, config),
		clustersMetrics:           registerMetrics(),
		lastKnownClustersTopology: &clusterTopologyMap{&sync.Map{}},
	}
}

// DeleteMetrics stops reporting metrics for the given cluster
func (m *PrometheusMetrics) DeleteMetrics(cluster *cluster.Cluster) {
	if !m.clustersMetrics.clusterSizeGauge.Delete(map[string]string{"cluster": cluster.Name(), "namespace": cluster.Namespace()}) {
		log.Warnf("Unable to delete cluster_size metrics for cluster %s", cluster.QualifiedName())
	}

	var clusterTopology *clusterTopology
	var ok bool
	if clusterTopology, ok = m.lastKnownClustersTopology.Get(cluster.QualifiedName()); !ok {
		log.Warnf("No last known cluster topology for cluster %s. Perhaps no metrics have ever been collected.", cluster.QualifiedName())
		return
	}
	for podName, rackName := range clusterTopology.nodesToRack {
		m.RemoveNodeFromMetrics(cluster, podName, rackName)
	}
	m.lastKnownClustersTopology.Delete(cluster.QualifiedName())
}

// RemoveNodeFromMetrics stops reporting metrics for the given node
func (m *PrometheusMetrics) RemoveNodeFromMetrics(cluster *cluster.Cluster, podName, rackName string) {
	log.Infof("Removing node cluster:%s, pod:%s, rack:%s from metrics", cluster.QualifiedName(), podName, rackName)
	for _, labelPair := range allLabelPairs {
		deleted := m.clustersMetrics.cassandraNodeStatusGauge.Delete(map[string]string{
			"cluster":   cluster.Name(),
			"namespace": cluster.Namespace(),
			"rack":      rackName,
			"pod":       podName,
			"liveness":  labelPair.liveness,
			"state":     labelPair.state,
		})
		if !deleted {
			log.Warnf("Unable to delete node status metrics for cluster %s, rack: %s, pod: %s, node status: %s", cluster.QualifiedName(), rackName, podName, labelPair)
		}
	}
}

// UpdateMetrics updates metrics for the given cluster
func (m *PrometheusMetrics) UpdateMetrics(cluster *cluster.Cluster) {
	podIPMapper, err := m.podsInCluster(cluster)
	if err != nil {
		log.Errorf("Unable to retrieve pod list for cluster %s: %v", cluster.QualifiedName(), err)
		return
	}

	clusterStatus, err := m.gatherer.GatherMetricsFor(cluster)
	if err != nil {
		log.Errorf("Unable to gather metrics for cluster %s: %v", cluster.QualifiedName(), err)
		return
	}

	podIPToNodeStatus := transformClusterStatus(clusterStatus)

	clusterLastKnownTopology := &clusterTopology{nodesToRack: make(map[string]string)}
	for podIP, nodeStatus := range podIPToNodeStatus {
		podIPMapper.withPodNameDoOrError(podIP, func(podName string) {
			var rack string
			var ok bool
			if rack, ok = clusterStatus.nodeRacks[podIP]; !ok {
				rack = "unknown"
			}
			clusterLastKnownTopology.nodesToRack[podName] = rack
			m.updateNodeStatus(cluster, rack, podName, nodeStatus)
		})
	}

	m.lastKnownClustersTopology.Set(cluster.QualifiedName(), clusterLastKnownTopology)
	m.clustersMetrics.clusterSizeGauge.WithLabelValues(cluster.Name(), cluster.Namespace()).Set(clusterLastKnownTopology.nodeCount())
}

func (m *PrometheusMetrics) updateNodeStatus(cluster *cluster.Cluster, rack string, podName string, nodeStatus *nodeStatus) {
	m.clustersMetrics.cassandraNodeStatusGauge.WithLabelValues(cluster.Name(), cluster.Namespace(), rack, podName, nodeStatus.livenessLabel(), nodeStatus.stateLabel()).Set(1)
	for _, ul := range nodeStatus.unapplicableLabelPairs() {
		m.clustersMetrics.cassandraNodeStatusGauge.WithLabelValues(cluster.Name(), cluster.Namespace(), rack, podName, ul.liveness, ul.state).Set(0)
	}
}

func registerMetrics() *clusterMetrics {
	cassandraNodeStatusGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cassandra_node_status",
			Help: "Records 1 if a node is in the given status, and 0 otherwise. Possible values for 'liveness' label are: 'up' and 'down'. Possible values for 'state' label are: 'normal', 'leaving', 'joining' and 'moving'.",
		},
		[]string{"cluster", "namespace", "rack", "pod", "liveness", "state"},
	)
	clusterSizeGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cassandra_cluster_size",
			Help: "Total number of nodes in the Cassandra cluster",
		},
		[]string{"cluster", "namespace"},
	)
	prometheus.MustRegister(cassandraNodeStatusGauge, clusterSizeGauge)
	return &clusterMetrics{cassandraNodeStatusGauge: cassandraNodeStatusGauge, clusterSizeGauge: clusterSizeGauge}
}

func (m *PrometheusMetrics) podsInCluster(cluster *cluster.Cluster) (*podIPMapper, error) {
	podList, err := m.podsGetter.Pods(cluster.Namespace()).List(metaV1.ListOptions{LabelSelector: cluster.CassandraPodSelector()})
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve pods for cluster %s, %v", cluster.QualifiedName(), err)
	}

	podIPToName := map[string]string{}
	for _, pod := range podList.Items {
		podIPToName[pod.Status.PodIP] = pod.Name
	}
	return &podIPMapper{cluster: cluster, podIPToName: podIPToName}, nil
}

type podIPMapper struct {
	cluster     *cluster.Cluster
	podIPToName map[string]string
}

func (p *podIPMapper) withPodNameDoOrError(podIP string, action func(string)) {
	podName, ok := p.podIPToName[podIP]
	if !ok {
		log.Warnf("Unable to find corresponding pod name for pod ip: %s for cluster %s", podIP, p.cluster.QualifiedName())
	} else {
		action(podName)
	}
}

func (u *randomisingJolokiaURLProvider) URLFor(cluster *cluster.Cluster) string {
	var jolokiaHostname string
	podsWithIPAddresses, err := u.podsWithIPAddresses(cluster)

	if err != nil {
		jolokiaHostname = cluster.Definition().ServiceName()
		log.Infof("Unable to retrieve list of pods for cluster %s. Falling back to the cluster service name for jolokia url. Error: %v", cluster.QualifiedName(), err)
	} else if len(podsWithIPAddresses) == 0 {
		jolokiaHostname = cluster.Definition().ServiceName()
		log.Infof("No pods with IP addresses found for cluster %s. Falling back to the cluster service name for jolokia url.", cluster.QualifiedName())
	} else {
		jolokiaHostname = podsWithIPAddresses[u.random.Intn(len(podsWithIPAddresses))].Status.PodIP
	}

	return fmt.Sprintf("http://%s:7777", jolokiaHostname)
}

func (u *randomisingJolokiaURLProvider) podsWithIPAddresses(cluster *cluster.Cluster) ([]v1.Pod, error) {
	podList, err := u.podsGetter.Pods(cluster.Namespace()).List(metaV1.ListOptions{LabelSelector: fmt.Sprintf("app.kubernetes.io/instance=%s", cluster.Name())})
	if err != nil {
		return nil, err
	}

	podsWithIPAddresses := []v1.Pod{}
	for _, pod := range podList.Items {
		if pod.Status.PodIP != "" {
			podsWithIPAddresses = append(podsWithIPAddresses, pod)
		}
	}
	return podsWithIPAddresses, nil
}
