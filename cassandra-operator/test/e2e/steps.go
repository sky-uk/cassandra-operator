package e2e

import (
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/ptr"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"time"
)

type ClusterBuilder struct {
	clusterName         string
	racks               []v1alpha1.Rack
	extraConfigFile     *ExtraConfigFile
	useEmptyDir         bool
	clusterSpec         *v1alpha1.CassandraSpec
	withoutCustomConfig bool
	snapshot            *v1alpha1.Snapshot
}

func AClusterWithName(clusterName string) *ClusterBuilder {
	return &ClusterBuilder{clusterName: clusterName}
}

func (c *ClusterBuilder) AndRacks(racks []v1alpha1.Rack) *ClusterBuilder {
	c.racks = racks
	return c
}

func (c *ClusterBuilder) WithoutRacks() *ClusterBuilder {
	return c.AndRacks([]v1alpha1.Rack{})
}

func (c *ClusterBuilder) AndCustomConfig(extraConfigFile *ExtraConfigFile) *ClusterBuilder {
	c.extraConfigFile = extraConfigFile
	return c
}

func (c *ClusterBuilder) UsingEmptyDir() *ClusterBuilder {
	c.useEmptyDir = true
	return c
}

func (c *ClusterBuilder) AndClusterSpec(clusterSpec *v1alpha1.CassandraSpec) *ClusterBuilder {
	c.clusterSpec = clusterSpec
	return c
}

func (c *ClusterBuilder) WithoutCustomConfig() *ClusterBuilder {
	c.withoutCustomConfig = true
	return c
}

func (c *ClusterBuilder) AndScheduledSnapshot(snapshot *v1alpha1.Snapshot) *ClusterBuilder {
	c.snapshot = snapshot
	return c
}

func (c *ClusterBuilder) IsDefined() {
	if c.clusterSpec == nil {
		c.clusterSpec = clusterDefaultSpec()
	}

	c.clusterSpec.Racks = c.racks
	c.clusterSpec.Snapshot = c.snapshot

	if c.useEmptyDir {
		c.clusterSpec.UseEmptyDir = ptr.Bool(true)
		// void values that do not apply
		c.clusterSpec.Pod.StorageSize = resource.MustParse("0")
	}

	if !c.withoutCustomConfig {
		_, err := customCassandraConfigMap(Namespace, c.clusterName, c.extraConfigFile)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}
	_, err := cassandraResource(Namespace, c.clusterName, c.clusterSpec)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
}

func (c *ClusterBuilder) Exists() {
	c.IsDefined()
	EventuallyClusterIsCreatedWithRacks(Namespace, c.clusterName, c.racks)
	log.Infof("Created cluster %s", c.clusterName)
}

func TheClusterIsDeleted(clusterName string) {
	deleteClusterDefinitionsWatchedByOperator(Namespace, clusterName)
	deleteCassandraCustomConfigurationConfigMap(Namespace, clusterName)
	log.Infof("Deleted cluster definition and configmap for cluster %s", clusterName)
}

func TheClusterPodSpecAreChangedTo(namespace, clusterName string, podSpec v1alpha1.Pod) {
	mutateCassandraSpec(namespace, clusterName, func(spec *v1alpha1.CassandraSpec) {
		spec.Pod.CPU = podSpec.CPU
		spec.Pod.Memory = podSpec.Memory
		spec.Pod.LivenessProbe = podSpec.LivenessProbe
		spec.Pod.ReadinessProbe = podSpec.ReadinessProbe
	})
	log.Infof("Updated pod spec for cluster %s", clusterName)
}

func TheImageImmutablePropertyIsChangedTo(namespace, clusterName, imageName string) {
	mutateCassandraSpec(namespace, clusterName, func(spec *v1alpha1.CassandraSpec) {
		spec.Pod.Image = &imageName
	})
	log.Infof("Updated pod image for cluster %s", clusterName)
}

func TheRackReplicationIsChangedTo(namespace, clusterName, rackName string, replicas int) {
	mutateCassandraSpec(namespace, clusterName, func(spec *v1alpha1.CassandraSpec) {
		for i := range spec.Racks {
			if spec.Racks[i].Name == rackName {
				spec.Racks[i].Replicas = int32(replicas)
			}
		}
	})
	log.Infof("Updated rack replication for cluster %s", clusterName)
}

func TheCustomConfigIsAddedForCluster(namespace, clusterName string, extraConfigFile *ExtraConfigFile) {
	_, err := customCassandraConfigMap(namespace, clusterName, extraConfigFile)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	log.Infof("Added custom config for cluster %s", clusterName)
}

