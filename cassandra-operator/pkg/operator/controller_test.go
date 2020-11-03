package operator

import (
	"context"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/metrics"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/operator/event"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/hash"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test/apis"
	"github.com/stretchr/testify/mock"
	"k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sync"
	"testing"
	"time"
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Controller Suite", test.CreateParallelReporters("reconcileController"))
}

const initialResourceVersion = "1"

var _ = Describe("reconciler", func() {

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
			receiver:         &mockReceiver{},
			eventRecorder:    &mockEventRecorder{},
			stateFinder:      &mockStateFinder{},
			objectFactory:    &mockObjectFactory{},
			dispatchedEvents: &[]event.Event{},
			metricsReporter:  &mockMetricsReporter{},
		}
		desiredCassandra = aClusterDefinition()
		desiredCassandra.Namespace = clusterNamespaceName.Namespace
		desiredCassandra.Name = clusterNamespaceName.Name
		clusters = make(map[types.NamespacedName]*v1alpha1.Cassandra)

		reconciler = CassandraReconciler{
			clusters:        clusters,
			client:          fakes.client,
			eventReceiver:   fakes.receiver,
			eventRecorder:   fakes.eventRecorder,
			objectFactory:   fakes.objectFactory,
			stateFinder:     fakes.stateFinder,
			operatorConfig:  &Config{Version: "latest", RepositoryPath: "skyuk"},
			metricsReporter: fakes.metricsReporter,
		}
	})

	AfterEach(func() {
		fakes.assertAll(GinkgoT())
	})

	Describe("event dispatching", func() {

		Context("cluster deleted", func() {
			It("should dispatch a DeleteCluster event when no Cassandra definition found", func() {
				fakes.cassandraIsNotFoundIn(clusterNamespaceName)
				fakes.eventIsProcessedSuccessfully(&event.Event{
					Kind: event.DeleteCluster,
					Key:  fmt.Sprintf("%s.%s", clusterNamespaceName.Namespace, clusterNamespaceName.Name),
					Data: &v1alpha1.Cassandra{ObjectMeta: metav1.ObjectMeta{Name: clusterNamespaceName.Name, Namespace: clusterNamespaceName.Namespace}}})

				result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("should dispatch a DeleteCluster event when the Cassandra definition is marked as being deleted", func() {
				fakes.cassandraIsBeingDeletedIn(clusterNamespaceName)
				fakes.eventIsProcessedSuccessfully(&event.Event{
					Kind: event.DeleteCluster,
					Key:  fmt.Sprintf("%s.%s", clusterNamespaceName.Namespace, clusterNamespaceName.Name),
					Data: &v1alpha1.Cassandra{ObjectMeta: metav1.ObjectMeta{Name: clusterNamespaceName.Name, Namespace: clusterNamespaceName.Namespace}}})

				result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("should re-enqueue the request when the event processing fails", func() {
				fakes.cassandraIsBeingDeletedIn(clusterNamespaceName)
				eventProcessingError := fakes.eventFailsToProcess(&event.Event{
					Kind: event.DeleteCluster,
					Key:  fmt.Sprintf("%s.%s", clusterNamespaceName.Namespace, clusterNamespaceName.Name),
					Data: &v1alpha1.Cassandra{ObjectMeta: metav1.ObjectMeta{Name: clusterNamespaceName.Name, Namespace: clusterNamespaceName.Namespace}}})

				_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("failed to process event %s. Will retry. Error: %s", event.DeleteCluster, eventProcessingError)))
			})

		})

		Context("cluster changes only", func() {

			BeforeEach(func() {
				fakes.cassandraServiceIsFoundIn(clusterNamespaceName)
				fakes.noConfigMapToReconcile(configMapNamespaceName, desiredCassandra)
				fakes.reconcilerInitialisesMetric(desiredCassandra)
			})

			It("should dispatch an AddCluster event when no corresponding cluster found", func() {
				fakes.cassandraIsFoundIn(clusterNamespaceName, desiredCassandra)
				fakes.noCurrentCassandraStateIsFoundFor(desiredCassandra)
				fakes.eventIsProcessedSuccessfully(&event.Event{Kind: event.AddCluster, Key: desiredCassandra.QualifiedName(), Data: desiredCassandra})

				result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("should re-enqueue the request when the add cluster event processing failed", func() {
				fakes.cassandraIsFoundIn(clusterNamespaceName, desiredCassandra)
				fakes.noCurrentCassandraStateIsFoundFor(desiredCassandra)
				eventProcessingError := fakes.eventFailsToProcess(&event.Event{Kind: event.AddCluster, Key: desiredCassandra.QualifiedName(), Data: desiredCassandra})

				_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("failed to process event %s: %s", event.AddCluster, eventProcessingError)))
			})

			It("should dispatch an UpdateCluster event when the current cluster definition is different", func() {
				currentCassandra := desiredCassandra.DeepCopy()
				currentCassandra.Spec.Pod.Resources.Requests[corev1.ResourceCPU] = resource.MustParse("101m")
				fakes.cassandraIsFoundIn(clusterNamespaceName, desiredCassandra)
				fakes.currentCassandraStateIsFoundFor(desiredCassandra, currentCassandra)

				fakes.eventIsProcessedSuccessfully(&event.Event{
					Kind: event.UpdateCluster, Key: desiredCassandra.QualifiedName(),
					Data: event.ClusterUpdate{OldCluster: currentCassandra, NewCluster: desiredCassandra},
				})

				result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("should re-enqueue the request when the update cluster event processing failed", func() {
				currentCassandra := desiredCassandra.DeepCopy()
				currentCassandra.Spec.Pod.Resources.Requests[corev1.ResourceCPU] = resource.MustParse("101m")
				fakes.cassandraIsFoundIn(clusterNamespaceName, desiredCassandra)
				fakes.currentCassandraStateIsFoundFor(desiredCassandra, currentCassandra)

				eventProcessingError := fakes.eventFailsToProcess(&event.Event{
					Kind: event.UpdateCluster, Key: desiredCassandra.QualifiedName(),
					Data: event.ClusterUpdate{OldCluster: currentCassandra, NewCluster: desiredCassandra},
				})

				_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("failed to process event %s: %s", event.UpdateCluster, eventProcessingError)))
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
						Name:            desiredCassandra.CustomConfigMapName(),
						Namespace:       desiredCassandra.Namespace,
						ResourceVersion: initialResourceVersion,
					},
				}
				fakes.cassandraDefinitionHasntChanged(clusterNamespaceName, desiredCassandra)
				fakes.cassandraServiceIsFoundIn(clusterNamespaceName)
				fakes.reconcilerInitialisesMetric(desiredCassandra)
			})

			It("should dispatch an AddCustomConfig when a configMap exists, but no custom config was actually associated with the Cassandra cluster", func() {
				fakes.configMapIsFoundIn(configMapNamespaceName, configMap)
				fakes.noCustomConfigIsAssociatedTo(desiredCassandra)
				fakes.eventIsProcessedSuccessfully(&event.Event{
					Kind: event.AddCustomConfig,
					Key:  desiredCassandra.QualifiedName(),
					Data: event.ConfigMapChange{Cassandra: desiredCassandra, ConfigMap: configMap},
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
				fakes.eventIsProcessedSuccessfully(&event.Event{
					Kind: event.UpdateCustomConfig,
					Key:  desiredCassandra.QualifiedName(),
					Data: event.ConfigMapChange{Cassandra: desiredCassandra, ConfigMap: configMap},
				})

				result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("should dispatch a DeleteCustomConfig event when when no configMap exists, but a custom config is actually associated with the Cassandra cluster", func() {
				configHashes := map[string]string{"rack": "some hash"}
				fakes.configMapIsNotFoundIn(configMapNamespaceName)
				fakes.customConfigIsAssociatedTo(desiredCassandra, configHashes)
				fakes.eventIsProcessedSuccessfully(&event.Event{
					Kind: event.DeleteCustomConfig,
					Key:  desiredCassandra.QualifiedName(),
					Data: event.ConfigMapChange{Cassandra: desiredCassandra},
				})

				result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("should re-enqueue the request when a DeleteCustomConfig event processing failed", func() {
				configHashes := map[string]string{"rack": "some hash"}
				fakes.configMapIsNotFoundIn(configMapNamespaceName)
				fakes.customConfigIsAssociatedTo(desiredCassandra, configHashes)
				eventProcessingError := fakes.eventFailsToProcess(&event.Event{
					Kind: event.DeleteCustomConfig,
					Key:  desiredCassandra.QualifiedName(),
					Data: event.ConfigMapChange{Cassandra: desiredCassandra},
				})

				_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("failed to process event %s: %s", event.DeleteCustomConfig, eventProcessingError)))
			})

			It("should re-enqueue the request when the UpdateCustomConfig event processing failed", func() {
				configHashes := map[string]string{"rack": "some hash"}
				fakes.configMapIsFoundIn(configMapNamespaceName, configMap)
				fakes.customConfigIsAssociatedTo(desiredCassandra, configHashes)
				eventProcessingError := fakes.eventFailsToProcess(&event.Event{
					Kind: event.UpdateCustomConfig,
					Key:  desiredCassandra.QualifiedName(),
					Data: event.ConfigMapChange{Cassandra: desiredCassandra, ConfigMap: configMap},
				})

				_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("failed to process event %s: %s", event.UpdateCustomConfig, eventProcessingError)))
			})

			It("should re-enqueue the request when the AddCustomConfig event processing failed", func() {
				configHashes := make(map[string]string)

				fakes.configMapIsFoundIn(configMapNamespaceName, configMap)
				fakes.customConfigIsAssociatedTo(desiredCassandra, configHashes)
				eventProcessingError := fakes.eventFailsToProcess(&event.Event{
					Kind: event.AddCustomConfig,
					Key:  desiredCassandra.QualifiedName(),
					Data: event.ConfigMapChange{Cassandra: desiredCassandra, ConfigMap: configMap},
				})

				_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("failed to process event %s: %s", event.AddCustomConfig, eventProcessingError)))
			})

			It("should not dispatch an event event when the configMap hash hasn't changed", func() {
				fakes.cassandraDefinitionHasntChanged(clusterNamespaceName, desiredCassandra)
				fakes.cassandraServiceIsFoundIn(clusterNamespaceName)
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
						Name:            desiredCassandra.CustomConfigMapName(),
						Namespace:       desiredCassandra.Namespace,
						ResourceVersion: initialResourceVersion,
					},
				}
				desiredCassandra.Spec.Racks = []v1alpha1.Rack{rackSpec("a"), rackSpec("b")}
				fakes.cassandraDefinitionHasntChanged(clusterNamespaceName, desiredCassandra)
				fakes.cassandraServiceIsFoundIn(clusterNamespaceName)
				fakes.reconcilerInitialisesMetric(desiredCassandra)
			})

			It("should dispatch an UpdateCustomConfig event when a configMap exists and multiple config hash values were found", func() {
				configHashes := map[string]string{"rack1": "some hash", "rack2": "some other hash"}
				fakes.configMapIsFoundIn(configMapNamespaceName, configMap)
				fakes.customConfigIsAssociatedTo(desiredCassandra, configHashes)
				fakes.eventIsProcessedSuccessfully(&event.Event{
					Kind: event.UpdateCustomConfig,
					Key:  desiredCassandra.QualifiedName(),
					Data: event.ConfigMapChange{Cassandra: desiredCassandra, ConfigMap: configMap},
				})

				result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("should dispatch an AddCustomConfig event when some statefulsets do not have a configMap", func() {
				desiredCassandra.Spec.Racks = []v1alpha1.Rack{rackSpec("a"), rackSpec("b")}
				configHashes := map[string]string{"a": "some hash"}

				fakes.configMapIsFoundIn(configMapNamespaceName, configMap)
				fakes.customConfigIsAssociatedTo(desiredCassandra, configHashes)
				fakes.eventIsProcessedSuccessfully(&event.Event{
					Kind: event.AddCustomConfig,
					Key:  desiredCassandra.QualifiedName(),
					Data: event.ConfigMapChange{Cassandra: desiredCassandra, ConfigMap: configMap},
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
				fakes.reconcilerInitialisesMetric(desiredCassandra)
			})

			It("should dispatch an AddService event when the Service does not exists", func() {
				fakes.cassandraServiceIsNotFoundIn(clusterNamespaceName)
				fakes.eventIsProcessedSuccessfully(&event.Event{
					Kind: event.AddService,
					Key:  desiredCassandra.QualifiedName(),
					Data: desiredCassandra,
				})

				result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("should re-enqueue the request when the AddService event processing failed", func() {
				fakes.cassandraServiceIsNotFoundIn(clusterNamespaceName)
				eventProcessingError := fakes.eventFailsToProcess(&event.Event{
					Kind: event.AddService,
					Key:  desiredCassandra.QualifiedName(),
					Data: desiredCassandra,
				})

				_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("failed to process event %s: %s", event.AddService, eventProcessingError)))
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
						Name:            desiredCassandra.CustomConfigMapName(),
						Namespace:       desiredCassandra.Namespace,
						ResourceVersion: initialResourceVersion,
					},
				}
				fakes.reconcilerInitialisesMetric(desiredCassandra)
			})

			It("should dispatch configMap Events before Cassandra ones", func() {
				currentCassandra := desiredCassandra.DeepCopy()
				currentCassandra.Spec.Pod.Resources.Requests[corev1.ResourceCPU] = resource.MustParse("101m")
				fakes.cassandraServiceIsFoundIn(clusterNamespaceName)
				fakes.cassandraIsFoundIn(clusterNamespaceName, desiredCassandra)
				fakes.currentCassandraStateIsFoundFor(desiredCassandra, currentCassandra)
				fakes.configMapIsFoundIn(configMapNamespaceName, configMap)
				fakes.noCustomConfigIsAssociatedTo(desiredCassandra)
				fakes.eventIsProcessedSuccessfully(&event.Event{
					Kind: event.AddCustomConfig,
					Key:  desiredCassandra.QualifiedName(),
					Data: event.ConfigMapChange{Cassandra: desiredCassandra, ConfigMap: configMap},
				})
				fakes.eventIsProcessedSuccessfully(&event.Event{
					Kind: event.UpdateCluster, Key: desiredCassandra.QualifiedName(),
					Data: event.ClusterUpdate{OldCluster: currentCassandra, NewCluster: desiredCassandra},
				})

				result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(fakes.recorderDispatchedEventsKind()).To(Equal([]string{event.AddCustomConfig, event.UpdateCluster}))
			})

			It("should dispatch add service before Cassandra Events", func() {
				currentCassandra := desiredCassandra.DeepCopy()
				currentCassandra.Spec.Pod.Resources.Requests[corev1.ResourceCPU] = resource.MustParse("101m")
				fakes.configMapHasntChangedFor(desiredCassandra, configMapNamespaceName)
				fakes.cassandraIsFoundIn(clusterNamespaceName, desiredCassandra)
				fakes.currentCassandraStateIsFoundFor(desiredCassandra, currentCassandra)
				fakes.cassandraServiceIsNotFoundIn(clusterNamespaceName)
				fakes.eventIsProcessedSuccessfully(&event.Event{
					Kind: event.AddService,
					Key:  desiredCassandra.QualifiedName(),
					Data: desiredCassandra,
				})
				fakes.eventIsProcessedSuccessfully(&event.Event{
					Kind: event.UpdateCluster, Key: desiredCassandra.QualifiedName(),
					Data: event.ClusterUpdate{OldCluster: currentCassandra, NewCluster: desiredCassandra},
				})

				result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(fakes.recorderDispatchedEventsKind()).To(Equal([]string{event.AddService, event.UpdateCluster}))
			})
		})

		Context("errors", func() {

			BeforeEach(func() {
				fakes.reconcilerInitialisesMetric(desiredCassandra)
			})

			It("should not dispatch an event but re-enqueue a reconciliation when unable to fetch the Cassandra definition", func() {
				unknownError := fakes.anErrorIsReturnWhenFindingCassandraIn(clusterNamespaceName)

				_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("Error: %s", unknownError)))
			})

			It("should not dispatch an event but re-enqueue the request when an error occurs while looking up the current config hash", func() {
				fakes.cassandraDefinitionHasntChanged(clusterNamespaceName, desiredCassandra)
				fakes.cassandraServiceIsFoundIn(clusterNamespaceName)
				fakes.configMapIsFoundIn(configMapNamespaceName, &corev1.ConfigMap{})
				unknownError := fmt.Errorf("something when wrong when talking to api server")
				fakes.errorWhenLookingForCustomConfigStateFor(desiredCassandra, unknownError)

				_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("failed to look up config hash: %s", unknownError)))
			})

			It("should not dispatch an event but re-enqueue the request when an error occurs while looking up a potential configMap", func() {
				fakes.cassandraIsFoundIn(clusterNamespaceName, desiredCassandra)
				unknownError := fakes.anErrorIsReturnWhenFindingConfigMapIn(configMapNamespaceName)

				_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("failed to lookup potential configMap. Will retry. Error: %s", unknownError)))
			})

			It("should not dispatch an event but re-enqueue the request when an error occurs while looking up the Cassandra service", func() {
				fakes.cassandraDefinitionHasntChanged(clusterNamespaceName, desiredCassandra)
				fakes.noConfigMapToReconcile(configMapNamespaceName, desiredCassandra)
				unknownError := fakes.anErrorIsReturnWhenFindingServiceIn(clusterNamespaceName)

				_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("failed to lookup Cassandra service: %s", unknownError)))
			})

			It("should re-enqueue a single request when multiple error occur", func() {
				fakes.cassandraDefinitionHasntChanged(clusterNamespaceName, desiredCassandra)
				fakes.configMapIsNotFoundIn(configMapNamespaceName)
				serviceLookupError := fakes.anErrorIsReturnWhenFindingServiceIn(clusterNamespaceName)
				stateFinderLookupError := fakes.anErrorIsReturnedWhenFindingState()

				_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("One or more error(s) occurred during reconciliation. Will retry. Error: 2 errors occurred"))
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("failed to lookup Cassandra service: %s", serviceLookupError)))
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("failed to look up config hash: %s", stateFinderLookupError)))
			})

			It("should not dispatch an event nor retry the reconciliation when desired Cassandra fails validation", func() {
				validationError := "spec.Racks: Required value"
				desiredCassandra.Spec.Racks = nil
				fakes.cassandraIsFoundIn(clusterNamespaceName, desiredCassandra)
				fakes.cassandraServiceIsFoundIn(clusterNamespaceName)
				fakes.noConfigMapToReconcile(configMapNamespaceName, desiredCassandra)
				fakes.cassandraIsFoundIn(clusterNamespaceName, desiredCassandra)
				fakes.invalidChangeEventIsRecordedWithError(desiredCassandra, validationError)
				fakes.invalidChangeEventIncrementsMetric(desiredCassandra)

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
				fakes.invalidChangeEventIncrementsMetric(desiredCassandra)

				result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Requeue).To(BeFalse())
			})
		})

	})

	Describe("map of clusters", func() {

		BeforeEach(func() {
			fakes.reconcilerInitialisesMetric(desiredCassandra)
		})

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
			clusters[clusterNamespaceName] = &v1alpha1.Cassandra{}
			fakes.cassandraIsNotFoundIn(clusterNamespaceName)
			fakes.eventIsProcessedSuccessfully(&event.Event{
				Kind: event.DeleteCluster,
				Key:  fmt.Sprintf("%s.%s", clusterNamespaceName.Namespace, clusterNamespaceName.Name),
				Data: &v1alpha1.Cassandra{ObjectMeta: metav1.ObjectMeta{Name: clusterNamespaceName.Name, Namespace: clusterNamespaceName.Namespace}}})

			_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
			Expect(err).NotTo(HaveOccurred())
			Expect(clusters).NotTo(HaveKey(clusterNamespaceName))
		})

		It("should not contain the name of the cluster being deleted even when the delete event failed to complete", func() {
			clusters[clusterNamespaceName] = &v1alpha1.Cassandra{}
			fakes.cassandraIsNotFoundIn(clusterNamespaceName)
			_ = fakes.eventFailsToProcess(&event.Event{
				Kind: event.DeleteCluster,
				Key:  fmt.Sprintf("%s.%s", clusterNamespaceName.Namespace, clusterNamespaceName.Name),
				Data: &v1alpha1.Cassandra{ObjectMeta: metav1.ObjectMeta{Name: clusterNamespaceName.Name, Namespace: clusterNamespaceName.Namespace}}})

			_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: clusterNamespaceName})
			Expect(err).To(HaveOccurred())
			Expect(clusters).NotTo(HaveKey(clusterNamespaceName))
		})

	})

})

