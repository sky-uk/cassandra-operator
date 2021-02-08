package cluster

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/ptr"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test/apis"
	"k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"testing"
)

func TestClusterState(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Cluster State Suite", test.CreateParallelReporters("clusterstate"))
}

var _ = Describe("the cluster state reconstruction", func() {

	var (
		clusterNamespaceName = types.NamespacedName{Namespace: "mynamespace", Name: "mycluster"}
		desiredCassandra     *v1alpha1.Cassandra
		expectedCassandra    *v1alpha1.Cassandra
		stateFinder          currentClusterStateFinder
		fakes                *mocks
		mockedStatefulSets   *v1beta2.StatefulSetList
	)

	BeforeEach(func() {
		desiredCassandra = aClusterDefinition()
		desiredCassandra.Namespace = clusterNamespaceName.Namespace
		desiredCassandra.Name = clusterNamespaceName.Name
		fakes = &mocks{
			client:        &mockClient{},
			objectFactory: &mockObjectFactory{},
		}
		stateFinder = currentClusterStateFinder{
			client:        fakes.client,
			objectFactory: fakes.objectFactory,
		}
	})

	AfterEach(func() {
		fakes.client.AssertExpectations(GinkgoT())
		fakes.objectFactory.AssertExpectations(GinkgoT())
	})

	Context("when no statefulset is found", func() {
		It("should be nil when the search returns a not found error", func() {
			fakes.noStatefulsetsAreFoundIn(clusterNamespaceName)

			currentCassandra, err := stateFinder.findClusterStateFor(desiredCassandra)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsNotFound(err)).To(BeTrue())
			Expect(currentCassandra).To(BeNil())
		})

		It("should be nil when the search returns an empty statefulset list", func() {
			fakes.anEmptyStatefulsetsListIsFoundIn(clusterNamespaceName)

			currentCassandra, err := stateFinder.findClusterStateFor(desiredCassandra)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsNotFound(err)).To(BeTrue())
			Expect(currentCassandra).To(BeNil())
		})
	})

	Context("regardless of persistence type", func() {
		BeforeEach(func() {
			expectedCassandra = aClusterDefinition()
		})

		AfterEach(func() {
			currentCassandra, err := stateFinder.findClusterStateFor(desiredCassandra)
			Expect(err).NotTo(HaveOccurred())
			Expect(currentCassandra).To(Equal(expectedCassandra))
		})

		It("should have the custom images when specified", func() {
			expectedCassandra.Spec.Pod.Image = ptr.String("my-cassandra-image")
			expectedCassandra.Spec.Pod.Sidecar.Image = ptr.String("my-cassandra-sidecar-image")
			expectedCassandra.Spec.Pod.BootstrapperImage = ptr.String("my-cassandra-bootstrapper-image")
			fakes.statefulsetsAreFoundIn(clusterNamespaceName, createStatefulSetsFor(expectedCassandra))
		})

		It("should have the custom liveness and readiness probes when specified", func() {
			expectedCassandra.Spec.Pod.ReadinessProbe = &v1alpha1.Probe{
				FailureThreshold:    ptr.Int32(13),
				InitialDelaySeconds: ptr.Int32(130),
				PeriodSeconds:       ptr.Int32(130),
				SuccessThreshold:    ptr.Int32(11),
				TimeoutSeconds:      ptr.Int32(15),
			}
			expectedCassandra.Spec.Pod.LivenessProbe = &v1alpha1.Probe{
				FailureThreshold:    ptr.Int32(30),
				InitialDelaySeconds: ptr.Int32(300),
				PeriodSeconds:       ptr.Int32(300),
				SuccessThreshold:    ptr.Int32(10),
				TimeoutSeconds:      ptr.Int32(50),
			}
			fakes.statefulsetsAreFoundIn(clusterNamespaceName, createStatefulSetsFor(expectedCassandra))
		})

		It("should order the statefulSet by rack name, so it is deterministic", func() {
			actualCassandra := expectedCassandra.DeepCopy()
			actualCassandra.Spec.Racks = []v1alpha1.Rack{
				rackSpec("b"),
				rackSpec("a"),
			}
			expectedCassandra.Spec.Racks = []v1alpha1.Rack{
				rackSpec("a"),
				rackSpec("b"),
			}
			fakes.statefulsetsAreFoundIn(clusterNamespaceName, createStatefulSetsFor(actualCassandra))
		})
	})

	Context("when using persistent volume", func() {
		BeforeEach(func() {
			expectedCassandra = aClusterDefinitionWithPersistenceVolumes()
			expectedCassandra.Spec.Racks[0].Storage[0].Path = ptr.String("/cassandra-storage-some-other-path")
			fakes.statefulsetsAreFoundIn(clusterNamespaceName, createStatefulSetsFor(expectedCassandra))
		})

		It("should have a persistent volume at the given path", func() {
			currentCassandra, err := stateFinder.findClusterStateFor(desiredCassandra)
			Expect(err).NotTo(HaveOccurred())
			Expect(currentCassandra).To(Equal(expectedCassandra))
			Expect(currentCassandra.Spec.Racks[0].Storage[0].EmptyDir).To(BeNil())
			Expect(currentCassandra.Spec.Racks[0].Storage[0].PersistentVolumeClaim).NotTo(BeNil())
			Expect(*currentCassandra.Spec.Racks[0].Storage[0].Path).To(Equal("/cassandra-storage-some-other-path"))
		})
	})

	Context("when using emptyDir", func() {
		BeforeEach(func() {
			expectedCassandra = aClusterDefinitionWithEmptyDir()
			expectedCassandra.Spec.Racks[0].Storage[0].Path = ptr.String("/cassandra-storage-path")
			fakes.statefulsetsAreFoundIn(clusterNamespaceName, createStatefulSetsFor(expectedCassandra))
		})

		It("should have an emptyDir at the given volume path", func() {
			currentCassandra, err := stateFinder.findClusterStateFor(desiredCassandra)
			Expect(err).NotTo(HaveOccurred())
			Expect(currentCassandra).To(Equal(expectedCassandra))
			Expect(currentCassandra.Spec.Racks[0].Storage[0].PersistentVolumeClaim).To(BeNil())
			Expect(currentCassandra.Spec.Racks[0].Storage[0].EmptyDir).NotTo(BeNil())
			Expect(*currentCassandra.Spec.Racks[0].Storage[0].Path).To(Equal("/cassandra-storage-path"))
		})
	})

	Context("when multiple storages are defined", func() {
		BeforeEach(func() {
			expectedCassandra = apis.ACassandra().WithDefaults().WithNamespace(clusterNamespaceName.Namespace).WithName(clusterNamespaceName.Name).WithSpec(
				apis.ACassandraSpec().WithDefaults().WithNoSnapshot().WithRacks(
					apis.ARack("a", 1).WithDefaults().WithStorages(
						apis.AnEmptyDir().AtPath("/emptydir-path-1"),
						apis.APersistentVolume().AtPath("/pv-cassandra").OfSize("100Gi").WithStorageClass("fast"),
						apis.APersistentVolume().AtPath("/pv-logs").OfSize("1Gi").WithStorageClass("standard"),
						apis.AnEmptyDir().AtPath("/emptydir-path-2"),
					),
				)).Build()
			statefulsSets := createStatefulSetsFor(expectedCassandra)
			fakes.statefulsetsAreFoundIn(clusterNamespaceName, statefulsSets)
		})

		It("should have a persistent volume storage defined for each persistent volume claim", func() {
			currentCassandra, err := stateFinder.findClusterStateFor(desiredCassandra)
			Expect(err).NotTo(HaveOccurred())
			Expect(currentCassandra).To(Equal(expectedCassandra))
			Expect(currentCassandra.Spec.Racks[0].Storage[1].PersistentVolumeClaim).NotTo(BeNil())
			Expect(currentCassandra.Spec.Racks[0].Storage[1].EmptyDir).To(BeNil())
			Expect(*currentCassandra.Spec.Racks[0].Storage[1].Path).To(Equal("/pv-cassandra"))
			Expect(currentCassandra.Spec.Racks[0].Storage[2].PersistentVolumeClaim).NotTo(BeNil())
			Expect(currentCassandra.Spec.Racks[0].Storage[2].EmptyDir).To(BeNil())
			Expect(*currentCassandra.Spec.Racks[0].Storage[2].Path).To(Equal("/pv-logs"))
		})

		It("should have emptyDir storage defined for each emptyDir volume in the same order", func() {
			currentCassandra, err := stateFinder.findClusterStateFor(desiredCassandra)
			Expect(err).NotTo(HaveOccurred())
			Expect(currentCassandra).To(Equal(expectedCassandra))
			Expect(currentCassandra.Spec.Racks[0].Storage[0].EmptyDir).NotTo(BeNil())
			Expect(currentCassandra.Spec.Racks[0].Storage[0].PersistentVolumeClaim).To(BeNil())
			Expect(*currentCassandra.Spec.Racks[0].Storage[0].Path).To(Equal("/emptydir-path-1"))
			Expect(currentCassandra.Spec.Racks[0].Storage[3].EmptyDir).NotTo(BeNil())
			Expect(currentCassandra.Spec.Racks[0].Storage[3].PersistentVolumeClaim).To(BeNil())
			Expect(*currentCassandra.Spec.Racks[0].Storage[3].Path).To(Equal("/emptydir-path-2"))
		})
	})
	Context("when no env vars are defined", func() {
		BeforeEach(func() {
			expectedCassandra = aClusterDefinitionWithEmptyDir()
			expectedCassandra.Spec.Racks[0].Storage[0].Path = ptr.String("/cassandra-storage-path")
			expectedCassandra.Spec.Pod.Env = &[]v1alpha1.CassEnvVar{}
			mockedStatefulSets = createStatefulSetsFor(expectedCassandra)
			fakes.statefulsetsAreFoundIn(clusterNamespaceName, mockedStatefulSets)
		})

		It("should have EXTRA_CLASSPATH in stateful set but not in cluster state", func() {
			currentCassandra, err := stateFinder.findClusterStateFor(desiredCassandra)
			Expect(err).NotTo(HaveOccurred())
			Expect(mockedStatefulSets.Items[0].Spec.Template.Spec.Containers[0].Env[0]).
				To(Equal(corev1.EnvVar{Name: "EXTRA_CLASSPATH", Value: "/extra-lib/cassandra-seed-provider.jar"}))
			Expect(currentCassandra).To(Equal(expectedCassandra))
			Expect(currentCassandra.Spec.Pod.Env).To(Equal(&[]v1alpha1.CassEnvVar{}))
		})
	})
})

func createStatefulSetsFor(clusterDef *v1alpha1.Cassandra) *v1beta2.StatefulSetList {
	c := New(clusterDef)
	var statefulSets []v1beta2.StatefulSet
	for _, rack := range c.Definition().Spec.Racks {
		ss := c.CreateStatefulSetForRack(&rack, &corev1.ConfigMap{})
		statefulSets = append(statefulSets, *ss)
	}
	return &v1beta2.StatefulSetList{
		Items: statefulSets,
	}
}
