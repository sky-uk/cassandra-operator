package cleanup

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sky-uk/cassandra-operator/cassandra-snapshot/test"
	. "github.com/sky-uk/cassandra-operator/cassandra-snapshot/test/e2e"
	"k8s.io/api/core/v1"
	"testing"
	"time"
)

func TestCleanup(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Cleanup Suite", test.CreateReporters("cleanup"))
}

var _ = Describe("Cleanup", func() {
	BeforeEach(func() {
		DeleteCassandraResourcesInNamespace(Namespace)
	})

	It("should delete any snapshots which are older than the retention period across all pods in the cluster", func() {
		// given
		clusterPods := []*v1.Pod{
			ACassandraPodWithDefaults().Exists(OperatorLabel, "mycluster-1", "app", "mycluster-1"),
			ACassandraPodWithDefaults().Exists(OperatorLabel, "mycluster-1", "app", "mycluster-1"),
		}

		snapshotPod := RunCommandInCassandraSnapshotPod(
			"mycluster-1",
			"/cassandra-snapshot", "create",
			"-L", "debug",
			"-n", Namespace,
			"-l", fmt.Sprintf("%s=%s,%s=%s", OperatorLabel, "mycluster-1", "app", "mycluster-1"),
			"-k", "system_auth,system_traces")
		Eventually(PodIsTerminatedSuccessfully(snapshotPod), NodeTerminationDuration, 2*time.Second).Should(BeTrue())

		BackdateSnapshotsForPods(clusterPods, time.Hour)

		// when
		cleanupPod := RunCommandInCassandraSnapshotPod(
			"mycluster-1",
			"/cassandra-snapshot", "cleanup",
			"-L", "debug",
			"-n", Namespace,
			"-l", fmt.Sprintf("%s=%s,%s=%s", OperatorLabel, "mycluster-1", "app", "mycluster-1"),
			"-r", "30m")
		Eventually(PodIsTerminatedSuccessfully(cleanupPod), NodeTerminationDuration, 2*time.Second).Should(BeTrue())

		// then
		for _, pod := range clusterPods {
			Expect(SnapshotListForPod(pod)).To(HaveLen(0))
		}
	})

	It("should delete any snapshots which are older than the retention period when nodetool args are not defined", func() {
		// given
		clusterPods := []*v1.Pod{
			ACassandraPodWithDefaults().WithoutEnvironmentVariables().Exists(OperatorLabel, "mycluster-1", "app", "mycluster-1"),
			ACassandraPodWithDefaults().WithoutEnvironmentVariables().Exists(OperatorLabel, "mycluster-1", "app", "mycluster-1"),
		}

		snapshotPod := RunCommandInCassandraSnapshotPod(
			"mycluster-1",
			"/cassandra-snapshot", "create",
			"-L", "debug",
			"-n", Namespace,
			"-l", fmt.Sprintf("%s=%s,%s=%s", OperatorLabel, "mycluster-1", "app", "mycluster-1"),
			"-k", "system_auth,system_traces")
		Eventually(PodIsTerminatedSuccessfully(snapshotPod), NodeTerminationDuration, 2*time.Second).Should(BeTrue())

		BackdateSnapshotsForPods(clusterPods, time.Hour)

		// when
		cleanupPod := RunCommandInCassandraSnapshotPod(
			"mycluster-1",
			"/cassandra-snapshot", "cleanup",
			"-L", "debug",
			"-n", Namespace,
			"-l", fmt.Sprintf("%s=%s,%s=%s", OperatorLabel, "mycluster-1", "app", "mycluster-1"),
			"-r", "30m")
		Eventually(PodIsTerminatedSuccessfully(cleanupPod), NodeTerminationDuration, 2*time.Second).Should(BeTrue())

		// then
		for _, pod := range clusterPods {
			Expect(SnapshotListForPod(pod)).To(HaveLen(0))
		}
	})

	It("should not delete snapshots which are younger than the retention period", func() {
		// given
		clusterPod := ACassandraPodWithDefaults().Exists(OperatorLabel, "mycluster-1", "app", "mycluster-1")

		snapshotPod := RunCommandInCassandraSnapshotPod(
			"mycluster-1",
			"/cassandra-snapshot", "create",
			"-L", "debug",
			"-n", Namespace,
			"-l", fmt.Sprintf("%s=%s,%s=%s", OperatorLabel, "mycluster-1", "app", "mycluster-1"),
			"-k", "system_auth,system_traces")
		Eventually(PodIsTerminatedSuccessfully(snapshotPod), NodeTerminationDuration, 2*time.Second).Should(BeTrue())
		snapshotsBeforeCleanup, err := SnapshotListForPod(clusterPod)
		Expect(err).ToNot(HaveOccurred())
		Expect(snapshotsBeforeCleanup).ToNot(BeEmpty())

		// when
		cleanupPod := RunCommandInCassandraSnapshotPod(
			"mycluster-1",
			"/cassandra-snapshot", "cleanup",
			"-L", "debug",
			"-n", Namespace,
			"-l", fmt.Sprintf("%s=%s,%s=%s", OperatorLabel, "mycluster-1", "app", "mycluster-1"),
			"-r", "30m")
		Eventually(PodIsTerminatedSuccessfully(cleanupPod), NodeTerminationDuration, 2*time.Second).Should(BeTrue())

		// then
		snapshotsAfterCleanup, err := SnapshotListForPod(clusterPod)
		Expect(err).ToNot(HaveOccurred())
		Expect(snapshotsAfterCleanup).To(Equal(snapshotsBeforeCleanup))
	})

	It("should not delete snapshots whose name do not match the naming convention", func() {
		// given
		clusterPod := ACassandraPodWithDefaults().Exists(OperatorLabel, "mycluster-1", "app", "mycluster-1")

		snapshotPod := RunCommandInCassandraSnapshotPod(
			"mycluster-1",
			"/cassandra-snapshot", "create",
			"-L", "debug",
			"-n", Namespace,
			"-l", fmt.Sprintf("%s=%s,%s=%s", OperatorLabel, "mycluster-1", "app", "mycluster-1"),
			"-k", "system_auth")
		Eventually(PodIsTerminatedSuccessfully(snapshotPod), NodeTerminationDuration, 2*time.Second).Should(BeTrue())

		RenameSnapshotsForPod(clusterPod, "another_snapshot")

		// when
		cleanupPod := RunCommandInCassandraSnapshotPod(
			"mycluster-1",
			"/cassandra-snapshot", "cleanup",
			"-L", "debug",
			"-n", Namespace,
			"-l", fmt.Sprintf("%s=%s,%s=%s", OperatorLabel, "mycluster-1", "app", "mycluster-1"))
		Eventually(PodIsTerminatedSuccessfully(cleanupPod), NodeTerminationDuration, 2*time.Second).Should(BeTrue())

		// then
		snapshots, err := SnapshotListForPod(clusterPod)
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeEmpty())
		for _, snapshot := range snapshots {
			Expect(snapshot.Name).To(Equal("another_snapshot"))
		}
	})

	It("should fail with a non-zero exit code when an invalid command is supplied", func() {
		snapshotPod := RunCommandInCassandraSnapshotPod("mycluster-1", "/cassandra-snapshot", "cleanup", "-L", "debug", "-n", "invalid-namespace")
		Eventually(PodIsTerminatedUnsuccessfully(snapshotPod), NodeTerminationDuration, 2*time.Second).Should(BeTrue())
	})

})
