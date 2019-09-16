package creation

import (
	"fmt"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
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
					AtPath("/var/lib/pv-cassandra-home")).
				Build(),
			apis.ARack("b", 1).
				WithZone("eu-west-1b").
				WithStorage(apis.APersistentVolume().
					OfSize("100Mi").
					WithStorageClass("standard-zone-b").
					AtPath("/var/lib/pv-cassandra-home")).
				Build(),
		},
		ExtraConfigFileName: "extraConfigFile",
	}
	emptyDirCluster = &TestCluster{
		Name: emptyDirClusterName,
		Racks: []v1alpha1.Rack{
			apis.ARack("a", 1).
				WithZone("eu-west-1a").
				WithStorage(apis.AnEmptyDir().AtPath("/var/lib/emptydir-cassandra-home")).
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
				BootstrapperImage: &CassandraBootstrapperImageName,
				SidecarImage:      &CassandraSidecarImageName,
				Image:             &CassandraImageName,
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
					TimeoutSeconds:      ptr.Int32(7),
					InitialDelaySeconds: ptr.Int32(CassandraInitialDelay),
					PeriodSeconds:       ptr.Int32(CassandraLivenessPeriod),
				},
				ReadinessProbe: &v1alpha1.Probe{
					FailureThreshold:    ptr.Int32(CassandraReadinessProbeFailureThreshold),
					TimeoutSeconds:      ptr.Int32(6),
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

var _ = Context("When a cluster with a given name doesn't already exist", func() {

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
			HaveContainer(ContainerExpectation{
				ImageName:                      CassandraImageName,
				ContainerName:                  "cassandra",
				MemoryRequest:                  "987Mi",
				MemoryLimit:                    "987Mi",
				CPURequest:                     "1m",
				LivenessProbePeriod:            DurationSeconds(CassandraLivenessPeriod),
				LivenessProbeFailureThreshold:  CassandraLivenessProbeFailureThreshold,
				LivenessProbeInitialDelay:      DurationSeconds(CassandraInitialDelay),
				LivenessProbeTimeout:           7 * time.Second,
				ReadinessProbeTimeout:          6 * time.Second,
				ReadinessProbePeriod:           DurationSeconds(CassandraReadinessPeriod),
				ReadinessProbeFailureThreshold: CassandraReadinessProbeFailureThreshold,
				ReadinessProbeInitialDelay:     DurationSeconds(CassandraInitialDelay),
				ReadinessProbeSuccessThreshold: 1,
				ContainerPorts:                 map[string]int{"internode": 7000, "jmx-exporter": 7070, "cassandra-jmx": 7199, "jolokia": 7777, "client": 9042}})),
		))

		By("creating a StatefulSet for each rack")
		Expect(StatefulSetsForCluster(Namespace, multipleRacksCluster.Name)()).Should(Each(And(
			BeCreatedWithServiceName(multipleRacksCluster.Name),
			HaveLabel("sky.uk/cassandra-operator", multipleRacksCluster.Name),
		)))

		By("creating a headless service for the cluster")
		Expect(HeadlessServiceForCluster(Namespace, multipleRacksCluster.Name)()).Should(And(
			Not(BeNil()),
			HaveLabel("sky.uk/cassandra-operator", multipleRacksCluster.Name)),
		)

		By("creating a persistent volume claim for each StatefulSet with the requested storage capacity")
		Expect(PersistentVolumeClaimsForCluster(Namespace, multipleRacksCluster.Name)()).Should(And(
			HaveLen(3),
			Each(HaveLabel("sky.uk/cassandra-operator", multipleRacksCluster.Name)),
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
		Expect(StatefulSetsForCluster(Namespace, multipleRacksCluster.Name)()).Should(
			Each(HavePersistentVolumeMountAtPath("/var/lib/pv-cassandra-home")))

		By("not creating an emptyDir volume at the same path")
		Expect(PodsForCluster(Namespace, multipleRacksCluster.Name)()).Should(
			Each(Not(HaveEmptyDirVolumeMountAtPath("/var/lib/pv-cassandra-home"))))
	})

	It("should setup the correct empty dir volumes in the cassandra containers", func() {
		By("creating an emptyDir volume at the given path")
		Expect(PodsForCluster(Namespace, emptyDirCluster.Name)()).Should(
			Each(HaveEmptyDirVolumeMountAtPath("/var/lib/emptydir-cassandra-home")))

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
		Eventually(CassandraEventsFor(Namespace, multipleRacksCluster.Name), 30*time.Second, CheckInterval).Should(HaveEvent(EventExpectation{
			Type:                 coreV1.EventTypeNormal,
			Reason:               cluster.StatefulSetChangeComplete,
			Message:              fmt.Sprintf("Stateful set %s.%s-%s is ready", Namespace, multipleRacksCluster.Name, multipleRacksCluster.Racks[1].Name),
			LastTimestampCloseTo: &multipleRacksClusterReadyTime, // cluster's ready when its last stateful set is
		}))
	})
})
