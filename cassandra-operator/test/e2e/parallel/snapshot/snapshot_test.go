package snapshot

import (
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	coreV1 "k8s.io/api/core/v1"

	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test"
	. "github.com/sky-uk/cassandra-operator/cassandra-operator/test/e2e"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test/e2e/parallel"
)

var (
	resources          *parallel.ResourceSemaphore
	resourcesToReclaim int
	testStartTime      time.Time
)

func TestSnapshot(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "E2E Suite (Snapshot Tests)", test.CreateParallelReporters("e2e_snapshot"))
}

var _ = ParallelTestBeforeSuite(func() []TestCluster {
	// initialise the resources available just once for the entire test suite
	resources = parallel.NewResourceSemaphore(MaxCassandraNodesPerNamespace)
	return []TestCluster{}
}, func(clusterNames []string) {
	// instantiate the accessor to the resource file for each spec,
	// so they can make use of it to acquire / release resources
	resources = parallel.NewUnInitialisedResourceSemaphore(MaxCassandraNodesPerNamespace)
})

func registerResourcesUsed(size int) {
	resourcesToReclaim = size
	resources.AcquireResource(size)
}

var _ = Describe("Cassandra snapshot scheduling", func() {
	var (
		clusterName     string
		retentionPeriod = int32(10)
		timeout         = int32(5)
	)

	BeforeEach(func() {
		testStartTime = time.Now()
		clusterName = AClusterName()
	})

	JustAfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			PrintDiagnosis(Namespace, testStartTime, clusterName)
		}
	})

	AfterEach(func() {
		DeleteCassandraResourcesForClusters(Namespace, clusterName)
		resources.ReleaseResource(resourcesToReclaim)
	})

	var _ = Context("when a cluster is created with a snapshot spec with a retention policy", func() {
		It("should create the corresponding snapshot jobs", func() {
			// when
			registerResourcesUsed(1)
			AClusterWithName(clusterName).
				AndRacks([]v1alpha1.Rack{RackWithEmptyDir("a", 1)}).
				AndScheduledSnapshot(&v1alpha1.Snapshot{
					Image:     CassandraSnapshotImageName,
					Schedule:  "59 23 * * *",
					Keyspaces: []string{"keyspace1", "keyspace3"},
					RetentionPolicy: &v1alpha1.RetentionPolicy{
						RetentionPeriodDays:   &retentionPeriod,
						CleanupSchedule:       "11 22 1 * *",
						CleanupTimeoutSeconds: &timeout,
					},
				}).
				Exists()
			clusterCreatedTime := time.Now()

			// then
			By("scheduling a job that will trigger a snapshot for the specified keyspaces")
			By("scheduling a job that will trigger a snapshot cleanup")
			Eventually(CronJobsForCluster(Namespace, clusterName), 30*time.Second, CheckInterval).Should(And(
				HaveLen(2),
				ContainElement(HaveJobSpec(&JobExpectation{
					Schedule:       "59 23 * * *",
					ContainerImage: "cassandra-snapshot",
					ContainerCommand: []string{
						"/cassandra-snapshot", "create",
						"-n", Namespace,
						"-l", fmt.Sprintf("%s=%s,%s=%s", cluster.OperatorLabel, clusterName, "app", clusterName),
						"-t", "10s",
						"-k", "keyspace1,keyspace3",
					}}),
				),
				ContainElement(HaveJobSpec(&JobExpectation{
					Schedule:       "11 22 1 * *",
					ContainerImage: "cassandra-snapshot:",
					ContainerCommand: []string{
						"/cassandra-snapshot", "cleanup",
						"-n", Namespace,
						"-l", fmt.Sprintf("%s=%s,%s=%s", cluster.OperatorLabel, clusterName, "app", clusterName),
						"-r", "240h0m0s",
						"-t", "5s",
					}}),
				),
			))

			By("registering an event for the snapshot job creation")
			Eventually(CassandraEventsFor(Namespace, clusterName), EventPublicationTimeout, CheckInterval).Should(HaveEvent(EventExpectation{
				Type:                 coreV1.EventTypeNormal,
				Reason:               cluster.ClusterSnapshotCreationScheduleEvent,
				Message:              fmt.Sprintf("Snapshot creation scheduled for cluster %s.%s", Namespace, clusterName),
				LastTimestampCloseTo: &clusterCreatedTime,
			}))
			By("registering an event for the snapshot cleanup job creation")
			Eventually(CassandraEventsFor(Namespace, clusterName), EventPublicationTimeout, CheckInterval).Should(HaveEvent(EventExpectation{
				Type:                 coreV1.EventTypeNormal,
				Reason:               cluster.ClusterSnapshotCleanupScheduleEvent,
				Message:              fmt.Sprintf("Snapshot cleanup scheduled for cluster %s.%s", Namespace, clusterName),
				LastTimestampCloseTo: &clusterCreatedTime,
			}))
		})
	})

	var _ = Context("when a scheduled snapshot is added for a cluster", func() {
		It("should create the corresponding snapshot creation job", func() {
			// given
			registerResourcesUsed(1)
			AClusterWithName(clusterName).
				AndRacks([]v1alpha1.Rack{RackWithEmptyDir("a", 1)}).
				Exists()

			// wait long enough to ensure we see all events related to cluster creation, so we can filter these out later
			waitForClusterCreationCompleteEvent(clusterName)

			// when
			// wait a bit to help spreading the creation from modification events
			time.Sleep(time.Second)
			clusterModifiedTime := asEventTime(time.Now())
			snapshotTimeout := int32(5)
			AScheduledSnapshotIsAddedToCluster(Namespace, clusterName, &v1alpha1.Snapshot{
				Image:          CassandraSnapshotImageName,
				Schedule:       "1 23 * * *",
				TimeoutSeconds: &snapshotTimeout,
				Keyspaces:      []string{"k1", "k2"},
			})

			// then
			By("creating a new snapshot cron job")
			Eventually(CronJobsForCluster(Namespace, clusterName), 30*time.Second, CheckInterval).Should(And(
				HaveLen(1),
				Each(HaveJobSpec(&JobExpectation{
					Schedule:       "1 23 * * *",
					ContainerImage: "cassandra-snapshot",
					ContainerCommand: []string{
						"/cassandra-snapshot", "create",
						"-n", Namespace,
						"-l", fmt.Sprintf("%s=%s,%s=%s", cluster.OperatorLabel, clusterName, "app", clusterName),
						"-t", "5s",
						"-k", "k1,k2",
					}}),
				),
			))

			By("registering an event for the cronjob creation")
			Eventually(CassandraEventsSince(Namespace, clusterName, clusterModifiedTime), 30*time.Second, CheckInterval).Should(HaveEvent(EventExpectation{
				Type:                 coreV1.EventTypeNormal,
				Reason:               cluster.ClusterSnapshotCreationScheduleEvent,
				Message:              fmt.Sprintf("Snapshot creation scheduled for cluster %s.%s", Namespace, clusterName),
				LastTimestampCloseTo: &clusterModifiedTime,
			}))

			By("not performing a rolling update of the cluster")
			Expect(CassandraEventsSince(Namespace, clusterName, clusterModifiedTime)()).Should(HaveLen(1))
			Expect(PodRestartForCluster(Namespace, clusterName)()).Should(Equal(0))
		})
	})

	var _ = Context("when a scheduled snapshot is removed", func() {
		It("should delete both snapshot and snapshot cleanup jobs", func() {
			// given
			registerResourcesUsed(1)
			AClusterWithName(clusterName).
				AndRacks([]v1alpha1.Rack{RackWithEmptyDir("a", 1)}).
				AndScheduledSnapshot(&v1alpha1.Snapshot{
					Image:     CassandraSnapshotImageName,
					Schedule:  "59 23 * * *",
					Keyspaces: []string{"keyspace1", "keyspace3"},
					RetentionPolicy: &v1alpha1.RetentionPolicy{
						RetentionPeriodDays:   &retentionPeriod,
						CleanupSchedule:       "11 22 1 * *",
						CleanupTimeoutSeconds: &timeout,
					},
				}).
				Exists()

			// wait long enough to ensure we see all events related to cluster creation, so we can filter these out later
			waitForClusterCreationCompleteEvent(clusterName)

			// when
			// wait a bit to help spreading the creation from modification events
			time.Sleep(time.Second)
			snapshotDeletedTime := asEventTime(time.Now())
			AScheduledSnapshotIsRemovedFromCluster(Namespace, clusterName)

			// then
			By("registering an event for the snapshot job deletion")
			Eventually(CassandraEventsSince(Namespace, clusterName, snapshotDeletedTime), 30*time.Second, CheckInterval).Should(HaveEvent(EventExpectation{
				Type:                 coreV1.EventTypeNormal,
				Reason:               cluster.ClusterSnapshotCreationUnscheduleEvent,
				Message:              fmt.Sprintf("Snapshot creation unscheduled for cluster %s.%s", Namespace, clusterName),
				LastTimestampCloseTo: &snapshotDeletedTime,
			}))
			By("registering an event for the snapshot cleanup job deletion")
			Eventually(CassandraEventsSince(Namespace, clusterName, snapshotDeletedTime), 30*time.Second, CheckInterval).Should(HaveEvent(EventExpectation{
				Type:                 coreV1.EventTypeNormal,
				Reason:               cluster.ClusterSnapshotCleanupUnscheduleEvent,
				Message:              fmt.Sprintf("Snapshot cleanup unscheduled for cluster %s.%s", Namespace, clusterName),
				LastTimestampCloseTo: &snapshotDeletedTime,
			}))

			By("removing the snapshot jobs")
			Expect(CronJobsForCluster(Namespace, clusterName)()).Should(HaveLen(0))

			By("not performing a rolling update of the cluster")
			Expect(CassandraEventsSince(Namespace, clusterName, snapshotDeletedTime)()).Should(HaveLen(2))
			Expect(PodRestartForCluster(Namespace, clusterName)()).Should(Equal(0))
		})
	})

	var _ = Context("when a scheduled snapshot is modified", func() {
		It("should create a kubernetes job with the new properties and delete the old one", func() {
			// given
			registerResourcesUsed(1)
			AClusterWithName(clusterName).
				AndRacks([]v1alpha1.Rack{RackWithEmptyDir("a", 1)}).
				AndScheduledSnapshot(&v1alpha1.Snapshot{
					Image:     CassandraSnapshotImageName,
					Schedule:  "59 23 * * *",
					Keyspaces: []string{"keyspace1", "keyspace3"},
					RetentionPolicy: &v1alpha1.RetentionPolicy{
						RetentionPeriodDays:   &retentionPeriod,
						CleanupSchedule:       "11 22 1 * *",
						CleanupTimeoutSeconds: &timeout,
					},
				}).
				Exists()

			// wait long enough to ensure we see all events related to cluster creation, so we can filter these out later
			waitForClusterCreationCompleteEvent(clusterName)

			// when
			// wait a bit to help spreading the creation from modification events
			time.Sleep(time.Second)
			snapshotModificationTime := asEventTime(time.Now())
			AScheduledSnapshotIsChangedForCluster(Namespace, clusterName, &v1alpha1.Snapshot{
				Image:     CassandraSnapshotImageName,
				Schedule:  "15 9 * * *",
				Keyspaces: []string{"k2"},
				RetentionPolicy: &v1alpha1.RetentionPolicy{
					RetentionPeriodDays:   &retentionPeriod,
					CleanupSchedule:       "2 5 1 * *",
					CleanupTimeoutSeconds: &timeout,
				},
			})

			// then
			By("updating the snapshot cron job with the new settings")
			Eventually(CronJobsForCluster(Namespace, clusterName), 30*time.Second, CheckInterval).Should(And(
				HaveLen(2),
				ContainElement(HaveJobSpec(&JobExpectation{
					Schedule:       "15 9 * * *",
					ContainerImage: "cassandra-snapshot",
					ContainerCommand: []string{
						"/cassandra-snapshot", "create",
						"-n", Namespace,
						"-l", fmt.Sprintf("%s=%s,%s=%s", cluster.OperatorLabel, clusterName, "app", clusterName),
						"-t", "10s",
						"-k", "k2",
					}}),
				),
				ContainElement(HaveJobSpec(&JobExpectation{
					Schedule:       "2 5 1 * *",
					ContainerImage: "cassandra-snapshot:",
					ContainerCommand: []string{
						"/cassandra-snapshot", "cleanup",
						"-n", Namespace,
						"-l", fmt.Sprintf("%s=%s,%s=%s", cluster.OperatorLabel, clusterName, "app", clusterName),
						"-r", "240h0m0s",
						"-t", "5s",
					}}),
				),
			))

			By("registering an event for the snapshot job modification")
			Eventually(CassandraEventsSince(Namespace, clusterName, snapshotModificationTime), 30*time.Second, CheckInterval).Should(HaveEvent(EventExpectation{
				Type:                 coreV1.EventTypeNormal,
				Reason:               cluster.ClusterSnapshotCreationModificationEvent,
				Message:              fmt.Sprintf("Snapshot creation modified for cluster %s.%s", Namespace, clusterName),
				LastTimestampCloseTo: &snapshotModificationTime,
			}))

			By("registering an event for the snapshot cleanup job modification")
			Eventually(CassandraEventsSince(Namespace, clusterName, snapshotModificationTime), 30*time.Second, CheckInterval).Should(HaveEvent(EventExpectation{
				Type:                 coreV1.EventTypeNormal,
				Reason:               cluster.ClusterSnapshotCleanupModificationEvent,
				Message:              fmt.Sprintf("Snapshot cleanup modified for cluster %s.%s", Namespace, clusterName),
				LastTimestampCloseTo: &snapshotModificationTime,
			}))

			By("not performing a rolling update of the cluster")
			Expect(CassandraEventsSince(Namespace, clusterName, snapshotModificationTime)()).Should(HaveLen(2))
			Expect(PodRestartForCluster(Namespace, clusterName)()).Should(Equal(0))
		})
	})

})

// events appears to be recorded with truncated seconds
func asEventTime(theTime time.Time) time.Time {
	return theTime.Truncate(time.Second)
}

func waitForClusterCreationCompleteEvent(clusterName string) {
	Eventually(CassandraEventsFor(Namespace, clusterName), EventPublicationTimeout, CheckInterval).Should(HaveEvent(EventExpectation{
		Type:   coreV1.EventTypeNormal,
		Reason: cluster.StatefulSetChangeComplete,
	}))
}
