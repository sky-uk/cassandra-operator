package modification

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"

	. "github.com/sky-uk/cassandra-operator/cassandra-operator/test/e2e"
)

var _ = Context("Cluster untriggered modifications", func() {
	var (
		clusterName string
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

	It("should report a node as down in metrics when it is offline", func() {
		// given
		registerResourcesUsed(2)
		racks := []v1alpha1.Rack{RackWithEmptyDir("a", 1), RackWithEmptyDir("b", 1)}
		AClusterWithName(clusterName).AndRacks(racks).UsingEmptyDir().Exists()

		// when
		aNodeIsOffline(Namespace, PodName(clusterName, "b", 0))

		// then
		Eventually(OperatorMetrics(Namespace), 60*time.Second, CheckInterval).Should(ReportAClusterWith([]MetricAssertion{
			ClusterSizeMetric(Namespace, clusterName, 2),
			LiveAndNormalNodeMetric(Namespace, clusterName, PodName(clusterName, "a", 0), "a", 1),
			DownAndNormalNodeMetric(Namespace, clusterName, PodName(clusterName, "b", 0), "b", 1),
		}))
	})
})

func aNodeIsOffline(namespace string, nodeName string) {
	command, output, err := Kubectl(namespace, "exec", nodeName, "nodetool", "drain")
	Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("command was %v.\nOutput of exec was:\n%s\n. Error: %v", command, output, err))
}
