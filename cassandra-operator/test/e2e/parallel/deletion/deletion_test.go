package deletion

import (
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test"
	. "github.com/sky-uk/cassandra-operator/cassandra-operator/test/e2e"
)

var (
	multipleNodeCluster *TestCluster
	singleNodeCluster   *TestCluster
	testStartTime       time.Time
)

func TestDeletion(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "E2E Suite (Deletion Tests)", test.CreateParallelReporters("e2e_deletion"))
}

func defineClusters(multipleNodeClusterName, singleNodeClusterName string) (multipleNodeCluster, singleNodeCluster *TestCluster) {
	multipleNodeCluster = &TestCluster{
		Name:  multipleNodeClusterName,
		Racks: []v1alpha1.Rack{Rack("a", 1), Rack("b", 1)},
	}

	singleNodeCluster = &TestCluster{
		Name:           singleNodeClusterName,
		Racks:          []v1alpha1.Rack{Rack("a", 1)},
		SnapshotConfig: SnapshotConfigFromSchedule("0/1 * * * *"),
	}
	return
}

func createClustersInParallel(multipleNodeCluster, singleNodeCluster *TestCluster) {
	AClusterWithName(multipleNodeCluster.Name).AndRacks(multipleNodeCluster.Racks).AndScheduledSnapshot(multipleNodeCluster.SnapshotConfig).IsDefined()
	AClusterWithName(singleNodeCluster.Name).AndRacks(singleNodeCluster.Racks).AndScheduledSnapshot(singleNodeCluster.SnapshotConfig).IsDefined()

	Eventually(PodReadyForCluster(Namespace, multipleNodeCluster.Name), 2*NodeStartDuration, CheckInterval).
		Should(Equal(2), fmt.Sprintf("For cluster %s", multipleNodeCluster.Name))
	Eventually(PodReadyForCluster(Namespace, singleNodeCluster.Name), NodeStartDuration, CheckInterval).
		Should(Equal(1), fmt.Sprintf("For cluster %s", singleNodeCluster.Name))
}

var _ = ParallelTestBeforeSuite(func() []TestCluster {
	multipleNodeCluster, singleNodeCluster = defineClusters(AClusterName(), AClusterName())
	createClustersInParallel(multipleNodeCluster, singleNodeCluster)
	return []TestCluster{*multipleNodeCluster, *singleNodeCluster}
}, func(clusterNames []string) {
	multipleNodeCluster, singleNodeCluster = defineClusters(clusterNames[0], clusterNames[1])
})

var _ = Context("Cluster and node deletion", func() {

	BeforeEach(func() {
		testStartTime = time.Now()
	})

	JustAfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			PrintDiagnosis(Namespace, testStartTime, multipleNodeCluster.Name, singleNodeCluster.Name)
		}
	})

	Context("when a cluster is deleted", func() {
		It("should clean up everything related to the cluster, except for data", func() {
			// given
			Eventually(SnapshotJobsFor(singleNodeCluster.Name), NodeStartDuration, CheckInterval).Should(BeNumerically(">", 0))

			// when
			TheClusterIsDeleted(singleNodeCluster.Name)

			// then
			By("removing all Kubernetes resources except for the persistent volumes")
			Eventually(StatefulSetsForCluster(Namespace, singleNodeCluster.Name), NodeTerminationDuration, CheckInterval).Should(BeEmpty())
			Eventually(PodsForCluster(Namespace, singleNodeCluster.Name), NodeTerminationDuration, CheckInterval).Should(HaveLen(0))
			Eventually(SnapshotJobsFor(singleNodeCluster.Name), NodeTerminationDuration, CheckInterval).Should(BeZero())
			Expect(HeadlessServiceForCluster(Namespace, singleNodeCluster.Name)()).Should(BeNil())

			Expect(PersistentVolumeClaimsForCluster(Namespace, singleNodeCluster.Name)()).Should(HaveLen(1))

			By("not reporting metrics for it any longer")
			Eventually(OperatorMetrics(Namespace), 60*time.Second, CheckInterval).Should(
				ReportNoClusterMetricsFor(Namespace, singleNodeCluster.Name))

			By("leaving the other clusters running")
			// there will be either 1 or 2 nodes ready in the multiple-node cluster, depending on whether the other
			// test has terminated a node or not
			Expect(PodReadyForCluster(Namespace, multipleNodeCluster.Name)()).Should(Or(Equal(1), Equal(2)))

			By("continuing to report metrics for the other clusters")
			Eventually(OperatorMetrics(Namespace), 60*time.Second, CheckInterval).Should(ReportAClusterWith([]MetricAssertion{
				LiveAndNormalNodeMetric(Namespace, multipleNodeCluster.Name, PodName(multipleNodeCluster.Name, "a", 0), "a", 1),
			}))
		})
	})

	Context("when a node is deleted from an existing cluster", func() {
		It("should be automatically replaced and rejoin the cluster", func() {
			// when
			deletionTimestamp := time.Now().Truncate(time.Second)
			err := KubeClientset.CoreV1().Pods(Namespace).Delete(PodName(multipleNodeCluster.Name, "b", 0), metaV1.NewDeleteOptions(0))
			Expect(err).ToNot(HaveOccurred())

			// then
			By("its pod being recreated and in the ready state")
			Eventually(PodCreationTime(Namespace, PodName(multipleNodeCluster.Name, "b", 0)), NodeStartDuration, CheckInterval).Should(BeTemporally(">=", deletionTimestamp))
			Eventually(PodReadinessStatus(Namespace, PodName(multipleNodeCluster.Name, "b", 0)), NodeStartDuration, CheckInterval).Should(BeTrue())

			By("not restarting any pods")
			Expect(PodRestartForCluster(Namespace, multipleNodeCluster.Name)()).Should(Equal(0))

			By("metrics being reported on the new pod's IP address")
			Eventually(OperatorMetrics(Namespace), 60*time.Second, CheckInterval).Should(ReportAClusterWith([]MetricAssertion{
				LiveAndNormalNodeMetric(Namespace, multipleNodeCluster.Name, PodName(multipleNodeCluster.Name, "a", 0), "a", 1),
				LiveAndNormalNodeMetric(Namespace, multipleNodeCluster.Name, PodName(multipleNodeCluster.Name, "b", 0), "b", 1),
				ClusterSizeMetric(Namespace, multipleNodeCluster.Name, 2),
				FailedValidationMetric(Namespace, multipleNodeCluster.Name, 0),
			}))

		})
	})
})
