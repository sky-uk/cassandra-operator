package modification

import (
	"fmt"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test/apis"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/ptr"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test"
	. "github.com/sky-uk/cassandra-operator/cassandra-operator/test/e2e"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test/e2e/parallel"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/watch"
)

var (
	resources          *parallel.ResourceSemaphore
	resourcesToReclaim int
)

func TestModification(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "E2E Suite (Modification Tests)", test.CreateParallelReporters("e2e_modification"))
}

var _ = ParallelTestBeforeSuite(func() []TestCluster {
	// initialise the resources available just once for the entire test suite
	resources = parallel.NewResourceSemaphore(MaxCassandraNodesPerNamespace)
	return []TestCluster{}
}, func(clusterNames []string) {
	// instantiate the accessor to the resource file for each spec,
	// so they can make use of it to acquire / release resources
	resources = parallel.NewUnInitialisedResourceSemaphore(MaxCassandraNodesPerNamespace)
})

func registerResourcesUsed(size int) {
	resourcesToReclaim = size
	resources.AcquireResource(size)
}

var _ = Context("Allowable cluster modifications", func() {
	var (
		clusterName   string
		podEvents     *PodEventLog
		podWatcher    watch.Interface
		testStartTime time.Time
	)

	BeforeEach(func() {
		testStartTime = time.Now()
		clusterName = AClusterName()
		podEvents, podWatcher = WatchPodEvents(Namespace, clusterName)
	})

	JustAfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			PrintDiagnosis(Namespace, testStartTime, clusterName)
		}
	})

	AfterEach(func() {
		podWatcher.Stop()
	})

	AfterEach(func() {
		DeleteCassandraResourcesForClusters(Namespace, clusterName)
		resources.ReleaseResource(resourcesToReclaim)
	})

	It("should allow modification of pod spec", func() {
		// given
		registerResourcesUsed(2)
		livenessProbeTimeoutSeconds := CassandraLivenessTimeout + 1
		readinessProbeTimeoutSeconds := CassandraReadinessTimeout + 1
		racks := []v1alpha1.Rack{Rack("a", 1), Rack("b", 1)}
		AClusterWithName(clusterName).
			AndRacks(racks).Exists()

		// when
		revisionsBeforeUpdate := statefulSetRevisions(clusterName, racks)
		TheClusterPodSpecAreChangedTo(Namespace, clusterName, v1alpha1.Pod{
			BootstrapperImage: CassandraBootstrapperImageName,
			SidecarImage:      CassandraSidecarImageName,
			Image:             CassandraImageName,
			Resources: coreV1.ResourceRequirements{
				Requests: coreV1.ResourceList{
					coreV1.ResourceMemory: resource.MustParse("999Mi"),
					coreV1.ResourceCPU:    resource.MustParse("1m"),
				},
				Limits: coreV1.ResourceList{
					coreV1.ResourceMemory: resource.MustParse("999Mi"),
				},
			},
			LivenessProbe: &v1alpha1.Probe{
				FailureThreshold:    ptr.Int32(CassandraLivenessProbeFailureThreshold + 1),
				InitialDelaySeconds: ptr.Int32(CassandraInitialDelay),
				PeriodSeconds:       ptr.Int32(CassandraLivenessPeriod),
				TimeoutSeconds:      ptr.Int32(livenessProbeTimeoutSeconds),
			},
			ReadinessProbe: &v1alpha1.Probe{
				FailureThreshold:    ptr.Int32(CassandraReadinessProbeFailureThreshold + 1),
				InitialDelaySeconds: ptr.Int32(CassandraInitialDelay),
				PeriodSeconds:       ptr.Int32(CassandraReadinessPeriod),
				TimeoutSeconds:      ptr.Int32(readinessProbeTimeoutSeconds),
			},
		})

		// then
		By("restarting each pod within the cluster")
		By("updating only the CPU and memory for each pod within the cluster")
		Eventually(PodsForCluster(Namespace, clusterName), 2*NodeRestartDuration, CheckInterval).Should(Each(And(
			HaveDifferentRevisionTo(revisionsBeforeUpdate),
			HaveContainer("cassandra", &ContainerExpectation{
				ImageName:                      *CassandraImageName,
				MemoryRequest:                  "999Mi",
				MemoryLimit:                    "999Mi",
				CPURequest:                     "1m",
				LivenessProbeFailureThreshold:  CassandraLivenessProbeFailureThreshold + 1,
				LivenessProbeInitialDelay:      DurationSeconds(CassandraInitialDelay),
				LivenessProbePeriod:            DurationSeconds(CassandraLivenessPeriod),
				LivenessProbeTimeout:           DurationSeconds(livenessProbeTimeoutSeconds),
				ReadinessProbeFailureThreshold: CassandraReadinessProbeFailureThreshold + 1,
				ReadinessProbeInitialDelay:     DurationSeconds(CassandraInitialDelay),
				ReadinessProbePeriod:           DurationSeconds(CassandraReadinessPeriod),
				ReadinessProbeTimeout:          DurationSeconds(readinessProbeTimeoutSeconds),
				ReadinessProbeSuccessThreshold: 1,
				ContainerPorts:                 map[string]int{"internode": 7000, "jmx-exporter": 7070, "cassandra-jmx": 7199, "jolokia": 7777, "client": 9042},
			}),
		)))

		By("restarting one stateful set at a time")
		Expect(podEvents.PodsNotDownAtTheSameTime(PodName(clusterName, "a", 0), PodName(clusterName, "b", 0))).To(BeTrue())
	})

	It("should allow the number of pods per rack to be scaled up", func() {
		// given
		registerResourcesUsed(3)
		AClusterWithName(clusterName).
			AndRacks([]v1alpha1.Rack{RackWithEmptyDir("a", 1), RackWithEmptyDir("b", 1)}).
			Exists()

		// when
		TheRackReplicationIsChangedTo(Namespace, clusterName, "a", 2)

		// then
		By("creating a new pod within the cluster rack")
		Eventually(PodReadyForCluster(Namespace, clusterName), NodeStartDuration, CheckInterval).
			Should(Equal(3), fmt.Sprintf("For cluster %s", clusterName))
		Expect(RacksForCluster(Namespace, clusterName)()).Should(And(
			HaveLen(2),
			HaveKeyWithValue("a", []string{PodName(clusterName, "a", 0), PodName(clusterName, "a", 1)}),
			HaveKeyWithValue("b", []string{PodName(clusterName, "b", 0)})))

		By("not restarting the other pods within the cluster")
		Expect(podEvents.PodsStartedEventCount(PodName(clusterName, "a", 0))).To(Equal(1))
		Expect(podEvents.PodsStartedEventCount(PodName(clusterName, "b", 0))).To(Equal(1))
	})

	It("should create a new stateful set when a new rack is added to the cluster definition", func() {
		// given
		registerResourcesUsed(2)
		AClusterWithName(clusterName).AndRacks([]v1alpha1.Rack{RackWithEmptyDir("a", 1)}).Exists()
		rackAHash := ClusterConfigHashForRack(Namespace, clusterName, "a")

		// when
		ANewRackIsAddedForCluster(Namespace, clusterName, RackWithEmptyDir("b", 1))

		// then
		By("adding a new rack to the cluster")
		Eventually(RacksForCluster(Namespace, clusterName), NodeStartDuration, CheckInterval).Should(And(
			HaveLen(2),
			HaveKeyWithValue("a", []string{PodName(clusterName, "a", 0)}),
			HaveKeyWithValue("b", []string{PodName(clusterName, "b", 0)}),
		))
		// config hash should be propagated to new rack
		Expect(PodsForCluster(Namespace, clusterName)()).Should(Each(And(
			HaveAnnotation("clusterConfigHash"),
			HaveAnnotationValue(AnnotationValueAssertion{Name: "clusterConfigHash", Value: rackAHash}),
		)))

		By("reporting metrics for the existing and new racks")
		Eventually(OperatorMetrics(Namespace), 3*time.Minute, CheckInterval).Should(ReportAClusterWith([]MetricAssertion{
			ClusterSizeMetric(Namespace, clusterName, 2),
			LiveAndNormalNodeMetric(Namespace, clusterName, PodName(clusterName, "a", 0), "a", 1),
			LiveAndNormalNodeMetric(Namespace, clusterName, PodName(clusterName, "b", 0), "b", 1),
		}))
	})

	It("should allow the cpu request and limits to be removed", func() {
		cpuDefaultLimit := findDefaultForCPU(Namespace, "limit")
		cpuDefaultRequest := findDefaultForCPU(Namespace, "request")

		// given
		registerResourcesUsed(1)
		AClusterWithName(clusterName).
			AndRacks([]v1alpha1.Rack{RackWithEmptyDir("a", 1)}).
			AndPodSpec(PodSpec().
				WithResources(&coreV1.ResourceRequirements{
					Requests: coreV1.ResourceList{
						coreV1.ResourceMemory: resource.MustParse("999Mi"),
						coreV1.ResourceCPU:    resource.MustParse("1m"),
					},
					Limits: coreV1.ResourceList{
						coreV1.ResourceMemory: resource.MustParse("999Mi"),
						coreV1.ResourceCPU:    resource.MustParse("1"),
					},
				}).
				Build()).
			Exists()

		// when
		TheClusterPodResourcesSpecAreChangedTo(Namespace, clusterName, coreV1.ResourceRequirements{
			Requests: coreV1.ResourceList{
				coreV1.ResourceMemory: resource.MustParse("999Mi"),
			},
			Limits: coreV1.ResourceList{
				coreV1.ResourceMemory: resource.MustParse("999Mi"),
			},
		})

		// then
		Eventually(PodsForCluster(Namespace, clusterName), NodeRestartDuration, CheckInterval).Should(Each(And(
			HaveResourcesRequirements(&ResourceRequirementsAssertion{
				ContainerName: "cassandra",
				MemoryRequest: ptr.String("999Mi"),
				MemoryLimit:   ptr.String("999Mi"),
				CPURequest:    cpuDefaultRequest,
				CPULimit:      cpuDefaultLimit,
			}),
		)))
	})

	Context("rack storage", func() {

		var rackStorage []apis.StorageBuilder

		BeforeEach(func() {
			// given
			registerResourcesUsed(1)
			rackStorage = []apis.StorageBuilder{}
			rackStorage = append(rackStorage, apis.APersistentVolume().
				OfSize("100Mi").
				WithStorageClass("standard-zone-a").
				AtPath("/var/lib/cassandra"))
			rackStorage = append(rackStorage, apis.AnEmptyDir().AtPath("/var/lib/my-empty-dir"))

			AClusterWithName(clusterName).
				AndRacks([]v1alpha1.Rack{apis.ARack("a", 1).
					WithZone("eu-west-1a").
					WithStorages(rackStorage...).
					Build(),
				}).
				Exists()
		})

		It("should allow the addition of a rack storage", func() {
			// when
			rackStorage = append(rackStorage, apis.AnEmptyDir().AtPath("/var/lib/my-other-empty-dir"))
			TheRackStorageIsChangedTo(Namespace, clusterName, "a", buildStorage(rackStorage))

			// then
			By("creating the additional emptyDir volume at the given path")
			Eventually(PodsForCluster(Namespace, clusterName), NodeRestartDuration, CheckInterval).Should(And(
				Each(HaveEmptyDirVolumeMountAtPath("/var/lib/my-empty-dir")),
				Each(HaveEmptyDirVolumeMountAtPath("/var/lib/my-other-empty-dir"))))

			By("preserving the persistent volume at the given path")
			Eventually(StatefulSetsForCluster(Namespace, clusterName), NodeRestartDuration, CheckInterval).Should(
				Each(HavePersistentVolumeMountAtPath("/var/lib/cassandra")))
			Eventually(PodReadyForCluster(Namespace, clusterName), NodeStartDuration, CheckInterval).Should(Equal(1))
		})

		It("should allow the removal of an emptyDir", func() {
			// when
			rackStorage = rackStorage[:1]
			TheRackStorageIsChangedTo(Namespace, clusterName, "a", buildStorage(rackStorage))

			// then
			By("removing the emptyDir")
			Eventually(PodsForCluster(Namespace, clusterName), NodeRestartDuration, CheckInterval).Should(And(
				Each(Not(HaveEmptyDirVolumeMountAtPath("/var/lib/my-empty-dir")))))

			By("preserving the persistent volume at the given path")
			Eventually(StatefulSetsForCluster(Namespace, clusterName), NodeRestartDuration, CheckInterval).Should(
				Each(HavePersistentVolumeMountAtPath("/var/lib/cassandra")))
			Eventually(PodReadyForCluster(Namespace, clusterName), NodeStartDuration, CheckInterval).Should(Equal(1))
		})
	})

	Context("cluster config file changes", func() {
		It("should trigger a rolling restart of the cluster stateful set when a custom config file is changed", func() {
			// given
			registerResourcesUsed(2)
			racks := []v1alpha1.Rack{Rack("a", 1), Rack("b", 1)}
			AClusterWithName(clusterName).
				AndRacks(racks).
				Exists()
			configHashBeforeUpdate := ClusterConfigHashForRack(Namespace, clusterName, "a")

			// when
			modificationTime := time.Now()
			revisionsBeforeUpdate := statefulSetRevisions(clusterName, racks)
			TheCustomJVMOptionsConfigIsChangedForCluster(Namespace, clusterName, DefaultJvmOptionsWithLine("-Dcluster.test.flag=true"))

			// then
			By("registering an event for the custom config modification")
			Eventually(CassandraEventsFor(Namespace, clusterName), EventPublicationTimeout, CheckInterval).Should(HaveEvent(EventExpectation{
				Type:                 coreV1.EventTypeNormal,
				Reason:               cluster.ClusterUpdateEvent,
				Message:              fmt.Sprintf("Custom config updated for cluster %s.%s", Namespace, clusterName),
				LastTimestampCloseTo: &modificationTime,
			}))

			By("applying the config changes to each pod")
			Eventually(PodsForCluster(Namespace, clusterName), 2*NodeRestartDuration, CheckInterval).Should(Each(And(
				HaveDifferentRevisionTo(revisionsBeforeUpdate),
				HaveJVMArg("-Dcluster.test.flag=true"),
				HaveAnnotation("clusterConfigHash"),
				Not(HaveAnnotationValue(AnnotationValueAssertion{Name: "clusterConfigHash", Value: configHashBeforeUpdate})),
			)))
			Eventually(PodReadinessStatus(Namespace, PodName(clusterName, "a", 0)), NodeRestartDuration, CheckInterval).Should(BeTrue())
			Eventually(PodReadinessStatus(Namespace, PodName(clusterName, "b", 0)), NodeRestartDuration, CheckInterval).Should(BeTrue())

			By("restarting one statefulset at a time")
			Expect(podEvents.PodsNotDownAtTheSameTime(PodName(clusterName, "a", 0), PodName(clusterName, "b", 0))).To(BeTrue())
		})

		It("should trigger a rolling restarts of the cluster stateful set when a custom config file is added", func() {
			// given
			registerResourcesUsed(2)
			racks := []v1alpha1.Rack{Rack("a", 1), Rack("b", 1)}
			AClusterWithName(clusterName).
				AndRacks(racks).
				WithoutCustomConfig().Exists()

			// when
			modificationTime := time.Now()
			revisionsBeforeUpdate := statefulSetRevisions(clusterName, racks)
			extraConfigFile := &ExtraConfigFile{Name: "customConfigFile", Content: "some content"}
			TheCustomConfigIsAddedForCluster(Namespace, clusterName, extraConfigFile)

			// then
			By("registering an event for the custom config addition")
			Eventually(CassandraEventsFor(Namespace, clusterName), EventPublicationTimeout, CheckInterval).Should(HaveEvent(EventExpectation{
				Type:                 coreV1.EventTypeNormal,
				Reason:               cluster.ClusterUpdateEvent,
				Message:              fmt.Sprintf("Custom config created for cluster %s.%s", Namespace, clusterName),
				LastTimestampCloseTo: &modificationTime,
			}))

			By("applying the config changes to each pod")
			Eventually(PodsForCluster(Namespace, clusterName), 2*NodeRestartDuration, CheckInterval).Should(Each(And(
				HaveDifferentRevisionTo(revisionsBeforeUpdate),
				HaveVolumeForConfigMap(fmt.Sprintf("%s-config", clusterName)),
				HaveAnnotation("clusterConfigHash"),
			)))
			Eventually(PodReadinessStatus(Namespace, PodName(clusterName, "a", 0)), NodeRestartDuration, CheckInterval).Should(BeTrue())
			Eventually(PodReadinessStatus(Namespace, PodName(clusterName, "b", 0)), NodeRestartDuration, CheckInterval).Should(BeTrue())

			By("restarting one statefulset at a time")
			Expect(podEvents.PodsNotDownAtTheSameTime(PodName(clusterName, "a", 0), PodName(clusterName, "b", 0))).To(BeTrue())
		})

		It("should trigger a rolling restarts of the cluster stateful set when the custom config file is deleted", func() {
			// given
			registerResourcesUsed(2)
			extraConfigFile := &ExtraConfigFile{Name: "customConfigFile", Content: "some content"}
			racks := []v1alpha1.Rack{Rack("a", 1), Rack("b", 1)}
			AClusterWithName(clusterName).
				AndRacks(racks).
				AndCustomConfig(extraConfigFile).Exists()

			// when
			modificationTime := time.Now()
			revisionsBeforeUpdate := statefulSetRevisions(clusterName, racks)
			TheCustomConfigIsDeletedForCluster(Namespace, clusterName)

			// then
			By("registering an event for the custom config deletion")
			Eventually(CassandraEventsFor(Namespace, clusterName), EventPublicationTimeout, CheckInterval).Should(HaveEvent(EventExpectation{
				Type:                 coreV1.EventTypeNormal,
				Reason:               cluster.ClusterUpdateEvent,
				Message:              fmt.Sprintf("Custom config deleted for cluster %s.%s", Namespace, clusterName),
				LastTimestampCloseTo: &modificationTime,
			}))

			By("applying the config changes to each pod")
			Eventually(PodsForCluster(Namespace, clusterName), 2*NodeRestartDuration, CheckInterval).Should(Each(And(
				HaveDifferentRevisionTo(revisionsBeforeUpdate),
				Not(HaveVolumeForConfigMap(fmt.Sprintf("%s-config", clusterName))),
				Not(HaveAnnotation("clusterConfigHash")),
			)))
			Eventually(PodReadinessStatus(Namespace, PodName(clusterName, "a", 0)), NodeRestartDuration, CheckInterval).Should(BeTrue())
			Eventually(PodReadinessStatus(Namespace, PodName(clusterName, "b", 0)), NodeRestartDuration, CheckInterval).Should(BeTrue())

			By("restarting one stateful set at a time")
			Expect(podEvents.PodsNotDownAtTheSameTime(PodName(clusterName, "a", 0), PodName(clusterName, "b", 0))).To(BeTrue())
		})

		It("should allow the cluster to be created once the invalid spec has been corrected", func() {
			// given
			registerResourcesUsed(1)
			AClusterWithName(clusterName).WithoutRacks().WithoutCustomConfig().IsDefined()

			// when
			ANewRackIsAddedForCluster(Namespace, clusterName, RackWithEmptyDir("a", 1))

			// then
			Eventually(PodReadyForCluster(Namespace, clusterName), NodeStartDuration, CheckInterval).Should(Equal(1))
		})
	})
})

