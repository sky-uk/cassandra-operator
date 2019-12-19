package creation

import (
	"fmt"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/imageversion"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test/apis"
	"path/filepath"
	"testing"
	"time"

	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test"
	"k8s.io/apimachinery/pkg/api/resource"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/ptr"
	. "github.com/sky-uk/cassandra-operator/cassandra-operator/test/e2e"

	coreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	multipleRacksCluster          *TestCluster
	emptyDirCluster               *TestCluster
	testStartTime                 time.Time
	multipleRacksClusterReadyTime time.Time
)

func TestCreation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "E2E Suite (Creation Tests)", test.CreateParallelReporters("e2e_creation"))
}

func defineClusters(multipleRacksClusterName, emptyDirClusterName string) (multipleRacksCluster, emptyDirCluster *TestCluster) {
	multipleRacksCluster = &TestCluster{
		Name: multipleRacksClusterName,
		Racks: []v1alpha1.Rack{
			apis.ARack("a", 2).
				WithZone("eu-west-1a").
				WithStorage(apis.APersistentVolume().
					OfSize("100Mi").
					WithStorageClass("standard-zone-a").
					AtPath("/var/lib/cassandra")).
				WithStorage(apis.AnEmptyDir().
					AtPath("/var/log/empty-dir-multi")).
				Build(),
			apis.ARack("b", 1).
				WithZone("eu-west-1b").
				WithStorage(apis.APersistentVolume().
					OfSize("100Mi").
					WithStorageClass("standard-zone-b").
					AtPath("/var/lib/cassandra")).
				WithStorage(apis.AnEmptyDir().
					AtPath("/var/log/empty-dir-multi")).
				Build(),
		},
		ExtraConfigFileName: "extraConfigFile",
	}
	emptyDirCluster = &TestCluster{
		Name: emptyDirClusterName,
		Racks: []v1alpha1.Rack{
			apis.ARack("a", 1).
				WithZone("eu-west-1a").
				WithStorage(apis.AnEmptyDir().AtPath("/var/lib/cassandra")).
				WithStorage(apis.AnEmptyDir().AtPath("/var/log/emptydir")).
				Build(),
		},
	}
	return
}

func createClustersInParallel(multipleRacksCluster, emptyDirCluster *TestCluster) {
	extraFile := &ExtraConfigFile{Name: multipleRacksCluster.ExtraConfigFileName, Content: "some content"}

	AClusterWithName(multipleRacksCluster.Name).
		AndCustomConfig(extraFile).
		AndClusterSpec(&v1alpha1.CassandraSpec{
			Datacenter: ptr.String("custom-dc"),
			Racks:      multipleRacksCluster.Racks,
			Pod: v1alpha1.Pod{
				BootstrapperImage: CassandraBootstrapperImageName,
				SidecarImage:      CassandraSidecarImageName,
				Image:             CassandraImageName,
				Resources: coreV1.ResourceRequirements{
					Requests: coreV1.ResourceList{
						coreV1.ResourceMemory: resource.MustParse("987Mi"),
						coreV1.ResourceCPU:    resource.MustParse("1m"),
					},
					Limits: coreV1.ResourceList{
						coreV1.ResourceMemory: resource.MustParse("987Mi"),
					},
				},
				LivenessProbe: &v1alpha1.Probe{
					FailureThreshold:    ptr.Int32(CassandraLivenessProbeFailureThreshold),
					TimeoutSeconds:      ptr.Int32(CassandraLivenessTimeout + 2),
					InitialDelaySeconds: ptr.Int32(CassandraInitialDelay),
					PeriodSeconds:       ptr.Int32(CassandraLivenessPeriod),
				},
				ReadinessProbe: &v1alpha1.Probe{
					FailureThreshold:    ptr.Int32(CassandraReadinessProbeFailureThreshold),
					TimeoutSeconds:      ptr.Int32(CassandraReadinessTimeout + 1),
					InitialDelaySeconds: ptr.Int32(CassandraInitialDelay),
					PeriodSeconds:       ptr.Int32(CassandraReadinessPeriod),
				},
			},
		}).IsDefined()

	AClusterWithName(emptyDirCluster.Name).AndRacks(emptyDirCluster.Racks).WithoutCustomConfig().IsDefined()
}

