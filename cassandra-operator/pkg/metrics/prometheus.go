package metrics

import (
	"fmt"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"math/rand"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreV1 "k8s.io/client-go/kubernetes/typed/core/v1"

	pkgcluster "github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
)

type clusterMetrics struct {
	cassandraNodeStatusGauge  *prometheus.GaugeVec
	clusterSizeGauge          *prometheus.GaugeVec
	clusterValidationFailures *prometheus.CounterVec
}

type clusterTopology struct {
	nodesToRack map[string]string
}

func (t *clusterTopology) nodeCount() float64 {
	return float64(len(t.nodesToRack))
}

// ClusterMetricsReporter allows us to increment and initialise metrics via the PrometheusMetrics and Mocks
type ClusterMetricsReporter interface {
	IncrementFailedValidationMetric(cassandra *v1alpha1.Cassandra)
	InitialiseFailedValidationMetric(cassandra *v1alpha1.Cassandra)
	UpdateMetrics(cassandra *v1alpha1.Cassandra)
	DeleteMetrics(cassandra *v1alpha1.Cassandra)
	RemoveNodeFromMetrics(cassandra *v1alpha1.Cassandra, podName, rackName string)
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
func (m *PrometheusMetrics) DeleteMetrics(cassandra *v1alpha1.Cassandra) {
	if !m.clustersMetrics.clusterSizeGauge.Delete(map[string]string{"cluster": cassandra.Name, "namespace": cassandra.Namespace}) {
		log.Warnf("Unable to delete cluster_size metrics for cluster %s", cassandra.QualifiedName())
	}

	if !m.clustersMetrics.clusterValidationFailures.Delete(map[string]string{"cluster": cassandra.Name, "namespace": cassandra.Namespace}) {
		log.Warnf("Unable to delete cassandra_failed_validation_count metrics for cluster %s", cassandra.QualifiedName())
	}

	var clusterTopology *clusterTopology
	var ok bool
	if clusterTopology, ok = m.lastKnownClustersTopology.Get(cassandra.QualifiedName()); !ok {
		log.Warnf("No last known cluster topology for cluster %s. Perhaps no metrics have ever been collected.", cassandra.QualifiedName())
		return
	}
	for podName, rackName := range clusterTopology.nodesToRack {
		m.RemoveNodeFromMetrics(cassandra, podName, rackName)
	}
	m.lastKnownClustersTopology.Delete(cassandra.QualifiedName())
}

// RemoveNodeFromMetrics stops reporting metrics for the given node
func (m *PrometheusMetrics) RemoveNodeFromMetrics(cassandra *v1alpha1.Cassandra, podName, rackName string) {
	log.Infof("Removing node cluster:%s, pod:%s, rack:%s from metrics", cassandra.QualifiedName(), podName, rackName)
	for _, labelPair := range allLabelPairs {
		deleted := m.clustersMetrics.cassandraNodeStatusGauge.Delete(map[string]string{
			"cluster":   cassandra.Name,
			"namespace": cassandra.Namespace,
			"rack":      rackName,
			"pod":       podName,
			"liveness":  labelPair.liveness,
			"state":     labelPair.state,
		})
		if !deleted {
			log.Warnf("Unable to delete node status metrics for cluster %s, rack: %s, pod: %s, node status: %s", cassandra.QualifiedName(), rackName, podName, labelPair)
		}
	}
}

// UpdateMetrics updates metrics for the given cluster
func (m *PrometheusMetrics) UpdateMetrics(cassandra *v1alpha1.Cassandra) {
	podIPMapper, err := m.podsInCluster(cassandra)
	if err != nil {
		log.Errorf("Unable to retrieve pod list for cluster %s: %v", cassandra.QualifiedName(), err)
		return
	}
	clusterFromCassandra := pkgcluster.New(cassandra)
	clusterStatus, err := m.gatherer.GatherMetricsFor(clusterFromCassandra)
	if err != nil {
		log.Errorf("Unable to gather metrics for cluster %s: %v", cassandra.QualifiedName(), err)
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
			m.updateNodeStatus(cassandra, rack, podName, nodeStatus)
		})
	}

	m.lastKnownClustersTopology.Set(cassandra.QualifiedName(), clusterLastKnownTopology)
	m.clustersMetrics.clusterSizeGauge.WithLabelValues(cassandra.Name, cassandra.Namespace).Set(clusterLastKnownTopology.nodeCount())
}