func TheCustomConfigIsDeletedForCluster(namespace, clusterName string) {
	cmClient := KubeClientset.CoreV1().ConfigMaps(namespace)
	err := cmClient.Delete(fmt.Sprintf("%s-config", clusterName), metaV1.NewDeleteOptions(0))
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	log.Infof("Deleted custom config for cluster %s", clusterName)
}

func TheCustomJVMOptionsConfigIsChangedForCluster(namespace, clusterName, jvmOptions string) {
	_, err := KubeClientset.CoreV1().ConfigMaps(namespace).Patch(
		fmt.Sprintf("%s-config", clusterName),
		types.StrategicMergePatchType,
		[]byte(fmt.Sprintf("{\"data\": { \"jvm.options\": \"%s\"}}", jvmOptions)),
	)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	log.Infof("Modified custom jvm options for cluster %s", clusterName)
}

func ANewRackIsAddedForCluster(namespace, clusterName string, rack v1alpha1.Rack) {
	mutateCassandraSpec(namespace, clusterName, func(spec *v1alpha1.CassandraSpec) {
		spec.Racks = append(spec.Racks, rack)
	})
	log.Infof("Added new rack %s for cluster %s", rack.Name, clusterName)
}

func ARackIsRemovedFromCluster(namespace, clusterName, rackToRemove string) {
	var racksAfterRemoval []v1alpha1.Rack
	mutateCassandraSpec(namespace, clusterName, func(spec *v1alpha1.CassandraSpec) {
		for _, rack := range spec.Racks {
			if rack.Name != rackToRemove {
				racksAfterRemoval = append(racksAfterRemoval, rack)
			}
		}
		spec.Racks = racksAfterRemoval
	})
	log.Infof("Removed rack %s for cluster %s", rackToRemove, clusterName)
}

func AScheduledSnapshotIsAddedToCluster(namespace, clusterName string, snapshot *v1alpha1.Snapshot) {
	mutateCassandraSpec(namespace, clusterName, func(spec *v1alpha1.CassandraSpec) {
		spec.Snapshot = snapshot
	})
	log.Infof("Added scheduled snapshot for cluster %s", clusterName)
}

func AScheduledSnapshotIsRemovedFromCluster(namespace, clusterName string) {
	mutateCassandraSpec(namespace, clusterName, func(spec *v1alpha1.CassandraSpec) {
		spec.Snapshot = nil
	})
	log.Infof("Removed scheduled snapshot for cluster %s", clusterName)
}

func AScheduledSnapshotIsChangedForCluster(namespace, clusterName string, snapshot *v1alpha1.Snapshot) {
	AScheduledSnapshotIsAddedToCluster(namespace, clusterName, snapshot)
	log.Infof("Updated scheduled snapshot for cluster %s", clusterName)
}

func mutateCassandraSpec(namespace, clusterName string, mutator func(*v1alpha1.CassandraSpec)) {
	cass, err := CassandraClientset.CoreV1alpha1().Cassandras(namespace).Get(clusterName, metaV1.GetOptions{})
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	cassBeforeMutation := cass.DeepCopy()

	mutator(&cass.Spec)
	var cassAfterMutation *v1alpha1.Cassandra
	cassAfterMutation, err = CassandraClientset.CoreV1alpha1().Cassandras(namespace).Update(cass)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	log.Infof(spew.Sprintf("Updated cassandra spec for cluster %s, before: %+v, \nafter: %+v", clusterName, cassBeforeMutation, cassAfterMutation))
}

func EventuallyClusterIsCreatedWithRacks(namespace string, clusterName string, racks []v1alpha1.Rack) {
	var clusterSize int
	for _, rack := range racks {
		clusterSize = clusterSize + int(rack.Replicas)
	}
	clusterBootstrapDuration := (time.Duration(clusterSize) * NodeStartDuration) + (30 * time.Second)
	gomega.Eventually(PodReadyForCluster(namespace, clusterName), clusterBootstrapDuration, CheckInterval).
		Should(gomega.Equal(clusterSize), fmt.Sprintf("Cluster %s was not created within the specified time", clusterName))
}

func CassandraEventsFor(namespace, clusterName string) func() ([]coreV1.Event, error) {
	allEvents := func(coreV1.Event) bool { return true }
	return cassandraEventsFilteredFor(namespace, clusterName, allEvents)
}