var _ = Describe("interrupter", func() {
	var (
		client                mockClient
		activeReconciliations sync.Map
		cassandra             *v1alpha1.Cassandra
		clusterName           types.NamespacedName
		configMapName         types.NamespacedName
		eventRecorder         mockEventRecorder
	)

	BeforeEach(func() {
		cassandra = aClusterDefinition()
		clusterName = types.NamespacedName{Namespace: cassandra.Namespace, Name: cassandra.Name}
		configMapName = types.NamespacedName{Namespace: cassandra.Namespace, Name: fmt.Sprintf("%s-config", cassandra.Name)}

		client.On("Get", context.TODO(), clusterName, &v1alpha1.Cassandra{}).Return(nil)
		client.On("Get", context.TODO(), configMapName, &corev1.ConfigMap{}).Return(nil)

		eventRecorder.On(
			"Eventf",
			mock.Anything,
			corev1.EventTypeNormal,
			cluster.ReconciliationInterruptedEvent,
			"Reconciliation interrupted for cluster %s",
			[]interface{}{cassandra.QualifiedName()},
		).Return()
	})

	AfterEach(func() {
		t := GinkgoT()
		client.AssertExpectations(t)
		eventRecorder.AssertExpectations(t)
	})

	It("should interrupt an in progress operation when Cassandra revision has changed", func() {
		// given
		reconciliation := cluster.NewReconciliation("2", "")
		activeReconciliations.Store(cassandra.QualifiedName(), reconciliation)

		// when
		interrupter := NewInterrupter(&activeReconciliations, &client, &eventRecorder)
		go func() { _, _ = interrupter.Reconcile(reconcile.Request{NamespacedName: clusterName}) }()

		// then
		Eventually(reconciliation.InterruptChannel()).Should(Receive())
	})

	It("should interrupt an in progress operation when ConfigMap revision has changed", func() {
		// given
		reconciliation := cluster.NewReconciliation("", "2")
		activeReconciliations.Store(cassandra.QualifiedName(), reconciliation)

		// when
		interrupter := NewInterrupter(&activeReconciliations, &client, &eventRecorder)
		go func() { _, _ = interrupter.Reconcile(reconcile.Request{NamespacedName: clusterName}) }()

		// then
		Eventually(reconciliation.InterruptChannel()).Should(Receive())
	})

	It("should not interrupt an in progress operation when neither Cassandra nor ConfigMap revision has changed", func() {
		// given
		reconciliation := cluster.NewReconciliation("", "")
		activeReconciliations.Store(cassandra.QualifiedName(), reconciliation)

		// when
		interrupter := NewInterrupter(&activeReconciliations, &client, &eventRecorder)
		go func() { _, _ = interrupter.Reconcile(reconcile.Request{NamespacedName: clusterName}) }()

		// then
		Consistently(reconciliation.InterruptChannel(), 3*time.Second, 1*time.Second).ShouldNot(Receive())
	})
})

