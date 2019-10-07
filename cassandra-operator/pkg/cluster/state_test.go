package cluster

import (
	"context"
	"fmt"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test/apis"
	"github.com/stretchr/testify/mock"
	"k8s.io/api/apps/v1beta2"
	"k8s.io/api/batch/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestState(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Cluster State Suite", test.CreateParallelReporters("cluster_state"))
}

var _ = Describe("current state", func() {

	Describe("the complete Cassandra state reconstruction", func() {
		var (
			fakes            *mockStateFinder
			stateFinder      StateFinder
			desiredCassandra *v1alpha1.Cassandra
		)

		BeforeEach(func() {
			fakes = &mockStateFinder{}
			stateFinder = &currentStateFinder{
				clusterStateFinder:         fakes,
				snapshotStateFinder:        fakes,
				snapshotCleanupStateFinder: fakes,
			}
			desiredCassandra = aClusterDefinition()
		})

		AfterEach(func() {
			fakes.AssertExpectations(GinkgoT())
		})

		Context("an error occurs when finding the state", func() {

			It("should return an error when unable to find the cluster state", func() {
				unknownError := fmt.Errorf("unknown")
				fakes.anErrorIsReturnWhenFindingClusterStateFor(desiredCassandra, unknownError)

				cassandra, err := stateFinder.FindCurrentStateFor(desiredCassandra)
				Expect(err).To(MatchError(unknownError))
				Expect(cassandra).To(BeNil())
			})

			It("should return an error when unable to find the snapshot state", func() {
				fakes.clusterStateIsFoundFor(desiredCassandra, &v1alpha1.Cassandra{})
				unknownError := fmt.Errorf("unknown")
				fakes.anErrorIsReturnWhenFindingSnapshotStateFor(desiredCassandra, unknownError)

				cassandra, err := stateFinder.FindCurrentStateFor(desiredCassandra)
				Expect(err).To(MatchError(unknownError))
				Expect(cassandra).To(BeNil())
			})

			It("should return an error when unable to find the snapshot state", func() {
				fakes.clusterStateIsFoundFor(desiredCassandra, &v1alpha1.Cassandra{})
				fakes.snapshotStateIsFoundFor(desiredCassandra, &v1alpha1.Snapshot{})
				unknownError := fmt.Errorf("unknown")
				fakes.anErrorIsReturnWhenFindingSnapshotCleanupStateFor(desiredCassandra, unknownError)

				cassandra, err := stateFinder.FindCurrentStateFor(desiredCassandra)
				Expect(err).To(MatchError(unknownError))
				Expect(cassandra).To(BeNil())
			})
		})

		Context("no existing cluster is found", func() {
			It("should be nil when no existing cluster is found", func() {
				fakes.anErrorIsReturnWhenFindingClusterStateFor(
					desiredCassandra,
					errors.NewNotFound(v1beta2.Resource("statefulset"), "the-statefulset"),
				)

				cassandra, err := stateFinder.FindCurrentStateFor(desiredCassandra)
				Expect(err).To(HaveOccurred())
				Expect(errors.IsNotFound(err)).To(BeTrue())
				Expect(cassandra).To(BeNil())
			})
		})

		Context("an existing cluster is found", func() {
			var (
				currentCluster         *v1alpha1.Cassandra
				currentSnapshot        *v1alpha1.Snapshot
				currentSnapshotCleanup *v1alpha1.RetentionPolicy
			)

			BeforeEach(func() {
				currentCluster = &v1alpha1.Cassandra{}
				currentSnapshot = &v1alpha1.Snapshot{}
				currentSnapshotCleanup = &v1alpha1.RetentionPolicy{}
			})

			It("should contain the cluster state without snapshot properties when not found", func() {
				fakes.clusterStateIsFoundFor(desiredCassandra, currentCluster)
				fakes.anErrorIsReturnWhenFindingSnapshotStateFor(
					desiredCassandra,
					errors.NewNotFound(v1beta1.Resource("cronjob"), "the-snapshot-job"),
				)

				currentCassandra, err := stateFinder.FindCurrentStateFor(desiredCassandra)
				Expect(err).NotTo(HaveOccurred())
				Expect(currentCassandra).To(Equal(currentCluster))
			})

			It("should contain the cluster state with the snapshot properties when found", func() {
				fakes.clusterStateIsFoundFor(desiredCassandra, currentCluster)
				fakes.snapshotStateIsFoundFor(desiredCassandra, currentSnapshot)
				fakes.anErrorIsReturnWhenFindingSnapshotCleanupStateFor(
					desiredCassandra,
					errors.NewNotFound(v1beta1.Resource("cronjob"), "the-snapshot-cleanup-job"),
				)
				expectedCassandra := currentCluster
				expectedCassandra.Spec.Snapshot = currentSnapshot

				currentCassandra, err := stateFinder.FindCurrentStateFor(desiredCassandra)
				Expect(err).NotTo(HaveOccurred())
				Expect(currentCassandra).To(Equal(expectedCassandra))
			})

			It("should contain the cluster state with the snapshot cleanup properties when found", func() {
				fakes.clusterStateIsFoundFor(desiredCassandra, currentCluster)
				fakes.snapshotStateIsFoundFor(desiredCassandra, currentSnapshot)
				fakes.snapshotStateCleanupIsFoundFor(desiredCassandra, currentSnapshotCleanup)
				expectedCassandra := currentCluster
				expectedCassandra.Spec.Snapshot = currentSnapshot
				expectedCassandra.Spec.Snapshot.RetentionPolicy = currentSnapshotCleanup

				currentCassandra, err := stateFinder.FindCurrentStateFor(desiredCassandra)
				Expect(err).NotTo(HaveOccurred())
				Expect(currentCassandra).To(Equal(expectedCassandra))
			})

		})

	})

	Describe("the Config Hash state", func() {
		var (
			fakes            *mockStateFinder
			stateFinder      StateFinder
			desiredCassandra *v1alpha1.Cassandra
		)

		BeforeEach(func() {
			fakes = &mockStateFinder{}
			stateFinder = &currentStateFinder{
				clusterStateFinder:         fakes,
				snapshotStateFinder:        fakes,
				snapshotCleanupStateFinder: fakes,
			}
			desiredCassandra = aClusterDefinition()
		})

		AfterEach(func() {
			fakes.AssertExpectations(GinkgoT())
		})

		It("should return nil when an error occurred while retrieving the statefulset", func() {
			unknownError := fmt.Errorf("unknown")
			fakes.anErrorIsReturnWhenFindingStatefulSetsFor(desiredCassandra, unknownError)

			configHash, err := stateFinder.FindCurrentConfigHashFor(desiredCassandra)
			Expect(err).To(MatchError(unknownError))
			Expect(configHash).To(BeEmpty())
		})

		It("should return nil when no annotation was found", func() {
			statefulSetList := &v1beta2.StatefulSetList{
				Items: []v1beta2.StatefulSet{
					aStatefulSetWithAnnotation("a", nil),
				},
			}
			fakes.statefulSetsFoundFor(desiredCassandra, statefulSetList)

			configHash, err := stateFinder.FindCurrentConfigHashFor(desiredCassandra)
			Expect(err).NotTo(HaveOccurred())
			Expect(configHash).To(BeEmpty())
		})

		It("should return nil when no configMap annotation was found", func() {
			statefulSetList := &v1beta2.StatefulSetList{
				Items: []v1beta2.StatefulSet{
					aStatefulSetWithAnnotation("a", map[string]string{"someannotation": "somevalue"}),
				},
			}
			fakes.statefulSetsFoundFor(desiredCassandra, statefulSetList)

			configHash, err := stateFinder.FindCurrentConfigHashFor(desiredCassandra)
			Expect(err).NotTo(HaveOccurred())
			Expect(configHash).To(BeEmpty())
		})

		It("should return the hash for each statefulset", func() {
			statefulSetList := &v1beta2.StatefulSetList{
				Items: []v1beta2.StatefulSet{
					aStatefulSetWithAnnotation("a", map[string]string{ConfigHashAnnotation: "theHash"}),
					aStatefulSetWithAnnotation("b", map[string]string{ConfigHashAnnotation: "theOtherHash"}),
				},
			}
			fakes.statefulSetsFoundFor(desiredCassandra, statefulSetList)

			configHash, err := stateFinder.FindCurrentConfigHashFor(desiredCassandra)
			Expect(err).NotTo(HaveOccurred())
			Expect(configHash).To(Equal(map[string]string{"a": "theHash", "b": "theOtherHash"}))
		})

	})
})

