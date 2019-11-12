package operator

import (
	"context"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/go-multierror"
	log "github.com/sirupsen/logrus"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	v1alpha1helpers "github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1/helpers"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1/validation"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/operator/event"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/hash"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"path/filepath"
	"runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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
	clusters              map[types.NamespacedName]*v1alpha1.Cassandra
	client                client.Client
	eventRecorder         record.EventRecorder
	eventReceiver         event.Receiver
	objectFactory         objectReferenceFactory
	stateFinder           cluster.StateFinder
	operatorConfig        *Config
}

// Implement reconcile.Reconciler so the controller can reconcile objects
var _ reconcile.Reconciler = &CassandraReconciler{}

// NewReconciler creates a CassandraReconciler
func NewReconciler(clusters map[types.NamespacedName]*v1alpha1.Cassandra, client client.Client, eventRecorder record.EventRecorder, eventReceiver event.Receiver, operatorConfig *Config) *CassandraReconciler {
	return &CassandraReconciler{
		clusters:              clusters,
		client:                client,
		eventRecorder:         eventRecorder,
		eventReceiver:         eventReceiver,
		objectFactory:         &defaultReferenceFactory{},
		stateFinder:           cluster.NewStateFinder(client),
		operatorConfig:        operatorConfig,
	}
}

// Reconcile implements reconcile.Reconciler
// Reconciliation for all watched resources is attempted in a single pass.
// Each request compares the current to the desired state and determines what has changed for each watched resource.
// Changes are calculated in one go for all watched resources so that the delta is calculated at a given point in time (as much as possible).
// Changes are applied one at a time without stopping on error to try bringing the cluster to the desired state as much as possible.
// Applying changes synchronously for a cluster while allowing concurrent updates for different clusters is made possible
// because the controller internal queue prevents concurrent updates for the same resource name -
// see https://github.com/kubernetes-sigs/controller-runtime/issues/616
func (r *CassandraReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := newContext(request)
	logger := ctx.logger
	logger.Info("Reconciling all Cassandra resources")

	// Lookup the object to reconcile or delete it right away and exit early
	desiredCassandra := r.objectFactory.newCassandra()
	err := r.client.Get(context.TODO(), request.NamespacedName, desiredCassandra)
	cassandraNotFound := errors.IsNotFound(err)
	if unexpectedError(err) {
		return retryReconciliation("failed to lookup cassandra definition", err)
	}

	if cassandraNotFound || cassandraIsBeingDeleted(desiredCassandra) {
		logger.Debug("Cassandra definition not found. Going to delete associated resources")
		delete(r.clusters, ctx.namespaceName)
		cassandraToDelete := &v1alpha1.Cassandra{ObjectMeta: metav1.ObjectMeta{Name: request.Name, Namespace: request.Namespace}}
		if err := r.eventReceiver.Receive(&event.Event{Kind: event.DeleteCluster, Key: ctx.eventKey, Data: cassandraToDelete}); err != nil {
			return retryReconciliation(fmt.Sprintf("failed to process event %s", event.DeleteCluster), err)
		}
		return completeReconciliation()
	}
	r.clusters[ctx.namespaceName] = desiredCassandra

	desiredConfigMap := r.objectFactory.newConfigMap()
	err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: desiredCassandra.Namespace, Name: desiredCassandra.CustomConfigMapName()}, desiredConfigMap)
	if unexpectedError(err) {
		return retryReconciliation("failed to lookup potential configMap", err)
	}

	// Reconcile the custom config first, as new statefulSet won't bootstrap
	// if they need to mount a volume for a configMap that no longer exists
	// This is an optimisation as the cluster will eventually self heal
	var combinedEvents []*event.Event
	combinedError := &multierror.Error{}
	combinedEvents, combinedError = r.appendChanges(r.determineChangesForCustomConfig, ctx, desiredCassandra, desiredConfigMap, combinedEvents, combinedError)
	combinedEvents, combinedError = r.appendChanges(r.determineChangesForCassandraService, ctx, desiredCassandra, desiredConfigMap, combinedEvents, combinedError)
	combinedEvents, combinedError = r.appendChanges(r.determineChangesForCassandraDefinition, ctx, desiredCassandra, desiredConfigMap, combinedEvents, combinedError)
	for _, evt := range combinedEvents {
		if err := r.eventReceiver.Receive(evt); err != nil {
			combinedError = multierror.Append(combinedError, fmt.Errorf("failed to process event %s: %v", evt.Kind, err))
		}
	}

	if len(combinedError.Errors) > 0 {
		logger.Error(combinedError.ErrorOrNil())
		return retryReconciliation("One or more error(s) occurred during reconciliation", combinedError.ErrorOrNil())
	}
	return completeReconciliation()
}

