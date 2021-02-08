package reconciliation

import (
	"fmt"
	coreV1 "k8s.io/api/core/v1"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	. "github.com/sky-uk/cassandra-operator/cassandra-operator/test/e2e"
)

var _ = Context("External statefulset cassandra modifications trigger reconciler", func() {
	var (
		clusterName   string
		testStartTime time.Time
	)

	BeforeEach(func() {
		testStartTime = time.Now()
	})

	JustAfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			PrintDiagnosis(Namespace, testStartTime, clusterName)
		}
	})

	AfterEach(func() {
		DeleteCassandraResourcesForClusters(Namespace, clusterName)
	})

	It("should eventually return the cassandra container definitions back to the desired state if the env vars are changed", func() {
		// given
		clusterName = AClusterName()

		AClusterWithName(clusterName).
			AndPodSpec(PodSpec().Build()).
			AndRacks([]v1alpha1.Rack{RackWithEmptyDir("a", 1)}).
			WithoutCustomConfig().
			IsDefined()

		By("verifying the rack statefulset has been created and the pods are ready with expected env var")
		Eventually(StatefulSetsForCluster(Namespace, clusterName), NodeStartDuration, CheckInterval).Should(
			ContainElement(HaveContainer("cassandra", &ContainerEnvExpectation{
				CassEnv: map[string]string{"EXTRA_CLASSPATH": "/extra-lib/cassandra-seed-provider.jar"},
			})))

		Eventually(PodReadyForCluster(Namespace, clusterName), NodeStartDuration, CheckInterval).
			Should(Equal(1), fmt.Sprintf("For cluster %s", clusterName))

		//when
		By("Modifying fields directly in the rack statefulset resource")
		modifiedEnvVars := []coreV1.EnvVar{
			coreV1.EnvVar{Name: "ExternalModName", Value: "ExternalModValue"},
			coreV1.EnvVar{Name: "EXTRA_CLASSPATH", Value: "/extra-lib/cassandra-seed-provider.jar"}, // Required or container crashes
		}

		TheStatefulSetContainerEnvVarsHaveChanged(Namespace, fmt.Sprintf("%s-a", clusterName), "cassandra", modifiedEnvVars)

		Eventually(StatefulSetsForCluster(Namespace, clusterName), NodeStartDuration, CheckInterval).Should(
			ContainElement(HaveContainer("cassandra", &ContainerEnvExpectation{
				CassEnv: map[string]string{
					"ExternalModName": "ExternalModValue",
					"EXTRA_CLASSPATH": "/extra-lib/cassandra-seed-provider.jar",
				},
			})))

		//then
		By("The statefulset should be returned to the original state. Note this is different to defined Cassandra Resource as we automatically add EXTRA_CLASSPATH")
		Eventually(StatefulSetsForCluster(Namespace, clusterName), NodeStartDuration*2, CheckInterval).Should(
			ContainElement(HaveContainer("cassandra", &ContainerEnvExpectation{
				CassEnv: map[string]string{
					"ExternalModName": "ExternalModValue",
					"EXTRA_CLASSPATH": "/extra-lib/cassandra-seed-provider.jar",
				},
			})))

	})

})