func aStatefulSetWithAnnotation(name string, annotations map[string]string) v1beta2.StatefulSet {
	return v1beta2.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1beta2.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
				},
			},
		},
	}
}

func aClusterDefinitionWithPersistenceVolumes() *v1alpha1.Cassandra {
	return aClusterDefinition()
}

func aClusterDefinition() *v1alpha1.Cassandra {
	return apis.ACassandra().
		WithDefaults().
		WithName("mycluster").
		WithNamespace("mynamespace").
		WithSpec(apis.ACassandraSpec().
			WithDefaults().
			WithNoSnapshot().
			WithRacks(apis.ARack("a", 1).WithDefaults())).
		Build()
}

func aClusterDefinitionWithEmptyDir() *v1alpha1.Cassandra {
	return apis.ACassandra().
		WithDefaults().
		WithName("mycluster").
		WithNamespace("mynamespace").
		WithSpec(apis.ACassandraSpec().
			WithDefaults().
			WithNoSnapshot().
			WithRacks(apis.ARack("a", 1).
				WithDefaults().
				WithStorages(apis.AnEmptyDir()))).
		Build()
}

func rackSpec(name string) v1alpha1.Rack {
	return apis.ARack(name, 1).WithDefaults().Build()
}

// mocks

type mocks struct {
	client        *mockClient
	stateFinder   *mockStateFinder
	objectFactory *mockObjectFactory
}