type changeDeterminer = func(*requestContext, *v1alpha1.Cassandra, *corev1.ConfigMap) (*event.Event, error)

func (r *CassandraReconciler) appendChanges(determineChange changeDeterminer, ctx *requestContext, desiredCassandra *v1alpha1.Cassandra, desiredConfigMap *corev1.ConfigMap, events []*event.Event, errors *multierror.Error) ([]*event.Event, *multierror.Error) {
	evt, err := determineChange(ctx, desiredCassandra, desiredConfigMap)
	if evt != nil {
		events = append(events, evt)
	}
	if err != nil {
		errors = multierror.Append(errors, err)
	}
	return events, errors
}

func (r *CassandraReconciler) determineChangesForCassandraDefinition(ctx *requestContext, desiredCassandra *v1alpha1.Cassandra, desiredConfigMap *corev1.ConfigMap) (*event.Event, error) {
	logger := ctx.logger
	logger.Debug("Reconciling Cassandra")

	v1alpha1helpers.SetDefaultsForCassandra(desiredCassandra, &v1alpha1helpers.TemplatedImageScheme{RepositoryPath: r.operatorConfig.RepositoryPath, ImageVersion: r.operatorConfig.Version})
	validationError := validation.ValidateCassandra(desiredCassandra).ToAggregate()
	if validationError != nil {
		logger.Errorf("Cassandra validation failed. Skipping reconciliation: %v", validationError)
		r.eventRecorder.Event(desiredCassandra, corev1.EventTypeWarning, cluster.InvalidChangeEvent, validationError.Error())
		return nil, nil
	}

	currentCassandra, err := r.stateFinder.FindCurrentStateFor(desiredCassandra)
	if unexpectedError(err) {
		return nil, fmt.Errorf("could not determine current state for Cassandra: %v", err)
	} else if errors.IsNotFound(err) {
		logger.Debug("No resources found for this Cassandra definition. Going to add cluster")
		return &event.Event{Kind: event.AddCluster, Key: ctx.eventKey, Data: desiredCassandra}, nil
	}

	v1alpha1helpers.SetDefaultsForCassandra(currentCassandra, &v1alpha1helpers.TemplatedImageScheme{RepositoryPath: r.operatorConfig.RepositoryPath, ImageVersion: r.operatorConfig.Version})
	validationError = validation.ValidateCassandraUpdate(currentCassandra, desiredCassandra).ToAggregate()
	if validationError != nil {
		logger.Errorf("Cassandra validation failed. Skipping reconciliation: %v", validationError)
		r.eventRecorder.Event(desiredCassandra, corev1.EventTypeWarning, cluster.InvalidChangeEvent, validationError.Error())
		return nil, nil
	}

	if !cmp.Equal(currentCassandra.Spec, desiredCassandra.Spec) {
		logger.Debug(spew.Sprintf("Cluster definition has changed. Update will be performed between current cluster: %+v \ndesired cluster: %+v", currentCassandra, desiredCassandra))
		return &event.Event{
			Kind: event.UpdateCluster,
			Key:  ctx.eventKey,
			Data: event.ClusterUpdate{OldCluster: currentCassandra, NewCluster: desiredCassandra},
		}, nil
	}

	logger.Debug("No cassandra definition changes detected")
	return nil, nil
}