var _ = ParallelTestBeforeSuite(func() []TestCluster {
	multipleRacksCluster, emptyDirCluster = defineClusters(AClusterName(), AClusterName())
	createClustersInParallel(multipleRacksCluster, emptyDirCluster)
	return []TestCluster{*multipleRacksCluster, *emptyDirCluster}
}, func(clusterNames []string) {
	multipleRacksCluster, emptyDirCluster = defineClusters(clusterNames[0], clusterNames[1])
})

var _ = Context("When a cluster doesn't already exist", func() {

	BeforeEach(func() {
		testStartTime = time.Now()
		Eventually(PodReadyForCluster(Namespace, multipleRacksCluster.Name), 3*NodeStartDuration, CheckInterval).
			Should(Equal(3), fmt.Sprintf("For cluster %s", multipleRacksCluster.Name))
		multipleRacksClusterReadyTime = time.Now()
		Eventually(PodReadyForCluster(Namespace, emptyDirCluster.Name), NodeStartDuration, CheckInterval).
			Should(Equal(1), fmt.Sprintf("For cluster %s", emptyDirCluster.Name))
	})

	JustAfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			PrintDiagnosis(Namespace, testStartTime, multipleRacksCluster.Name, emptyDirCluster.Name)
		}
	})

	It("should create all the required Kubernetes resources according to the cassandra specs", func() {
		// then
		By("creating all pods only once")
		Expect(PodRestartForCluster(Namespace, multipleRacksCluster.Name)()).Should(Equal(0))

		By("creating pods with the specified resources")
		Expect(PodsForCluster(Namespace, multipleRacksCluster.Name)()).Should(Each(And(
			HaveContainer("cassandra", &ContainerExpectation{
				ImageName:                      *CassandraImageName,
				MemoryRequest:                  "987Mi",
				MemoryLimit:                    "987Mi",
				CPURequest:                     "1m",
				LivenessProbePeriod:            DurationSeconds(CassandraLivenessPeriod),
				LivenessProbeFailureThreshold:  CassandraLivenessProbeFailureThreshold,
				LivenessProbeInitialDelay:      DurationSeconds(CassandraInitialDelay),
				LivenessProbeTimeout:           DurationSeconds(CassandraLivenessTimeout + 2),
				ReadinessProbeTimeout:          DurationSeconds(CassandraReadinessTimeout + 1),
				ReadinessProbePeriod:           DurationSeconds(CassandraReadinessPeriod),
				ReadinessProbeFailureThreshold: CassandraReadinessProbeFailureThreshold,
				ReadinessProbeInitialDelay:     DurationSeconds(CassandraInitialDelay),
				ReadinessProbeSuccessThreshold: 1,
				ContainerPorts:                 map[string]int{"internode": 7000, "jmx-exporter": 7070, "cassandra-jmx": 7199, "jolokia": 7777, "client": 9042}})),
		))

		By("creating a StatefulSet for each rack")
		Expect(StatefulSetsForCluster(Namespace, multipleRacksCluster.Name)()).Should(Each(And(
			BeCreatedWithServiceName(multipleRacksCluster.Name),
			HaveLabel("app.kubernetes.io/name", multipleRacksCluster.Name),
			HaveLabel("app.kubernetes.io/instance", fmt.Sprintf("%s.%s", Namespace, multipleRacksCluster.Name)),
			HaveLabel("app.kubernetes.io/managed-by", "cassandra-operator"),
		)))

		By("creating a headless service for the cluster")
		Expect(HeadlessServiceForCluster(Namespace, multipleRacksCluster.Name)()).Should(And(
			Not(BeNil()),
			HaveLabel("app.kubernetes.io/name", multipleRacksCluster.Name),
			HaveLabel("app.kubernetes.io/instance", fmt.Sprintf("%s.%s", Namespace, multipleRacksCluster.Name)),
			HaveLabel("app.kubernetes.io/managed-by", "cassandra-operator"),
		))

		By("creating a persistent volume claim for each StatefulSet with the requested storage capacity")
		Expect(PersistentVolumeClaimsForCluster(Namespace, multipleRacksCluster.Name)()).Should(And(
			HaveLen(3),
			//Each(HaveLabel("app.kubernetes.io/name", multipleRacksCluster.Name)),
			Each(HaveLabel("app.kubernetes.io/instance", fmt.Sprintf("%s.%s", Namespace, multipleRacksCluster.Name))),
			Each(HaveLabel("app.kubernetes.io/managed-by", "cassandra-operator")),
			Each(HaveStorageCapacity("100Mi"))))

		if !UseMockedImage {
			By("creating a cluster with the specified datacenter")
			Eventually(DataCenterForCluster(Namespace, multipleRacksCluster.Name), NodeStartDuration, CheckInterval).Should(Equal("custom-dc"))
		}
	})

	It("should copy custom config files into a cassandra config directory within pods", func() {
		Expect(FileExistsInConfigurationDirectory(Namespace, PodName(multipleRacksCluster.Name, "a", 0), filepath.Base(multipleRacksCluster.ExtraConfigFileName))()).To(BeTrue())
	})

	It("should spread out the nodes in different locations", func() {
		By("creating as many racks as locations")
		By("creating the same number of nodes for each rack")
		Expect(RacksForCluster(Namespace, multipleRacksCluster.Name)()).Should(And(
			HaveLen(2),
			HaveKeyWithValue("a", []string{PodName(multipleRacksCluster.Name, "a", 0), PodName(multipleRacksCluster.Name, "a", 1)}),
			HaveKeyWithValue("b", []string{PodName(multipleRacksCluster.Name, "b", 0)})))
	})

	It("should create the pods on different nodes", func() {
		Expect(UniqueNodesUsed(Namespace, multipleRacksCluster.Name)).Should(HaveLen(3))
	})

	It("should setup the correct persistent volumes in the cassandra containers", func() {
		By("creating persistent volume at the given path")
		Expect(StatefulSetsForCluster(Namespace, multipleRacksCluster.Name)()).Should(And(
			Each(HavePersistentVolumeMountAtPath("/var/lib/cassandra"))))
	})

	It("should setup the correct empty dir volumes in the cassandra containers", func() {
		By("creating the emptyDir volumes at the given path")
		Expect(PodsForCluster(Namespace, multipleRacksCluster.Name)()).Should(
			Each(HaveEmptyDirVolumeMountAtPath("/var/log/empty-dir-multi")))
		Expect(PodsForCluster(Namespace, emptyDirCluster.Name)()).Should(And(
			Each(HaveEmptyDirVolumeMountAtPath("/var/lib/cassandra")),
			Each(HaveEmptyDirVolumeMountAtPath("/var/log/emptydir"))))

		By("not creating a persistent volume claim")
		Expect(PersistentVolumeClaimsForCluster(Namespace, emptyDirCluster.Name)()).Should(BeEmpty())
	})

	It("should add an annotation named customConfigHash for clusters with custom config", func() {
		Expect(PodsForCluster(Namespace, multipleRacksCluster.Name)()).Should(Each(
			HaveAnnotation("clusterConfigHash"),
		))
	})

	It("should not add an annotation named customConfigHash for clusters without custom config", func() {
		Expect(PodsForCluster(Namespace, emptyDirCluster.Name)()).Should(Each(
			Not(HaveAnnotation("clusterConfigHash")),
		))
	})

	It("should report metrics for all clusters", func() {
		By("exposing node and rack information in metrics")
		Eventually(OperatorMetrics(Namespace), 60*time.Second, CheckInterval).Should(ReportAClusterWith([]MetricAssertion{
			ClusterSizeMetric(Namespace, multipleRacksCluster.Name, 3),
			LiveAndNormalNodeMetric(Namespace, multipleRacksCluster.Name, PodName(multipleRacksCluster.Name, "a", 0), "a", 1),
			LiveAndNormalNodeMetric(Namespace, multipleRacksCluster.Name, PodName(multipleRacksCluster.Name, "a", 1), "a", 1),
			LiveAndNormalNodeMetric(Namespace, multipleRacksCluster.Name, PodName(multipleRacksCluster.Name, "b", 0), "b", 1),
		}))

		By("exposing metrics for the other cluster")
		Eventually(OperatorMetrics(Namespace), 60*time.Second, CheckInterval).Should(ReportAClusterWith([]MetricAssertion{
			ClusterSizeMetric(Namespace, emptyDirCluster.Name, 1),
			LiveAndNormalNodeMetric(Namespace, emptyDirCluster.Name, PodName(emptyDirCluster.Name, "a", 0), "a", 1),
		}))
	})

	It("should generate events informing of cluster creation", func() {
		By("registering an event while each stateful set is being created")
		Expect(CassandraEventsFor(Namespace, multipleRacksCluster.Name)()).Should(HaveEvent(EventExpectation{
			Type:    coreV1.EventTypeNormal,
			Reason:  cluster.WaitingForStatefulSetChange,
			Message: fmt.Sprintf("Waiting for stateful set %s.%s-%s to be ready", Namespace, multipleRacksCluster.Name, multipleRacksCluster.Racks[0].Name),
		}))
		Expect(CassandraEventsFor(Namespace, multipleRacksCluster.Name)()).Should(HaveEvent(EventExpectation{
			Type:    coreV1.EventTypeNormal,
			Reason:  cluster.WaitingForStatefulSetChange,
			Message: fmt.Sprintf("Waiting for stateful set %s.%s-%s to be ready", Namespace, multipleRacksCluster.Name, multipleRacksCluster.Racks[1].Name),
		}))

		By("registering an event when each stateful set creation is complete")
		Expect(CassandraEventsFor(Namespace, multipleRacksCluster.Name)()).Should(HaveEvent(EventExpectation{
			Type:    coreV1.EventTypeNormal,
			Reason:  cluster.StatefulSetChangeComplete,
			Message: fmt.Sprintf("Stateful set %s.%s-%s is ready", Namespace, multipleRacksCluster.Name, multipleRacksCluster.Racks[0].Name),
		}))
		// give long enough to ensure event is propagated
		Eventually(CassandraEventsFor(Namespace, multipleRacksCluster.Name), EventPublicationTimeout, CheckInterval).Should(HaveEvent(EventExpectation{
			Type:                 coreV1.EventTypeNormal,
			Reason:               cluster.StatefulSetChangeComplete,
			Message:              fmt.Sprintf("Stateful set %s.%s-%s is ready", Namespace, multipleRacksCluster.Name, multipleRacksCluster.Racks[1].Name),
			LastTimestampCloseTo: &multipleRacksClusterReadyTime, // cluster's ready when its last stateful set is
		}))
	})

})