func buildStorage(builders []apis.StorageBuilder) []v1alpha1.Storage {
	var rackStorage []v1alpha1.Storage
	for _, builder := range builders {
		rackStorage = append(rackStorage, builder.Build())
	}
	return rackStorage
}

func statefulSetRevisions(clusterName string, racks []v1alpha1.Rack) map[string]string {
	m := map[string]string{}
	for _, rack := range racks {
		revision, err := StatefulSetRevision(Namespace, fmt.Sprintf("%s-%s", clusterName, rack.Name))
		Expect(err).To(BeNil())
		m[rack.Name] = revision
	}
	return m
}

func findDefaultForCPU(namespace, defaultType string) *string {
	limitRanges, err := KubeClientset.CoreV1().LimitRanges(namespace).List(metaV1.ListOptions{})
	Expect(err).ToNot(HaveOccurred())

	for _, lr := range limitRanges.Items {
		for _, lri := range lr.Spec.Limits {
			if lri.Type == coreV1.LimitTypeContainer {
				var resourceList coreV1.ResourceList
				if defaultType == "limit" {
					resourceList = lri.Default
				} else {
					resourceList = lri.DefaultRequest
				}

				if value, ok := resourceList[coreV1.ResourceCPU]; ok {
					return ptr.String(value.String())
				}
			}
		}
	}

	return nil
}