func (r *CassandraReconciler) determineChangesForCustomConfig(ctx *requestContext, desiredCassandra *v1alpha1.Cassandra, desiredConfigMap *corev1.ConfigMap) (*event.Event, error) {
	logger := ctx.logger
	logger.Debug("Reconciling Cassandra custom config")

	configMapFound := desiredConfigMap.ResourceVersion != ""

	configHashes, configHashErr := r.stateFinder.FindCurrentConfigHashFor(desiredCassandra)
	logger.Debugf("Custom config hash looked up: %v", configHashes)
	if unexpectedError(configHashErr) {
		return nil, fmt.Errorf("failed to look up config hash: %v", configHashErr)
	} else if errors.IsNotFound(configHashErr) {
		logger.Debug("No custom config changes required as no corresponding Cassandra cluster found")
		return nil, nil
	}

	currentCassandraHasCustomConfig := len(configHashes) > 0
	if !configMapFound {
		if currentCassandraHasCustomConfig {
			logger.Debug("Custom config configured but no configMap exists. Going to delete configuration")
			return &event.Event{
				Kind: event.DeleteCustomConfig,
				Key:  ctx.eventKey,
				Data: event.ConfigMapChange{Cassandra: desiredCassandra},
			}, nil
		}
		logger.Debug("Custom config not configured and no configMap exists. Nothing to do")
		return nil, nil
	}

	if configMapFound && !currentCassandraHasCustomConfig {
		logger.Debug("Custom config exists, but not configured. Going to add configuration")
		return &event.Event{
			Kind: event.AddCustomConfig,
			Key:  ctx.eventKey,
			Data: event.ConfigMapChange{ConfigMap: desiredConfigMap, Cassandra: desiredCassandra},
		}, nil
	}

	if len(configHashes) != len(desiredCassandra.Spec.Racks) {
		logger.Debugf("Custom config is missing on one or more racks. Custom config hashes: %v. Going to add the configuration", configHashes)
		return &event.Event{
			Kind: event.AddCustomConfig,
			Key:  ctx.eventKey,
			Data: event.ConfigMapChange{ConfigMap: desiredConfigMap, Cassandra: desiredCassandra},
		}, nil
	}

	configHashNotChanged := true
	for _, configHash := range configHashes {
		configHashNotChanged = configHashNotChanged && configHash == hash.ConfigMapHash(desiredConfigMap)
	}
	if !configHashNotChanged {
		logger.Debug("Custom config has changed. Going to update configuration")
		return &event.Event{
			Kind: event.UpdateCustomConfig,
			Key:  ctx.eventKey,
			Data: event.ConfigMapChange{ConfigMap: desiredConfigMap, Cassandra: desiredCassandra},
		}, nil
	}

	logger.Debugf("No config map changes detected for configMap %s.%s", desiredConfigMap.Namespace, desiredConfigMap.Name)
	return nil, nil
}

func (r *CassandraReconciler) determineChangesForCassandraService(ctx *requestContext, desiredCassandra *v1alpha1.Cassandra, desiredConfigMap *corev1.ConfigMap) (*event.Event, error) {
	logger := ctx.logger
	logger.Debug("Reconciling Cassandra Service")

	service := r.objectFactory.newService()
	err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: desiredCassandra.Namespace, Name: desiredCassandra.Name}, service)
	if unexpectedError(err) {
		return nil, fmt.Errorf("failed to lookup Cassandra service: %v", err)
	} else if errors.IsNotFound(err) {
		logger.Debug("Cassandra service not found. Going to create it")
		return &event.Event{
			Kind: event.AddService,
			Key:  ctx.eventKey,
			Data: desiredCassandra,
		}, nil
	}
	return nil, nil
}

func newContext(request reconcile.Request) *requestContext {
	clusterID := fmt.Sprintf("%s.%s", request.Namespace, request.Name)
	return &requestContext{
		eventKey: clusterID,
		logger: log.WithFields(
			log.Fields{
				"logger":  callerFilename(),
				"cluster": clusterID,
			},
		),
		namespaceName: types.NamespacedName{Namespace: request.Namespace, Name: request.Name},
	}
}

func callerFilename() string {
	logger := "reconciler.go"
	_, file, _, ok := runtime.Caller(2)
	if ok {
		logger = filepath.Base(file)
	}
	return logger
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
