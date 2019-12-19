package modification

import (
	"fmt"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sky-uk/cassandra-operator/cassandra-operator/test/e2e"
)

var _ = Context("External cluster modifications", func() {
	var (
		clusterName   string
		testStartTime time.Time
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

	Context("resources are deleted unintentionally", func() {
		It("should eventually bring the cluster to the desired state", func() {
			// given
			registerResourcesUsed(2)
			racks := []v1alpha1.Rack{Rack("a", 1), Rack("b", 1)}
			AClusterWithName(clusterName).
				AndRacks(racks).
				AndScheduledSnapshot(&v1alpha1.Snapshot{
					Image:    CassandraSnapshotImageName,
					Schedule: "59 23 * * *",
					RetentionPolicy: &v1alpha1.RetentionPolicy{
						CleanupSchedule: "11 22 1 * *",
					},
				}).
				Exists()

			// when
			deletionTimestamp := time.Now().Truncate(time.Second)
			TheServiceIsDeletedFor(Namespace, clusterName)
			TheStatefulSetIsDeletedForRack(Namespace, clusterName, "a")
			TheSnapshotCronJobIsDeleted(Namespace, clusterName)
			TheSnapshotCleanupCronJobIsDeleted(Namespace, clusterName)

			// then
			By("creating the deleted headless service for the cluster")
			Eventually(HeadlessServiceForCluster(Namespace, clusterName), NodeStartDuration, CheckInterval).Should(And(
				Not(BeNil()),
				HaveLabel("app.kubernetes.io/name", clusterName),
				HaveLabel("app.kubernetes.io/instance", fmt.Sprintf("%s.%s", Namespace, clusterName)),
				HaveLabel("app.kubernetes.io/managed-by", "cassandra-operator"),
			))

			By("re-creating the deleted rack")
			Eventually(RacksForCluster(Namespace, clusterName), NodeStartDuration, CheckInterval).Should(And(
				HaveLen(2),
				HaveKeyWithValue("a", []string{PodName(clusterName, "a", 0)}),
				HaveKeyWithValue("b", []string{PodName(clusterName, "b", 0)}),
			))
			Eventually(PodReadyForCluster(Namespace, clusterName), NodeStartDuration, CheckInterval).
				Should(Equal(2), fmt.Sprintf("For cluster %s", clusterName))
			Eventually(PodCreationTime(Namespace, PodName(clusterName, "a", 0)), NodeStartDuration, CheckInterval).
				Should(BeTemporally(">=", deletionTimestamp))

			By("re-creating the delete snapshot job")
			By("re-creating the delete snapshot cleanup job")
			Eventually(CronJobsForCluster(Namespace, clusterName), time.Minute, CheckInterval).Should(And(
				HaveLen(2),
				Each(BeCreatedOnOrAfter(deletionTimestamp)),
			))
		})
	})

})