func (m mocks) assertAll(t GinkgoTInterface) {
	m.client.AssertExpectations(t)
	m.stateFinder.AssertExpectations(t)
	m.objectFactory.AssertExpectations(t)
}

func (m mocks) anEmptyStatefulsetsListIsFoundIn(namespaceName types.NamespacedName) {
	m.objectFactory.On("newStatefulSetList").Return(&v1beta2.StatefulSetList{})
	m.client.On("List", context.TODO(), &v1beta2.StatefulSetList{}, mock.AnythingOfType("[]client.ListOptionFunc")).
		Return(nil)
}

func (m mocks) noStatefulsetsAreFoundIn(namespaceName types.NamespacedName) {
	m.objectFactory.On("newStatefulSetList").Return(&v1beta2.StatefulSetList{})
	m.client.On("List", context.TODO(), &v1beta2.StatefulSetList{}, mock.AnythingOfType("[]client.ListOptionFunc")).
		Return(errors.NewNotFound(v1beta2.Resource("statefulset"), namespaceName.Name))
}

func (m mocks) statefulsetsAreFoundIn(namespaceName types.NamespacedName, statefulsetsFound *v1beta2.StatefulSetList) {
	m.objectFactory.On("newStatefulSetList").Return(statefulsetsFound)
	m.client.On("List", context.TODO(), statefulsetsFound, mock.AnythingOfType("[]client.ListOptionFunc")).
		Return(nil)
}

func (m mocks) snapshotJobIsFoundFor(cassandra *v1alpha1.Cassandra, job *v1beta1.CronJob) {
	namespaceName := types.NamespacedName{Namespace: cassandra.Namespace, Name: cassandra.SnapshotJobName()}
	m.objectFactory.On("newCronJob").Return(job)
	m.client.On("Get", context.TODO(), namespaceName, job).
		Return(nil)
}

func (m mocks) snapshotJobIsNotFoundFor(cassandra *v1alpha1.Cassandra) {
	namespaceName := types.NamespacedName{Namespace: cassandra.Namespace, Name: cassandra.SnapshotJobName()}
	job := &v1beta1.CronJob{}
	m.objectFactory.On("newCronJob").Return(job)
	m.client.On("Get", context.TODO(), namespaceName, job).
		Return(errors.NewNotFound(v1beta1.Resource("cronjob"), cassandra.Namespace))
}

func (m mocks) snapshotCleanupJobIsFoundFor(cassandra *v1alpha1.Cassandra, job *v1beta1.CronJob) {
	namespaceName := types.NamespacedName{Namespace: cassandra.Namespace, Name: cassandra.SnapshotCleanupJobName()}
	m.objectFactory.On("newCronJob").Return(job)
	m.client.On("Get", context.TODO(), namespaceName, job).
		Return(nil)
}

func (m mocks) snapshotJobCleanupIsNotFoundFor(cassandra *v1alpha1.Cassandra) {
	job := &v1beta1.CronJob{}
	m.objectFactory.On("newCronJob").Return(job)
	namespaceName := types.NamespacedName{Namespace: cassandra.Namespace, Name: cassandra.SnapshotCleanupJobName()}
	m.client.On("Get", context.TODO(), namespaceName, job).
		Return(errors.NewNotFound(v1beta1.Resource("cronjob"), cassandra.Namespace))
}

// mockObjectFactory

type mockObjectFactory struct {
	mock.Mock
}

func (o *mockObjectFactory) newStatefulSetList() *v1beta2.StatefulSetList {
	args := o.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*v1beta2.StatefulSetList)
}

func (o *mockObjectFactory) newCronJob() *v1beta1.CronJob {
	args := o.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*v1beta1.CronJob)
}

// mockStateFinder

type mockStateFinder struct {
	mock.Mock
}

func (s *mockStateFinder) findClusterStateFor(desiredCassandra *v1alpha1.Cassandra) (*v1alpha1.Cassandra, error) {
	args := s.Called(desiredCassandra)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*v1alpha1.Cassandra), args.Error(1)
}

func (s *mockStateFinder) findStatefulSetsFor(desiredCassandra *v1alpha1.Cassandra) (*v1beta2.StatefulSetList, error) {
	args := s.Called(desiredCassandra)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*v1beta2.StatefulSetList), args.Error(1)
}

