package operator

import (
	"context"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"github.com/google/go-cmp/cmp"
	log "github.com/sirupsen/logrus"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1/validation"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/operator/hash"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	v1alpha1helpers "github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1/helpers"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/dispatcher"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/operator/operations"
)

type objectReferenceFactory interface {
	newCassandra() *v1alpha1.Cassandra
	newConfigMap() *corev1.ConfigMap
	newService() *corev1.Service
}

// implements objectReferenceFactory
type defaultReferenceFactory struct {
}

type requestContext struct {
	eventKey      string
	logger        *log.Entry
	namespaceName types.NamespacedName
}

// CassandraReconciler is a controller that reconciles Cassandra resources
type CassandraReconciler struct {
	clusters        map[types.NamespacedName]*v1alpha1.Cassandra
	client          client.Client
	eventRecorder   record.EventRecorder
	eventDispatcher dispatcher.Dispatcher
	objectFactory   objectReferenceFactory
	stateFinder     cluster.StateFinder
}

// Implement reconcile.Reconciler so the controller can reconcile objects
var _ reconcile.Reconciler = &CassandraReconciler{}

// NewReconciler creates a CassandraReconciler
func NewReconciler(clusters map[types.NamespacedName]*v1alpha1.Cassandra, client client.Client, eventRecorder record.EventRecorder, eventDispatcher dispatcher.Dispatcher) *CassandraReconciler {
	return &CassandraReconciler{
		clusters:        clusters,
		client:          client,
		eventRecorder:   eventRecorder,
		eventDispatcher: eventDispatcher,
		objectFactory:   &defaultReferenceFactory{},
		stateFinder:     cluster.NewStateFinder(client),
	}
}

// Reconcile implements reconcile.Reconciler
func (r *CassandraReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := newContext(request)
	logger := ctx.logger
	logger.Info("Reconciling all Cassandra resources")

	// Lookup the object to reconcile or delete it right away and exit early
	desiredCassandra := r.objectFactory.newCassandra()
	err := r.client.Get(context.TODO(), request.NamespacedName, desiredCassandra)
	if unexpectedError(err) {
		return retryReconciliation("failed to lookup cassandra definition", err)
	} else if errors.IsNotFound(err) || cassandraIsBeingDeleted(desiredCassandra) {
		logger.Debug("Cassandra definition not found. Going to delete associated resources")
		cassandraToDelete := &v1alpha1.Cassandra{ObjectMeta: metav1.ObjectMeta{Name: request.Name, Namespace: request.Namespace}}
		r.eventDispatcher.Dispatch(&dispatcher.Event{Kind: operations.DeleteCluster, Key: ctx.eventKey, Data: cassandraToDelete})
		delete(r.clusters, ctx.namespaceName)
		return completeReconciliation()
	}
	r.clusters[ctx.namespaceName] = desiredCassandra

	// Reconcile the custom config first, as new statefulSet won't bootstrap
	// if they need to mount a volume for a configMap that no longer exists
	result, err := r.reconcileCustomConfig(ctx, desiredCassandra)
	if err != nil {
		logger.Errorf("An error occurred while reconciling custom config for Cassandra: %v", err)
		return result, err
	}

	// Create the service before Cassandra pods to allow pods to access that service (e.g. to find seeds)
	result, err = r.reconcileCassandraService(ctx, desiredCassandra)
	if err != nil {
		logger.Errorf("An error occurred while reconciling Cassandra service: %v", err)
		return result, err
	}

	result, err = r.reconcileCassandraDefinition(ctx, desiredCassandra)
	if err != nil {
		logger.Errorf("An error occurred while reconciling Cassandra definition: %v", err)
		return result, err
	}
	return completeReconciliation()
}

func (r *CassandraReconciler) reconcileCassandraDefinition(ctx *requestContext, desiredCassandra *v1alpha1.Cassandra) (reconcile.Result, error) {
	logger := ctx.logger
	logger.Debug("Reconciling Cassandra")

	v1alpha1helpers.SetDefaultsForCassandra(desiredCassandra)
	validationError := validation.ValidateCassandra(desiredCassandra).ToAggregate()
	if validationError != nil {
		logger.Errorf("Cassandra validation failed. Skipping reconciliation: %v", validationError)
		r.eventRecorder.Event(desiredCassandra, corev1.EventTypeWarning, cluster.InvalidChangeEvent, validationError.Error())
		return completeReconciliation()
	}

	currentCassandra, err := r.stateFinder.FindCurrentStateFor(desiredCassandra)
	if unexpectedError(err) {
		return retryReconciliation("could not determine current state for Cassandra", err)
	} else if errors.IsNotFound(err) {
		logger.Debug("No resources found for this Cassandra definition. Going to add cluster")
		r.eventDispatcher.Dispatch(&dispatcher.Event{Kind: operations.AddCluster, Key: ctx.eventKey, Data: desiredCassandra})
		return completeReconciliation()
	}

	v1alpha1helpers.SetDefaultsForCassandra(currentCassandra)
	validationError = validation.ValidateCassandraUpdate(currentCassandra, desiredCassandra).ToAggregate()
	if validationError != nil {
		logger.Errorf("Cassandra validation failed. Skipping reconciliation: %v", validationError)
		r.eventRecorder.Event(desiredCassandra, corev1.EventTypeWarning, cluster.InvalidChangeEvent, validationError.Error())
		return completeReconciliation()
	}

	if !cmp.Equal(currentCassandra.Spec, desiredCassandra.Spec) {
		logger.Debug(spew.Sprintf("Cluster definition has changed. Update will be performed between current cluster: %+v \ndesired cluster: %+v", currentCassandra, desiredCassandra))
		r.eventDispatcher.Dispatch(&dispatcher.Event{
			Kind: operations.UpdateCluster,
			Key:  ctx.eventKey,
			Data: operations.ClusterUpdate{OldCluster: currentCassandra, NewCluster: desiredCassandra},
		})
		return completeReconciliation()
	}

	logger.Debug("No cassandra definition changes detected")
	return completeReconciliation()
}