// IncrementFailedValidationMetric increment failed validation metrics metrics for the given cluster
func (m *PrometheusMetrics) IncrementFailedValidationMetric(cassandra *v1alpha1.Cassandra) {
	m.clustersMetrics.clusterValidationFailures.WithLabelValues(cassandra.Name, cassandra.Namespace).Inc()
}

// InitialiseFailedValidationMetric increment failed validation metrics metrics for the given cluster
func (m *PrometheusMetrics) InitialiseFailedValidationMetric(cassandra *v1alpha1.Cassandra) {
	m.clustersMetrics.clusterValidationFailures.WithLabelValues(cassandra.Name, cassandra.Namespace).Add(0)
}

func (m *PrometheusMetrics) updateNodeStatus(cassandra *v1alpha1.Cassandra, rack string, podName string, nodeStatus *nodeStatus) {
	m.clustersMetrics.cassandraNodeStatusGauge.WithLabelValues(cassandra.Name, cassandra.Namespace, rack, podName, nodeStatus.livenessLabel(), nodeStatus.stateLabel()).Set(1)
	for _, ul := range nodeStatus.unapplicableLabelPairs() {
		m.clustersMetrics.cassandraNodeStatusGauge.WithLabelValues(cassandra.Name, cassandra.Namespace, rack, podName, ul.liveness, ul.state).Set(0)
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
	clusterValidationFailures := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cassandra_failed_validation_total",
			Help: "Total number of validation failures for a Cassandra cluster",
		},
		[]string{"cluster", "namespace"},
	)
	prometheus.MustRegister(cassandraNodeStatusGauge, clusterSizeGauge, clusterValidationFailures)
	return &clusterMetrics{cassandraNodeStatusGauge: cassandraNodeStatusGauge, clusterSizeGauge: clusterSizeGauge, clusterValidationFailures: clusterValidationFailures}
}

func (m *PrometheusMetrics) podsInCluster(cassandra *v1alpha1.Cassandra) (*podIPMapper, error) {
	clusterFromCassandra := pkgcluster.New(cassandra)
	podList, err := m.podsGetter.Pods(cassandra.Namespace).List(metaV1.ListOptions{LabelSelector: clusterFromCassandra.CassandraPodSelector()})
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve pods for cluster %s, %v", cassandra.QualifiedName(), err)
	}

	podIPToName := map[string]string{}
	for _, pod := range podList.Items {
		podIPToName[pod.Status.PodIP] = pod.Name
	}
	return &podIPMapper{cassandra: cassandra, podIPToName: podIPToName}, nil
}

type podIPMapper struct {
	cassandra   *v1alpha1.Cassandra
	podIPToName map[string]string
}

func (p *podIPMapper) withPodNameDoOrError(podIP string, action func(string)) {
	podName, ok := p.podIPToName[podIP]
	if !ok {
		log.Warnf("Unable to find corresponding pod name for pod ip: %s for cluster %s", podIP, p.cassandra.QualifiedName())
	} else {
		action(podName)
	}
}

func (u *randomisingJolokiaURLProvider) URLFor(cluster *pkgcluster.Cluster) string {
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

func (u *randomisingJolokiaURLProvider) podsWithIPAddresses(cluster *pkgcluster.Cluster) ([]v1.Pod, error) {
	podList, err := u.podsGetter.Pods(cluster.Namespace()).List(metaV1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", pkgcluster.ApplicationInstanceLabel, cluster.QualifiedName())})
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