var _ = Context("When a cluster definition does not specify custom images", func() {

	JustAfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			PrintDiagnosis(Namespace, testStartTime, multipleRacksCluster.Name, emptyDirCluster.Name)
		}
	})

	It("should set custom images version to the operator version", func() {
		// given
		clusterName := AClusterName()
		AClusterWithName(clusterName).
			AndPodSpec(PodSpec().WithoutBootstrapperImageName().WithoutSidecarImageName().Build()).
			AndRacks([]v1alpha1.Rack{RackWithEmptyDir("a", 1)}).
			WithoutCustomConfig().
			IsDefined()

		// then
		operatorRepository, operatorVersion := operatorImageRepositoryAndVersion(Namespace)
		Eventually(PodsForCluster(Namespace, clusterName), NodeStartDuration, CheckInterval).Should(Each(And(
			HaveContainer("cassandra", &ContainerImageExpectation{
				ImageName: *CassandraImageName,
			}),
			HaveContainer("cassandra-sidecar", &ContainerImageExpectation{
				ImageName: fmt.Sprintf("%s/cassandra-sidecar:%s", operatorRepository, operatorVersion),
			}),
			HaveInitContainer("cassandra-bootstrapper", &ContainerImageExpectation{
				ImageName: fmt.Sprintf("%s/cassandra-bootstrapper:%s", operatorRepository, operatorVersion),
			}),
		)))
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