func (s *mockStateFinder) findSnapshotStateFor(desiredCassandra *v1alpha1.Cassandra) (*v1alpha1.Snapshot, error) {
	args := s.Called(desiredCassandra)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*v1alpha1.Snapshot), args.Error(1)
}

func (s *mockStateFinder) findSnapshotCleanupStateFor(desiredCassandra *v1alpha1.Cassandra) (*v1alpha1.RetentionPolicy, error) {
	args := s.Called(desiredCassandra)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*v1alpha1.RetentionPolicy), args.Error(1)
}

func (s *mockStateFinder) anErrorIsReturnWhenFindingClusterStateFor(cassandra *v1alpha1.Cassandra, err error) {
	s.On("findClusterStateFor", cassandra).Return(nil, err)
}

func (s *mockStateFinder) anErrorIsReturnWhenFindingSnapshotStateFor(cassandra *v1alpha1.Cassandra, err error) {
	s.On("findSnapshotStateFor", cassandra).Return(nil, err)
}

func (s *mockStateFinder) anErrorIsReturnWhenFindingSnapshotCleanupStateFor(cassandra *v1alpha1.Cassandra, err error) {
	s.On("findSnapshotCleanupStateFor", cassandra).Return(nil, err)
}

func (s *mockStateFinder) clusterStateIsFoundFor(desiredCassandra *v1alpha1.Cassandra, foundClusterState *v1alpha1.Cassandra) {
	s.On("findClusterStateFor", desiredCassandra).Return(foundClusterState, nil)
}

func (s *mockStateFinder) snapshotStateIsFoundFor(desiredCassandra *v1alpha1.Cassandra, foundClusterState *v1alpha1.Snapshot) {
	s.On("findSnapshotStateFor", desiredCassandra).Return(foundClusterState, nil)
}

func (s *mockStateFinder) snapshotStateCleanupIsFoundFor(desiredCassandra *v1alpha1.Cassandra, foundClusterState *v1alpha1.RetentionPolicy) {
	s.On("findSnapshotCleanupStateFor", desiredCassandra).Return(foundClusterState, nil)
}

func (s *mockStateFinder) anErrorIsReturnWhenFindingStatefulSetsFor(desiredCassandra *v1alpha1.Cassandra, err error) {
	s.On("findStatefulSetsFor", desiredCassandra).Return(nil, err)
}

func (s *mockStateFinder) statefulSetsFoundFor(desiredCassandra *v1alpha1.Cassandra, foundStatefulSets *v1beta2.StatefulSetList) {
	s.On("findStatefulSetsFor", desiredCassandra).Return(foundStatefulSets, nil)
}

// mockClient

type mockClient struct {
	mock.Mock
}

func (c *mockClient) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	args := c.Called(ctx, key, obj)
	return args.Error(0)
}
func (c *mockClient) Update(ctx context.Context, obj runtime.Object, opts ...client.UpdateOptionFunc) error {
	var args mock.Arguments
	if len(opts) > 0 {
		args = c.Called(ctx, obj, opts)
	} else {
		args = c.Called(ctx, obj)
	}
	return args.Error(0)
}
func (c *mockClient) List(ctx context.Context, list runtime.Object, opts ...client.ListOptionFunc) error {
	var args mock.Arguments
	if len(opts) > 0 {
		args = c.Called(ctx, list, opts)
	} else {
		args = c.Called(ctx, list)
	}
	return args.Error(0)
}
func (c *mockClient) Create(ctx context.Context, obj runtime.Object, opts ...client.CreateOptionFunc) error {
	return fmt.Errorf("not implemented")
}
func (c *mockClient) Delete(ctx context.Context, obj runtime.Object, opts ...client.DeleteOptionFunc) error {
	return fmt.Errorf("not implemented")
}
func (c *mockClient) Patch(ctx context.Context, obj runtime.Object, patch client.Patch, opts ...client.PatchOptionFunc) error {
	return fmt.Errorf("not implemented")
}
func (c *mockClient) Status() client.StatusWriter {
	return nil
}

// mockEventRecorder

type mockEventRecorder struct {
	mock.Mock
}

func (e *mockEventRecorder) Event(object runtime.Object, eventtype, reason, message string) {
	e.Called(object, eventtype, reason, message)
}
func (e *mockEventRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	e.Called(object, eventtype, reason, messageFmt, args)
}
func (e *mockEventRecorder) PastEventf(object runtime.Object, timestamp metav1.Time, eventtype, reason, messageFmt string, args ...interface{}) {
	e.Called(object, timestamp, eventtype, reason, messageFmt, args)
}
func (e *mockEventRecorder) AnnotatedEventf(object runtime.Object, annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {
	e.Called(object, annotations, eventtype, reason, messageFmt, args)
}