func (r *CassandraReconciler) reconcileCustomConfig(ctx *requestContext, desiredCassandra *v1alpha1.Cassandra) (reconcile.Result, error) {
	logger := ctx.logger
	logger.Debug("Reconciling Cassandra custom config")

	configMap := r.objectFactory.newConfigMap()
	configMapErr := r.client.Get(context.TODO(), types.NamespacedName{Namespace: desiredCassandra.Namespace, Name: desiredCassandra.CustomConfigMapName()}, configMap)
	configMapFound := configMapErr == nil
	if unexpectedError(configMapErr) {
		return retryReconciliation("failed to lookup potential configMap", configMapErr)
	}

	configHashes, configHashErr := r.stateFinder.FindCurrentConfigHashFor(desiredCassandra)
	logger.Debugf("Custom config hash looked up: %v", configHashes)
	if unexpectedError(configHashErr) {
		return reconcile.Result{}, fmt.Errorf("failed to look up config hash: %v", configHashErr)
	} else if errors.IsNotFound(configHashErr) {
		logger.Debug("No custom config changes required as no corresponding Cassandra cluster found")
		return completeReconciliation()
	}

	currentCassandraHasCustomConfig := len(configHashes) > 0
	if errors.IsNotFound(configMapErr) {
		if currentCassandraHasCustomConfig {
			logger.Debug("Custom config configured but no configMap exists. Going to delete configuration")
			r.eventDispatcher.Dispatch(&dispatcher.Event{
				Kind: operations.DeleteCustomConfig,
				Key:  ctx.eventKey,
				Data: operations.ConfigMapChange{Cassandra: desiredCassandra},
			})
		} else {
			logger.Debug("Custom config not configured and no configMap exists. Nothing to do")
		}
		return completeReconciliation()
	}

	if configMapFound && !currentCassandraHasCustomConfig {
		logger.Debug("Custom config exists, but not configured. Going to add configuration")
		r.eventDispatcher.Dispatch(&dispatcher.Event{
			Kind: operations.AddCustomConfig,
			Key:  ctx.eventKey,
			Data: operations.ConfigMapChange{ConfigMap: configMap, Cassandra: desiredCassandra},
		})
		return completeReconciliation()
	}

	if len(configHashes) != len(desiredCassandra.Spec.Racks) {
		logger.Debugf("Custom config is missing on one or more racks. Custom config hashes: %v. Going to add the configuration", configHashes)
		r.eventDispatcher.Dispatch(&dispatcher.Event{
			Kind: operations.AddCustomConfig,
			Key:  ctx.eventKey,
			Data: operations.ConfigMapChange{ConfigMap: configMap, Cassandra: desiredCassandra},
		})
		return completeReconciliation()
	}

	configHashNotChanged := true
	for _, configHash := range configHashes {
		configHashNotChanged = configHashNotChanged && configHash == hash.ConfigMapHash(configMap)
	}
	if !configHashNotChanged {
		logger.Debug("Custom config has changed. Going to update configuration")
		r.eventDispatcher.Dispatch(&dispatcher.Event{
			Kind: operations.UpdateCustomConfig,
			Key:  ctx.eventKey,
			Data: operations.ConfigMapChange{ConfigMap: configMap, Cassandra: desiredCassandra},
		})
		return completeReconciliation()
	}

	logger.Debugf("No config map changes detected for configMap %s.%s", configMap.Namespace, configMap.Name)
	return completeReconciliation()
}

func (r *CassandraReconciler) reconcileCassandraService(ctx *requestContext, desiredCassandra *v1alpha1.Cassandra) (reconcile.Result, error) {
	logger := ctx.logger
	logger.Debug("Reconciling Cassandra Service")

	service := r.objectFactory.newService()
	err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: desiredCassandra.Namespace, Name: desiredCassandra.Name}, service)
	if unexpectedError(err) {
		return retryReconciliation("failed to lookup Cassandra service", err)
	} else if errors.IsNotFound(err) {
		logger.Debug("Cassandra service not found. Going to create it")
		r.eventDispatcher.Dispatch(&dispatcher.Event{
			Kind: operations.AddService,
			Key:  ctx.eventKey,
			Data: desiredCassandra,
		})
		return completeReconciliation()
	}
	return completeReconciliation()
}

func newContext(request reconcile.Request) *requestContext {
	clusterID := fmt.Sprintf("%s.%s", request.Namespace, request.Name)
	return &requestContext{
		eventKey: clusterID,
		logger: log.WithFields(
			log.Fields{
				"logger":  "reconciler.go",
				"cluster": clusterID,
			},
		),
		namespaceName: types.NamespacedName{Namespace: request.Namespace, Name: request.Name},
	}
}

func retryReconciliation(reason string, err error) (reconcile.Result, error) {
	return reconcile.Result{}, fmt.Errorf("%s. Will retry. Error: %v", reason, err)
}

func completeReconciliation() (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

func unexpectedError(err error) bool {
	return err != nil && !errors.IsNotFound(err)
}

func (r *defaultReferenceFactory) newCassandra() *v1alpha1.Cassandra {
	return &v1alpha1.Cassandra{}
}

func (r *defaultReferenceFactory) newConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{}
}

func (r *defaultReferenceFactory) newService() *corev1.Service {
	return &corev1.Service{}
}

func cassandraIsBeingDeleted(cassandra *v1alpha1.Cassandra) bool {
	return cassandra.DeletionTimestamp != nil
}