func aClusterDefinition() *v1alpha1.Cassandra {
	return apis.ACassandra().
		WithDefaults().
		WithSpec(apis.ACassandraSpec().
			WithDefaults().
			WithRacks(apis.ARack("a", 1).WithDefaults())).
		Build()
}

func rackSpec(name string) v1alpha1.Rack {
	return apis.ARack(name, 1).WithDefaults().WithStorages(apis.AnEmptyDir()).Build()
}

// mocks

type mocks struct {
	client           *mockClient
	receiver         *mockReceiver
	eventRecorder    *mockEventRecorder
	stateFinder      *mockStateFinder
	objectFactory    *mockObjectFactory
	dispatchedEvents *[]event.Event
	metricsReporter  *mockMetricsReporter
}

func (m mocks) assertAll(t GinkgoTInterface) {
	m.client.AssertExpectations(t)
	m.stateFinder.AssertExpectations(t)
	m.receiver.AssertExpectations(t)
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

func (m mocks) anErrorIsReturnedWhenFindingState() error {
	unknownError := fmt.Errorf("something when wrong when talking to api server")
	m.stateFinder.On("FindCurrentConfigHashFor", mock.Anything).Return(nil, unknownError)
	return unknownError
}

func (m mocks) configMapHasntChangedFor(desiredCassandra *v1alpha1.Cassandra, configMapNamespaceName types.NamespacedName) {
	configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{ResourceVersion: initialResourceVersion}}
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

func (m mocks) invalidChangeEventIncrementsMetric(desiredCassandra *v1alpha1.Cassandra) {
	m.metricsReporter.On("IncrementFailedValidationMetric", desiredCassandra).Return()
}

func (m mocks) reconcilerInitialisesMetric(desiredCassandra *v1alpha1.Cassandra) {
	m.metricsReporter.On("InitialiseFailedValidationMetric", desiredCassandra).Return()
}

func (m mocks) eventIsProcessedSuccessfully(event *event.Event) {
	m.receiver.On("Receive", event).Return(nil)
	*m.dispatchedEvents = append(*m.dispatchedEvents, *event)
}

func (m mocks) eventFailsToProcess(event *event.Event) error {
	eventProcessingError := fmt.Errorf("some error")
	m.receiver.On("Receive", event).Return(eventProcessingError)
	return eventProcessingError
}

func (m mocks) recorderDispatchedEventsKind() []string {
	var eventsKind []string
	for _, evt := range *m.dispatchedEvents {
		eventsKind = append(eventsKind, evt.Kind)
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

// implements objectReferenceFactory at compilation time
var _ objectReferenceFactory = &mockObjectFactory{}

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

// implements cluster.StateFinder at compilation time
var _ cluster.StateFinder = &mockStateFinder{}

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

// mockReceiver

type mockReceiver struct {
	mock.Mock
}

// force implementation of  operations.Receiver at compilation time
var _ event.Receiver = &mockReceiver{}

func (d *mockReceiver) Receive(e *event.Event) error {
	args := d.Called(e)
	return args.Error(0)
}

// mockClient

type mockClient struct {
	mock.Mock
}

// force implementation of client.Client at compilation time
var _ client.Client = &mockClient{}

func (c *mockClient) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	args := c.Called(ctx, key, obj)
	return args.Error(0)
}
func (c *mockClient) Update(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error {
	var args mock.Arguments
	if len(opts) > 0 {
		args = c.Called(ctx, obj, opts)
	} else {
		args = c.Called(ctx, obj)
	}
	return args.Error(0)
}
func (c *mockClient) List(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
	var args mock.Arguments
	if len(opts) > 0 {
		args = c.Called(ctx, list, opts)
	} else {
		args = c.Called(ctx, list)
	}
	return args.Error(0)
}
func (c *mockClient) Create(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
	return fmt.Errorf("not implemented")
}
func (c *mockClient) Delete(ctx context.Context, obj runtime.Object, opts ...client.DeleteOption) error {
	return fmt.Errorf("not implemented")
}
func (c *mockClient) DeleteAllOf(ctx context.Context, obj runtime.Object, opts ...client.DeleteAllOfOption) error {
	return fmt.Errorf("not implemented")
}
func (c *mockClient) Patch(ctx context.Context, obj runtime.Object, patch client.Patch, opts ...client.PatchOption) error {
	return fmt.Errorf("not implemented")
}
func (c *mockClient) Status() client.StatusWriter {
	return nil
}

// mockEventRecorder

type mockEventRecorder struct {
	mock.Mock
}

// force implementation of record.EventRecorder at compilation time
var _ record.EventRecorder = &mockEventRecorder{}

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

// mockMetricsReporter

type mockMetricsReporter struct {
	mock.Mock
}

// force implementation of metrics.ClusterMetricsReporter at compilation time
var _ metrics.ClusterMetricsReporter = &mockMetricsReporter{}

func (m *mockMetricsReporter) IncrementFailedValidationMetric(desiredCassandra *v1alpha1.Cassandra) {
	m.Called(desiredCassandra)
}

func (m *mockMetricsReporter) InitialiseFailedValidationMetric(desiredCassandra *v1alpha1.Cassandra) {
	m.Called(desiredCassandra)
}

func (m *mockMetricsReporter) DeleteMetrics(desiredCassandra *v1alpha1.Cassandra) {
	m.Called(desiredCassandra)
}

func (m *mockMetricsReporter) UpdateMetrics(desiredCassandra *v1alpha1.Cassandra) {
	m.Called(desiredCassandra)
}

func (m *mockMetricsReporter) RemoveNodeFromMetrics(desiredCassandra *v1alpha1.Cassandra, podName, rackName string) {
	m.Called(desiredCassandra, podName, rackName)
}