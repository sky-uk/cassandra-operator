package modification

import (
	"fmt"

	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/imageversion"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/ptr"
	"k8s.io/apimachinery/pkg/api/resource"

	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sky-uk/cassandra-operator/cassandra-operator/test/e2e"

	coreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Context("External sidecar statefulset modifications", func() {
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

	It("should eventually return the sidecar container definitions back to the desired state if the sidecar image is changed", func() {
		// given
		clusterName = AClusterName()

		AClusterWithName(clusterName).
			AndPodSpec(PodSpec().Build()).
			AndRacks([]v1alpha1.Rack{RackWithEmptyDir("a", 1)}).
			WithoutCustomConfig().
			IsDefined()

		By("verifying the rack statefulset has been created and the pods are ready")
		repository, version := operatorImageRepositoryAndVersion(Namespace)
		Eventually(StatefulSetsForCluster(Namespace, clusterName), NodeStartDuration, CheckInterval).Should(
			ContainElement(HaveContainer("cassandra-sidecar", &ContainerImageExpectation{
				ImageName: fmt.Sprintf("%s/cassandra-sidecar:%s", repository, version),
			})))

		Eventually(PodReadyForCluster(Namespace, clusterName), NodeStartDuration, CheckInterval).
			Should(Equal(1), fmt.Sprintf("For cluster %s", clusterName))

		//when
		By("Modifying fields directly in the rack statefulset resource")
		modifiedSidecarImage := "skyuk/cassandra-sidecar:latest"

		TheStatefulSetContainerImageNameIsChanged(Namespace, fmt.Sprintf("%s-a", clusterName), "cassandra-sidecar", modifiedSidecarImage)

		Eventually(StatefulSetsForCluster(Namespace, clusterName), NodeStartDuration, CheckInterval).Should(
			ContainElement(HaveContainer("cassandra-sidecar", &ContainerImageExpectation{
				ImageName: modifiedSidecarImage,
			})))

		//then
		By("The statefulset should be returned to the state defined in the cassandra resource")
		Eventually(StatefulSetsForCluster(Namespace, clusterName), NodeStartDuration*2, CheckInterval).Should(
			ContainElement(HaveContainer("cassandra-sidecar", &ContainerImageExpectation{
				ImageName: fmt.Sprintf("%s/cassandra-sidecar:%s", repository, version),
			})))

	})

	It("should eventually return the sidecar container definitions back to the desired state if the sidecar resources are changed", func() {
		// given
		clusterName = AClusterName()

		AClusterWithName(clusterName).
			AndPodSpec(PodSpec().Build()).
			AndRacks([]v1alpha1.Rack{RackWithEmptyDir("a", 1)}).
			WithoutCustomConfig().
			IsDefined()

		originalResourceRequirements := &ResourceRequirementsAssertion{
			ContainerName: "cassandra-sidecar",
			MemoryRequest: ptr.String("50Mi"),
			MemoryLimit:   ptr.String("50Mi"),
			CPURequest:    ptr.String("0"),
			CPULimit:      nil,
		}

		By("verifying the rack statefulset has been created and the pods are ready")
		Eventually(StatefulSetsForCluster(Namespace, clusterName), NodeStartDuration, CheckInterval).Should(
			ContainElement(HaveResourcesRequirements(originalResourceRequirements)))

		Eventually(PodReadyForCluster(Namespace, clusterName), NodeStartDuration, CheckInterval).
			Should(Equal(1), fmt.Sprintf("For cluster %s", clusterName))

		//when
		By("Modifying resource fields directly in the rack statefulset resource")
		modifiedMemory := "60Mi"
		modifiedCPU := "50m"
		editedResources := &coreV1.ResourceRequirements{
			Requests: coreV1.ResourceList{
				coreV1.ResourceMemory: resource.MustParse(modifiedMemory),
				coreV1.ResourceCPU:    resource.MustParse(modifiedCPU),
			},
			Limits: coreV1.ResourceList{
				coreV1.ResourceMemory: resource.MustParse(modifiedMemory),
			},
		}

		TheStatefulSetContainerResourcesChanged(Namespace, fmt.Sprintf("%s-a", clusterName), "cassandra-sidecar", *editedResources)

		Eventually(StatefulSetsForCluster(Namespace, clusterName), NodeStartDuration, CheckInterval).Should(
			ContainElement(HaveResourcesRequirements(&ResourceRequirementsAssertion{
				ContainerName: "cassandra-sidecar",
				MemoryRequest: ptr.String(modifiedMemory),
				MemoryLimit:   ptr.String(modifiedMemory),
				CPURequest:    ptr.String(modifiedCPU),
				CPULimit:      nil,
			})))

		//then
		By("The statefulset should be returned to the state defined in the cassandra resource")
		Eventually(StatefulSetsForCluster(Namespace, clusterName), NodeStartDuration, CheckInterval).Should(
			ContainElement(HaveResourcesRequirements(originalResourceRequirements)))
	})
})

func operatorImageRepositoryAndVersion(namespace string) (operatorRepository, operatorVersion string) {
	pods, err := KubeClientset.CoreV1().Pods(namespace).List(metaV1.ListOptions{LabelSelector: "app.kubernetes.io/name=cassandra-operator"})
	Expect(err).ToNot(HaveOccurred())
	Expect(pods.Items).To(HaveLen(1))
	operatorImage := pods.Items[0].Spec.Containers[0].Image
	operatorRepository = imageversion.RepositoryPath(&operatorImage)
	operatorVersion = imageversion.Version(&operatorImage)
	return
}
