package operator

import (
	"context"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	v1alpha1helpers "github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1/helpers"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/dispatcher"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/operator/hash"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/operator/operations"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/ptr"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test"
	"github.com/stretchr/testify/mock"
	"k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"testing"
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Controller Suite", test.CreateParallelReporters("controller"))
}

var _ = Describe("reconciliation", func() {

	var (
		configMapNamespaceName = types.NamespacedName{Namespace: "mynamespace", Name: "mycluster-config"}
		clusterNamespaceName   = types.NamespacedName{Namespace: "mynamespace", Name: "mycluster"}
		desiredCassandra       *v1alpha1.Cassandra
		fakes                  *mocks
		reconciler             CassandraReconciler
		clusters               map[types.NamespacedName]*v1alpha1.Cassandra
	)

	BeforeEach(func() {
		fakes = &mocks{
			client:           &mockClient{},
			dispatcher:       &mockDispatcher{},
			eventRecorder:    &mockEventRecorder{},
			stateFinder:      &mockStateFinder{},
			objectFactory:    &mockObjectFactory{},
			dispatchedEvents: &[]dispatcher.Event{},
		}
		desiredCassandra = aClusterDefinition()
		desiredCassandra.Namespace = clusterNamespaceName.Namespace
		desiredCassandra.Name = clusterNamespaceName.Name
		clusters = make(map[types.NamespacedName]*v1alpha1.Cassandra)

		reconciler = CassandraReconciler{
			clusters:        clusters,
			client:          fakes.client,
			eventDispatcher: fakes.dispatcher,
			eventRecorder:   fakes.eventRecorder,
			objectFactory:   fakes.objectFactory,
			stateFinder:     fakes.stateFinder,
		}
	})

	AfterEach(func() {
		fakes.assertAll(GinkgoT())
	})

	Describe("event dispatching", func() {

		Context("cluster deleted", func() {
			It("should dispatch a DeleteCluster event when no Cassandra definition found", func() {
				fakes.cassandraIsNotFoundIn(clusterNamespaceName)
				fakes.eventIsDispatched(&dispatcher.Event{
					Kind: operations.DeleteCluster,
					Key:  fmt.Sprintf("%s.%s", clusterNamespaceName.Namespace, clusterNamespaceName.Name),
					Data: &v1alpha1.Cassandra{ObjectMeta: metav1.ObjectMeta{Name: clusterNamespaceName.Name, Namespace: clusterNamespaceName.Namespace}}})

				result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("should dispatch a DeleteCluster event when the Cassandra definition is marked as being deleted", func() {
				fakes.cassandraIsBeingDeletedIn(clusterNamespaceName)
				fakes.eventIsDispatched(&dispatcher.Event{
					Kind: operations.DeleteCluster,
					Key:  fmt.Sprintf("%s.%s", clusterNamespaceName.Namespace, clusterNamespaceName.Name),
					Data: &v1alpha1.Cassandra{ObjectMeta: metav1.ObjectMeta{Name: clusterNamespaceName.Name, Namespace: clusterNamespaceName.Namespace}}})

				result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})
		})

		Context("cluster changes only", func() {

			BeforeEach(func() {
				fakes.cassandraServiceIsFoundIn(clusterNamespaceName)
				fakes.noConfigMapToReconcile(configMapNamespaceName, desiredCassandra)
			})

			It("should dispatch an AddCluster event when no corresponding cluster found", func() {
				fakes.cassandraIsFoundIn(clusterNamespaceName, desiredCassandra)
				fakes.noCurrentCassandraStateIsFoundFor(desiredCassandra)
				fakes.eventIsDispatched(&dispatcher.Event{Kind: operations.AddCluster, Key: desiredCassandra.QualifiedName(), Data: desiredCassandra})

				result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("should dispatch an UpdateCluster event when the current cluster definition is different", func() {
				currentCassandra := desiredCassandra.DeepCopy()
				currentCassandra.Spec.Pod.CPU = resource.MustParse("101m")
				fakes.cassandraIsFoundIn(clusterNamespaceName, desiredCassandra)
				fakes.currentCassandraStateIsFoundFor(desiredCassandra, currentCassandra)

				fakes.eventIsDispatched(&dispatcher.Event{
					Kind: operations.UpdateCluster, Key: desiredCassandra.QualifiedName(),
					Data: operations.ClusterUpdate{OldCluster: currentCassandra, NewCluster: desiredCassandra},
				})

				result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("should not dispatch an UpdateCluster event when current and desired Cassandra are the same", func() {
				fakes.cassandraDefinitionHasntChanged(clusterNamespaceName, desiredCassandra)

				result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})

		})

		Context("configMap changes only", func() {
			var (
				configMap *corev1.ConfigMap
			)

			BeforeEach(func() {
				configMap = &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      desiredCassandra.CustomConfigMapName(),
						Namespace: desiredCassandra.Namespace,
					},
				}
				fakes.cassandraDefinitionHasntChanged(clusterNamespaceName, desiredCassandra)
				fakes.cassandraServiceIsFoundIn(clusterNamespaceName)
			})

			It("should dispatch an AddCustomConfig when a configMap exists, but no custom config was actually associated with the Cassandra cluster", func() {
				fakes.configMapIsFoundIn(configMapNamespaceName, configMap)
				fakes.noCustomConfigIsAssociatedTo(desiredCassandra)
				fakes.eventIsDispatched(&dispatcher.Event{
					Kind: operations.AddCustomConfig,
					Key:  desiredCassandra.QualifiedName(),
					Data: operations.ConfigMapChange{Cassandra: desiredCassandra, ConfigMap: configMap},
				})

				result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("should not dispatch an event when no configMap exists and no custom config is associated with the Cassandra cluster", func() {
				fakes.configMapIsNotFoundIn(configMapNamespaceName)
				fakes.noCustomConfigIsAssociatedTo(desiredCassandra)

				result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("should not dispatch an event when no configMap exists and looking for custom config returns no cluster found error", func() {
				fakes.configMapIsNotFoundIn(configMapNamespaceName)
				fakes.customConfigStateReturnsNoClusterFoundError(desiredCassandra)

				result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("should dispatch an UpdateCustomConfig event when a configMap exists with a different hash than the one already associated with the Cassandra cluster", func() {
				configHashes := map[string]string{"rack": "some hash"}
				fakes.configMapIsFoundIn(configMapNamespaceName, configMap)
				fakes.customConfigIsAssociatedTo(desiredCassandra, configHashes)
				fakes.eventIsDispatched(&dispatcher.Event{
					Kind: operations.UpdateCustomConfig,
					Key:  desiredCassandra.QualifiedName(),
					Data: operations.ConfigMapChange{Cassandra: desiredCassandra, ConfigMap: configMap},
				})

				result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("should dispatch a DeleteCustomConfig event when when no configMap exists, but a custom config is actually associated with the Cassandra cluster", func() {
				configHashes := map[string]string{"rack": "some hash"}
				fakes.configMapIsNotFoundIn(configMapNamespaceName)
				fakes.customConfigIsAssociatedTo(desiredCassandra, configHashes)
				fakes.eventIsDispatched(&dispatcher.Event{
					Kind: operations.DeleteCustomConfig,
					Key:  desiredCassandra.QualifiedName(),
					Data: operations.ConfigMapChange{Cassandra: desiredCassandra},
				})

				result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("should not dispatch an event event when the configMap hash hasn't changed", func() {
				fakes.configMapHasntChangedFor(desiredCassandra, configMapNamespaceName)

				result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})
		})

		Context("configMap changes with multiple racks", func() {
			var (
				configMap *corev1.ConfigMap
			)

			BeforeEach(func() {
				configMap = &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      desiredCassandra.CustomConfigMapName(),
						Namespace: desiredCassandra.Namespace,
					},
				}
				desiredCassandra.Spec.Racks = []v1alpha1.Rack{
					{Name: "a", Replicas: 1, StorageClass: "rack-storage-a", Zone: "rack-zone"},
					{Name: "b", Replicas: 1, StorageClass: "rack-storage-b", Zone: "rack-zone"},
				}
				fakes.cassandraDefinitionHasntChanged(clusterNamespaceName, desiredCassandra)
				fakes.cassandraServiceIsFoundIn(clusterNamespaceName)
			})

			It("should dispatch an UpdateCustomConfig event when a configMap exists and multiple config hash values were found", func() {
				configHashes := map[string]string{"rack1": "some hash", "rack2": "some other hash"}
				fakes.configMapIsFoundIn(configMapNamespaceName, configMap)
				fakes.customConfigIsAssociatedTo(desiredCassandra, configHashes)
				fakes.eventIsDispatched(&dispatcher.Event{
					Kind: operations.UpdateCustomConfig,
					Key:  desiredCassandra.QualifiedName(),
					Data: operations.ConfigMapChange{Cassandra: desiredCassandra, ConfigMap: configMap},
				})

				result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("should dispatch an AddCustomConfig event when some statefulsets", func() {
				desiredCassandra.Spec.Racks = []v1alpha1.Rack{
					{Name: "a", Replicas: 1, StorageClass: "rack-storage-a", Zone: "rack-zone"},
					{Name: "b", Replicas: 1, StorageClass: "rack-storage-b", Zone: "rack-zone"},
				}
				configHashes := map[string]string{"a": "some hash"}

				fakes.configMapIsFoundIn(configMapNamespaceName, configMap)
				fakes.customConfigIsAssociatedTo(desiredCassandra, configHashes)
				fakes.eventIsDispatched(&dispatcher.Event{
					Kind: operations.AddCustomConfig,
					Key:  desiredCassandra.QualifiedName(),
					Data: operations.ConfigMapChange{Cassandra: desiredCassandra, ConfigMap: configMap},
				})

				result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})
		})

		Context("service changes only", func() {

			BeforeEach(func() {
				fakes.noConfigMapToReconcile(configMapNamespaceName, desiredCassandra)
				fakes.cassandraDefinitionHasntChanged(clusterNamespaceName, desiredCassandra)
			})

			It("should dispatch an AddService event when the Service does not exists", func() {
				fakes.cassandraServiceIsNotFoundIn(clusterNamespaceName)
				fakes.eventIsDispatched(&dispatcher.Event{
					Kind: operations.AddService,
					Key:  desiredCassandra.QualifiedName(),
					Data: desiredCassandra,
				})

				result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("should not dispatch an AddService event when the Service exists", func() {
				fakes.cassandraServiceIsFoundIn(clusterNamespaceName)

				result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})
		})

		Context("multiple resource changes", func() {
			var (
				configMap *corev1.ConfigMap
			)

			BeforeEach(func() {
				configMap = &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      desiredCassandra.CustomConfigMapName(),
						Namespace: desiredCassandra.Namespace,
					},
				}
			})

			It("should dispatch configMap events before Cassandra ones", func() {
				currentCassandra := desiredCassandra.DeepCopy()
				currentCassandra.Spec.Pod.CPU = resource.MustParse("101m")
				fakes.cassandraServiceIsFoundIn(clusterNamespaceName)
				fakes.cassandraIsFoundIn(clusterNamespaceName, desiredCassandra)
				fakes.currentCassandraStateIsFoundFor(desiredCassandra, currentCassandra)
				fakes.configMapIsFoundIn(configMapNamespaceName, configMap)
				fakes.noCustomConfigIsAssociatedTo(desiredCassandra)
				fakes.eventIsDispatched(&dispatcher.Event{
					Kind: operations.AddCustomConfig,
					Key:  desiredCassandra.QualifiedName(),
					Data: operations.ConfigMapChange{Cassandra: desiredCassandra, ConfigMap: configMap},
				})
				fakes.eventIsDispatched(&dispatcher.Event{
					Kind: operations.UpdateCluster, Key: desiredCassandra.QualifiedName(),
					Data: operations.ClusterUpdate{OldCluster: currentCassandra, NewCluster: desiredCassandra},
				})

				result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(fakes.recorderDispatchedEventsKind()).To(Equal([]string{operations.AddCustomConfig, operations.UpdateCluster}))
			})

			It("should dispatch add service before Cassandra events", func() {
				currentCassandra := desiredCassandra.DeepCopy()
				currentCassandra.Spec.Pod.CPU = resource.MustParse("101m")
				fakes.configMapHasntChangedFor(desiredCassandra, configMapNamespaceName)
				fakes.cassandraIsFoundIn(clusterNamespaceName, desiredCassandra)
				fakes.currentCassandraStateIsFoundFor(desiredCassandra, currentCassandra)
				fakes.cassandraServiceIsNotFoundIn(clusterNamespaceName)
				fakes.eventIsDispatched(&dispatcher.Event{
					Kind: operations.AddService,
					Key:  desiredCassandra.QualifiedName(),
					Data: desiredCassandra,
				})
				fakes.eventIsDispatched(&dispatcher.Event{
					Kind: operations.UpdateCluster, Key: desiredCassandra.QualifiedName(),
					Data: operations.ClusterUpdate{OldCluster: currentCassandra, NewCluster: desiredCassandra},
				})

				result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(fakes.recorderDispatchedEventsKind()).To(Equal([]string{operations.AddService, operations.UpdateCluster}))
			})
		})

		Context("errors", func() {

			It("should not dispatch an event but re-enqueue a reconciliation when unable to fetch the Cassandra definition", func() {
				unknownError := fakes.anErrorIsReturnWhenFindingCassandraIn(clusterNamespaceName)

				_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(fmt.Sprintf("failed to lookup cassandra definition. Will retry. Error: %s", unknownError)))
			})

			It("should not dispatch an event but re-enqueue the request when an error occurs while looking up the current config hash", func() {
				fakes.cassandraIsFoundIn(clusterNamespaceName, desiredCassandra)
				fakes.configMapIsFoundIn(configMapNamespaceName, &corev1.ConfigMap{})
				unknownError := fmt.Errorf("something when wrong when talking to api server")
				fakes.errorWhenLookingForCustomConfigStateFor(desiredCassandra, unknownError)

				_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(fmt.Sprintf("failed to look up config hash: %s", unknownError)))
			})

			It("should not dispatch an event but re-enqueue the request when an error occurs while looking up a potential configMap", func() {
				fakes.cassandraIsFoundIn(clusterNamespaceName, desiredCassandra)
				unknownError := fakes.anErrorIsReturnWhenFindingConfigMapIn(configMapNamespaceName)

				_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(fmt.Sprintf("failed to lookup potential configMap. Will retry. Error: %s", unknownError)))
			})

			It("should not dispatch an event but re-enqueue the request when an error occurs while looking up the Cassandra service", func() {
				fakes.cassandraIsFoundIn(clusterNamespaceName, desiredCassandra)
				fakes.noConfigMapToReconcile(configMapNamespaceName, desiredCassandra)
				unknownError := fakes.anErrorIsReturnWhenFindingServiceIn(clusterNamespaceName)

				_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(fmt.Sprintf("failed to lookup Cassandra service. Will retry. Error: %s", unknownError)))
			})

			It("should not dispatch an event nor retry the reconciliation when desired Cassandra fails validation", func() {
				validationError := "spec.Racks: Required value"
				desiredCassandra.Spec.Racks = nil
				fakes.cassandraServiceIsFoundIn(clusterNamespaceName)
				fakes.noConfigMapToReconcile(configMapNamespaceName, desiredCassandra)
				fakes.cassandraIsFoundIn(clusterNamespaceName, desiredCassandra)
				fakes.invalidChangeEventIsRecordedWithError(desiredCassandra, validationError)

				result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Requeue).To(BeFalse())
			})

			It("should not dispatch an event nor retry the reconciliation when the Cassandra update is not allowed", func() {
				validationError := "spec.Racks.a.Replicas: Forbidden: This field can not be decremented (scale-in is not yet supported): current: 2, new: 1"
				currentCassandra := desiredCassandra.DeepCopy()
				currentCassandra.Spec.Racks[0].Replicas = 2
				fakes.cassandraServiceIsFoundIn(clusterNamespaceName)
				fakes.noConfigMapToReconcile(configMapNamespaceName, desiredCassandra)
				fakes.currentCassandraStateIsFoundFor(desiredCassandra, currentCassandra)
				fakes.cassandraIsFoundIn(clusterNamespaceName, desiredCassandra)
				fakes.invalidChangeEventIsRecordedWithError(desiredCassandra, validationError)

				result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Requeue).To(BeFalse())
			})
		})

	})

	Describe("map of clusters", func() {

		It("should contain the fully qualified name of the cluster being reconciled even when nothing has changed", func() {
			fakes.cassandraServiceIsFoundIn(clusterNamespaceName)
			fakes.noConfigMapToReconcile(configMapNamespaceName, desiredCassandra)
			fakes.cassandraDefinitionHasntChanged(clusterNamespaceName, desiredCassandra)

			result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(clusters).To(HaveKey(clusterNamespaceName))
		})

		It("should not contain the name of the cluster being deleted", func() {
			fakes.cassandraIsNotFoundIn(clusterNamespaceName)
			fakes.eventIsDispatched(&dispatcher.Event{
				Kind: operations.DeleteCluster,
				Key:  fmt.Sprintf("%s.%s", clusterNamespaceName.Namespace, clusterNamespaceName.Name),
				Data: &v1alpha1.Cassandra{ObjectMeta: metav1.ObjectMeta{Name: clusterNamespaceName.Name, Namespace: clusterNamespaceName.Namespace}}})

			_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
			Expect(err).NotTo(HaveOccurred())
			Expect(clusters).NotTo(HaveKey(clusterNamespaceName))
		})

	})

})

func aClusterDefinition() *v1alpha1.Cassandra {
	cassandra := &v1alpha1.Cassandra{
		ObjectMeta: metav1.ObjectMeta{Name: "mycluster", Namespace: "mynamespace"},
		Spec: v1alpha1.CassandraSpec{
			Datacenter: ptr.String("my-datacenter"),
			Racks:      []v1alpha1.Rack{{Name: "a", Replicas: 1, StorageClass: "rack-storage-a", Zone: "rack-zone"}},
			Pod: v1alpha1.Pod{
				Memory:      resource.MustParse("1Gi"),
				CPU:         resource.MustParse("100m"),
				StorageSize: resource.MustParse("2Gi"),
			},
		},
	}
	v1alpha1helpers.SetDefaultsForCassandra(cassandra)
	return cassandra
}

// mocks

type mocks struct {
	client           *mockClient
	dispatcher       *mockDispatcher
	eventRecorder    *mockEventRecorder
	stateFinder      *mockStateFinder
	objectFactory    *mockObjectFactory
	dispatchedEvents *[]dispatcher.Event
}

func (m mocks) assertAll(t GinkgoTInterface) {
	m.client.AssertExpectations(t)
	m.stateFinder.AssertExpectations(t)
	m.dispatcher.AssertExpectations(t)
	m.objectFactory.AssertExpectations(t)
	m.eventRecorder.AssertExpectations(t)
}

func (m mocks) cassandraIsNotFoundIn(namespaceName types.NamespacedName) {
	cassandra := &v1alpha1.Cassandra{}
	m.objectFactory.On("newCassandra").Return(cassandra)
	m.client.On("Get", context.TODO(), namespaceName, cassandra).
		Return(errors.NewNotFound(v1alpha1.Resource("cassandra"), namespaceName.Name))
}

func (m mocks) cassandraIsBeingDeletedIn(namespaceName types.NamespacedName) {
	now := metav1.Now()
	cassandra := &v1alpha1.Cassandra{
		ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &now},
	}
	m.objectFactory.On("newCassandra").Return(cassandra)
	m.client.On("Get", context.TODO(), namespaceName, cassandra).
		Return(nil)
}

func (m mocks) cassandraIsFoundIn(namespaceName types.NamespacedName, cassandraFound *v1alpha1.Cassandra) {
	m.objectFactory.On("newCassandra").Return(cassandraFound)
	m.client.On("Get", context.TODO(), namespaceName, cassandraFound).
		Return(nil)
}

func (m mocks) anErrorIsReturnWhenFindingCassandraIn(namespaceName types.NamespacedName) error {
	unknownError := fmt.Errorf("something when wrong when talking to api server")
	cassandra := &v1alpha1.Cassandra{}
	m.objectFactory.On("newCassandra").Return(cassandra)
	m.client.On("Get", context.TODO(), namespaceName, cassandra).Return(unknownError)
	return unknownError
}

func (m mocks) configMapIsFoundIn(namespacedName types.NamespacedName, configMapFound *corev1.ConfigMap) {
	m.objectFactory.On("newConfigMap").Return(configMapFound)
	m.client.On("Get", context.TODO(), namespacedName, configMapFound).
		Return(nil)
}

func (m mocks) configMapIsNotFoundIn(namespacedName types.NamespacedName) {
	configMap := &corev1.ConfigMap{}
	m.objectFactory.On("newConfigMap").Return(configMap)
	m.client.On("Get", context.TODO(), namespacedName, configMap).
		Return(errors.NewNotFound(corev1.Resource("configmap"), namespacedName.Name))
}

func (m mocks) anErrorIsReturnWhenFindingConfigMapIn(namespaceName types.NamespacedName) error {
	unknownError := fmt.Errorf("something when wrong when talking to api server")
	configMap := &corev1.ConfigMap{}
	m.objectFactory.On("newConfigMap").Return(configMap)
	m.client.On("Get", context.TODO(), namespaceName, configMap).Return(unknownError)
	return unknownError
}

func (m mocks) anErrorIsReturnWhenFindingServiceIn(namespaceName types.NamespacedName) error {
	unknownError := fmt.Errorf("something when wrong when talking to api server")
	service := &corev1.Service{}
	m.objectFactory.On("newService").Return(service)
	m.client.On("Get", context.TODO(), namespaceName, service).Return(unknownError)
	return unknownError
}

func (m mocks) configMapHasntChangedFor(desiredCassandra *v1alpha1.Cassandra, configMapNamespaceName types.NamespacedName) {
	configMap := &corev1.ConfigMap{}
	m.configMapIsFoundIn(configMapNamespaceName, configMap)

	configHash := hash.ConfigMapHash(configMap)
	m.stateFinder.On("FindCurrentConfigHashFor", desiredCassandra).Return(map[string]string{"rack": configHash}, nil)
}

func (m mocks) noConfigMapToReconcile(configMapNamespaceName types.NamespacedName, desiredCassandra *v1alpha1.Cassandra) {
	m.configMapIsNotFoundIn(configMapNamespaceName)
	m.stateFinder.On("FindCurrentConfigHashFor", desiredCassandra).Return(nil, nil)
}

func (m mocks) cassandraDefinitionHasntChanged(clusterNamespaceName types.NamespacedName, desiredCassandra *v1alpha1.Cassandra) {
	currentCassandra := desiredCassandra.DeepCopy()
	m.cassandraIsFoundIn(clusterNamespaceName, desiredCassandra)
	m.stateFinder.On("FindCurrentStateFor", desiredCassandra).Return(currentCassandra, nil)
}

func (m mocks) noCustomConfigIsAssociatedTo(desiredCassandra *v1alpha1.Cassandra) {
	m.stateFinder.On("FindCurrentConfigHashFor", desiredCassandra).Return(nil, nil)
}

func (m mocks) customConfigStateReturnsNoClusterFoundError(desiredCassandra *v1alpha1.Cassandra) {
	m.stateFinder.On("FindCurrentConfigHashFor", desiredCassandra).
		Return(nil, errors.NewNotFound(v1beta2.Resource("statefulset"), desiredCassandra.Name))
}

func (m mocks) customConfigIsAssociatedTo(desiredCassandra *v1alpha1.Cassandra, configHashes map[string]string) {
	m.stateFinder.On("FindCurrentConfigHashFor", desiredCassandra).Return(configHashes, nil)
}

func (m mocks) currentCassandraStateIsFoundFor(desiredCassandra *v1alpha1.Cassandra, currentCassandra *v1alpha1.Cassandra) {
	m.stateFinder.On("FindCurrentStateFor", desiredCassandra).Return(currentCassandra, nil)
}

func (m mocks) noCurrentCassandraStateIsFoundFor(desiredCassandra *v1alpha1.Cassandra) {
	m.stateFinder.On("FindCurrentStateFor", desiredCassandra).
		Return(nil, errors.NewNotFound(v1beta2.Resource("statefulset"), desiredCassandra.Name))
}

func (m mocks) errorWhenLookingForCustomConfigStateFor(cassandra *v1alpha1.Cassandra, err error) {
	m.stateFinder.On("FindCurrentConfigHashFor", cassandra).Return(nil, err)
}

func (m mocks) invalidChangeEventIsRecordedWithError(desiredCassandra *v1alpha1.Cassandra, errorStr string) {
	m.eventRecorder.On("Event", desiredCassandra, corev1.EventTypeWarning, cluster.InvalidChangeEvent, errorStr).Return()
}

func (m mocks) eventIsDispatched(event *dispatcher.Event) {
	m.dispatcher.On("Dispatch", event).Return()
	*m.dispatchedEvents = append(*m.dispatchedEvents, *event)
}

func (m mocks) recorderDispatchedEventsKind() []string {
	var eventsKind []string
	for _, event := range *m.dispatchedEvents {
		eventsKind = append(eventsKind, event.Kind)
	}
	return eventsKind
}

func (m mocks) cassandraServiceIsFoundIn(namespaceName types.NamespacedName) {
	service := &corev1.Service{}
	m.objectFactory.On("newService").Return(service)
	m.client.On("Get", context.TODO(), namespaceName, service).Return(nil)
}

func (m mocks) cassandraServiceIsNotFoundIn(namespaceName types.NamespacedName) {
	service := &corev1.Service{}
	m.objectFactory.On("newService").Return(service)
	m.client.On("Get", context.TODO(), namespaceName, service).
		Return(errors.NewNotFound(corev1.Resource("service"), namespaceName.Name))
}

// mockObjectFactory

type mockObjectFactory struct {
	mock.Mock
}

func (o *mockObjectFactory) newCassandra() *v1alpha1.Cassandra {
	args := o.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*v1alpha1.Cassandra)
}

func (o *mockObjectFactory) newConfigMap() *corev1.ConfigMap {
	args := o.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*corev1.ConfigMap)
}

func (o *mockObjectFactory) newService() *corev1.Service {
	args := o.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*corev1.Service)
}

// mockStateFinder

type mockStateFinder struct {
	mock.Mock
}

func (b *mockStateFinder) FindCurrentStateFor(desiredCassandra *v1alpha1.Cassandra) (*v1alpha1.Cassandra, error) {
	args := b.Called(desiredCassandra)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*v1alpha1.Cassandra), args.Error(1)
}

func (b *mockStateFinder) FindCurrentConfigHashFor(desiredCassandra *v1alpha1.Cassandra) (map[string]string, error) {
	args := b.Called(desiredCassandra)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]string), args.Error(1)
}

// mockDispatcher

type mockDispatcher struct {
	mock.Mock
}

func (d *mockDispatcher) Dispatch(e *dispatcher.Event) {
	d.Called(e)
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