func CassandraEventsSince(namespace, clusterName string, sinceTime time.Time) func() ([]coreV1.Event, error) {
	eventsOnOrAfterTime := func(event coreV1.Event) bool {
		metaSinceTime := metaV1.NewTime(sinceTime)
		return event.LastTimestamp.Equal(&metaSinceTime) || event.LastTimestamp.After(sinceTime)
	}
	return cassandraEventsFilteredFor(namespace, clusterName, eventsOnOrAfterTime)
}

func cassandraEventsFilteredFor(namespace, clusterName string, filter func(coreV1.Event) bool) func() ([]coreV1.Event, error) {
	return func() ([]coreV1.Event, error) {
		var cassandraEvents []coreV1.Event
		allEvents, err := KubeClientset.CoreV1().Events(namespace).List(metaV1.ListOptions{})
		if err != nil {
			return nil, err
		}

		for _, event := range allEvents.Items {
			if event.InvolvedObject.Kind == cassandra.Kind && event.InvolvedObject.Name == clusterName && filter(event) {
				cassandraEvents = append(cassandraEvents, event)
			}
		}
		return cassandraEvents, nil
	}
}

func DurationSeconds(seconds int32) time.Duration {
	return time.Duration(seconds) * time.Second
}

func TheStatefulSetIsDeletedForRack(namespace, clusterName, rackName string) {
	statefulSetName := fmt.Sprintf("%s-%s", clusterName, rackName)

	statefulSet, err := KubeClientset.AppsV1beta2().StatefulSets(namespace).Get(statefulSetName, metaV1.GetOptions{})
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	cascadingDelete := metaV1.DeletePropagationForeground
	err = KubeClientset.AppsV1beta1().StatefulSets(namespace).Delete(
		statefulSetName,
		&metaV1.DeleteOptions{PropagationPolicy: &cascadingDelete},
	)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	gomega.Eventually(StatefulSetDeletedSince(namespace, statefulSetName, statefulSet.CreationTimestamp.Time), NodeTerminationDuration, CheckInterval).Should(gomega.BeTrue())
	log.Infof("StatefulSet %s has been deleted since it was initially created at %s", statefulSetName, statefulSet.CreationTimestamp.Time)
}

func TheServiceIsDeletedFor(namespace, clusterName string) {
	service, err := KubeClientset.CoreV1().Services(namespace).Get(clusterName, metaV1.GetOptions{})
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	err = KubeClientset.CoreV1().Services(namespace).Delete(clusterName, metaV1.NewDeleteOptions(0))
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	gomega.Eventually(ServiceIsDeletedSince(namespace, clusterName, service.CreationTimestamp.Time), 30*time.Second, CheckInterval).Should(gomega.BeTrue())
	log.Infof("Service %s has been deleted since it was initially created at %s", clusterName, service.CreationTimestamp)
}

func TheSnapshotCronJobIsDeleted(namespace, clusterName string) {
	TheCronJobIsDeleted(namespace, fmt.Sprintf("%s-snapshot", clusterName))
}

func TheSnapshotCleanupCronJobIsDeleted(namespace, clusterName string) {
	TheCronJobIsDeleted(namespace, fmt.Sprintf("%s-snapshot-cleanup", clusterName))
}

func TheCronJobIsDeleted(namespace, jobName string) {
	job, err := KubeClientset.BatchV1beta1().CronJobs(namespace).Get(jobName, metaV1.GetOptions{})
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	err = KubeClientset.BatchV1beta1().CronJobs(namespace).Delete(jobName, metaV1.NewDeleteOptions(0))
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	gomega.Eventually(CronJobIsDeletedSince(namespace, jobName, job.CreationTimestamp.Time), 30*time.Second, CheckInterval).Should(gomega.BeTrue())
	log.Infof("Cronjob %s has been deleted since it was initially created at %s", jobName, job.CreationTimestamp)
}

func TheCustomConfigHashIsChangedForRack(namespace, clusterName, rackName string) {
	statefulSet, err := KubeClientset.AppsV1beta2().StatefulSets(namespace).Get(fmt.Sprintf("%s-%s", clusterName, rackName), metaV1.GetOptions{})
	gomega.Expect(err).To(gomega.BeNil())
	statefulSet.Spec.Template.Annotations["clusterConfigHash"] = "something else"
	_, err = KubeClientset.AppsV1beta2().StatefulSets(namespace).Update(statefulSet)
	gomega.Expect(err).To(gomega.BeNil())
	log.Infof("Custom config has changed for rack %s", rackName)
}
