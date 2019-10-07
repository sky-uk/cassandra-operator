package modification

import (
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sky-uk/cassandra-operator/cassandra-operator/test/e2e"
)

var _ = Context("Incomplete cluster modifications", func() {
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

	Context("a configMap change was not rolled out to a rack", func() {
		It("should eventually bring the rack to the desired state", func() {
			// given
			registerResourcesUsed(2)
			racks := []v1alpha1.Rack{Rack("a", 1), Rack("b", 1)}
			AClusterWithName(clusterName).
				AndRacks(racks).
				Exists()

			// when
			expectedConfigHash := ClusterConfigHashForRack(Namespace, clusterName, "a")
			modificationTime := time.Now().Truncate(time.Second)
			// pretend this rack was left on an old config
			TheCustomConfigHashIsChangedForRack(Namespace, clusterName, "b")

			// then
			By("rolling out the new config to the out-of-date rack")
			Eventually(PodCreationTime(Namespace, PodName(clusterName, "b", 0)), NodeStartDuration, CheckInterval).
				Should(BeTemporally(">=", modificationTime))
			Eventually(PodsForCluster(Namespace, clusterName), NodeRestartDuration, CheckInterval).Should(Each(And(
				HaveAnnotation("clusterConfigHash"),
				HaveAnnotationValue(AnnotationValueAssertion{Name: "clusterConfigHash", Value: expectedConfigHash}),
			)))
		})
	})

})
