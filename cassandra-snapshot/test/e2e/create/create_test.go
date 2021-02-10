package create

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

func TestCreate(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Create Suite", test.CreateReporters("create"))
}

var _ = Describe("Create", func() {

	BeforeEach(func() {
		DeleteCassandraResourcesInNamespace(Namespace)
	})

	It("should create a snapshot for each specified keyspace on each node of the cluster", func() {
		// given
		snapshotClusterPods := []*v1.Pod{
			ACassandraPodWithDefaults().Exists(OperatorLabel, "mycluster-1", "app", "mycluster-1"),
			ACassandraPodWithDefaults().Exists(OperatorLabel, "mycluster-1", "app", "mycluster-1"),
		}
		noSnapshotClusterPod := ACassandraPodWithDefaults().Exists(OperatorLabel, "mycluster-2", "app", "mycluster-2")

		// when
		startTime := time.Now()
		snapshotPod := RunCommandInCassandraSnapshotPod(
			"mycluster-1",
			"/cassandra-snapshot", "create",
			"-L", "debug",
			"-n", Namespace,
			"-k", "system_auth,system_traces",
			"-l", fmt.Sprintf("%s=%s,%s=%s", OperatorLabel, "mycluster-1", "app", "mycluster-1"))
		Eventually(PodIsTerminatedSuccessfully(snapshotPod), NodeTerminationDuration, 2*time.Second).Should(BeTrue())
		stopTime := time.Now()

		// then
		for _, pod := range snapshotClusterPods {
			snapshotsForPod, err := SnapshotListForPod(pod)
			Expect(err).ToNot(HaveOccurred())

			for _, snapshot := range snapshotsForPod {
				Expect(snapshot).To(Or(
					BeForKeyspace("system_auth").AndWithinTimeRange(startTime, stopTime),
					BeForKeyspace("system_traces").AndWithinTimeRange(startTime, stopTime),
				))
			}
		}
		Expect(SnapshotListForPod(noSnapshotClusterPod)).To(HaveLen(0))
	})

	It("should create a snapshot for each specified keyspace on each node of the cluster", func() {
		// given
		snapshotClusterPods := []*v1.Pod{
			ACassandraPodWithDefaults().WithoutEnvironmentVariables().Exists(OperatorLabel, "mycluster-1", "app", "mycluster-1"),
			ACassandraPodWithDefaults().WithoutEnvironmentVariables().Exists(OperatorLabel, "mycluster-1", "app", "mycluster-1"),
		}
		noSnapshotClusterPod := ACassandraPodWithDefaults().Exists(OperatorLabel, "mycluster-2", "app", "mycluster-2")

		// when
		startTime := time.Now()
		snapshotPod := RunCommandInCassandraSnapshotPod(
			"mycluster-1",
			"/cassandra-snapshot", "create",
			"-L", "debug",
			"-n", Namespace,
			"-k", "system_auth,system_traces",
			"-l", fmt.Sprintf("%s=%s,%s=%s", OperatorLabel, "mycluster-1", "app", "mycluster-1"))
		Eventually(PodIsTerminatedSuccessfully(snapshotPod), NodeTerminationDuration, 2*time.Second).Should(BeTrue())
		stopTime := time.Now()

		// then
		for _, pod := range snapshotClusterPods {
			snapshotsForPod, err := SnapshotListForPod(pod)
			Expect(err).ToNot(HaveOccurred())

			for _, snapshot := range snapshotsForPod {
				Expect(snapshot).To(Or(
					BeForKeyspace("system_auth").AndWithinTimeRange(startTime, stopTime),
					BeForKeyspace("system_traces").AndWithinTimeRange(startTime, stopTime),
				))
			}
		}
		Expect(SnapshotListForPod(noSnapshotClusterPod)).To(HaveLen(0))
	})
	It("should create a snapshot for all keyspaces when no keyspace is specified", func() {
		// given
		cassandraPod := ACassandraPodWithDefaults().Exists(OperatorLabel, "mycluster-1", "app", "mycluster-1")

		// when
		startTime := time.Now()
		snapshotPod := RunCommandInCassandraSnapshotPod(
			"mycluster-1",
			"/cassandra-snapshot", "create",
			"-L", "debug",
			"-n", Namespace,
			"-l", fmt.Sprintf("%s=%s,%s=%s", OperatorLabel, "mycluster-1", "app", "mycluster-1"))
		Eventually(PodIsTerminatedSuccessfully(snapshotPod), NodeTerminationDuration, 2*time.Second).Should(BeTrue())
		stopTime := time.Now()

		// then
		snapshotsForPod, err := SnapshotListForPod(cassandraPod)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(snapshotsForPod)).To(BeNumerically(">", 1))
		for _, snapshot := range snapshotsForPod {
			Expect(snapshot).To(Or(
				BeForKeyspace("system_auth").AndWithinTimeRange(startTime, stopTime),
				BeForKeyspace("system_traces").AndWithinTimeRange(startTime, stopTime),
				BeForKeyspace("system_distributed").AndWithinTimeRange(startTime, stopTime),
			))
		}
	})

	It("should fail with a non-zero exit code when an invalid command is supplied", func() {
		snapshotPod := RunCommandInCassandraSnapshotPod("mycluster-1", "/cassandra-snapshot", "create", "-L", "debug", "-n", "invalid-namespace")
		Eventually(PodIsTerminatedUnsuccessfully(snapshotPod), NodeTerminationDuration, 2*time.Second).Should(BeTrue())
	})
})
