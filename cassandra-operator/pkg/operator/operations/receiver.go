package operations

import (
	log "github.com/sirupsen/logrus"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	v1alpha1helpers "github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1/helpers"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/dispatcher"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/metrics"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/operator/operations/adjuster"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
)

const (
	// AddCluster is a kind of event which the receiver is able to handle
	AddCluster = "ADD_CLUSTER"
	// DeleteCluster is a kind of event which the receiver is able to handle
	DeleteCluster = "DELETE_CLUSTER"
	// UpdateCluster is a kind of event which the receiver is able to handle
	UpdateCluster = "UPDATE_CLUSTER"
	// GatherMetrics is a kind of event which the receiver is able to handle
	GatherMetrics = "GATHER_METRICS"
	// AddCustomConfig is a kind of event which the receiver is able to handle
	AddCustomConfig = "ADD_CUSTOM_CONFIG"
	// UpdateCustomConfig is a kind of event which the receiver is able to handle
	UpdateCustomConfig = "UPDATE_CUSTOM_CONFIG"
	// DeleteCustomConfig is a kind of event which the receiver is able to handle
	DeleteCustomConfig = "DELETE_CUSTOM_CONFIG"
	// AddService is a kind of event which the receiver is able to handle
	AddService = "ADD_SERVICE"
)

// ClusterUpdate encapsulates Cassandra specs before and after the change
type ClusterUpdate struct {
	OldCluster *v1alpha1.Cassandra
	NewCluster *v1alpha1.Cassandra
}

// ConfigMapChange encapsulates ConfigMap changes for the desired Cassandra cluster
type ConfigMapChange struct {
	ConfigMap *v1.ConfigMap
	Cassandra *v1alpha1.Cassandra
}

// Receiver receives events dispatched by the operator
type Receiver struct {
	clusterAccessor     cluster.Accessor
	statefulSetAccessor *statefulSetAccessor
	metricsPoller       *metrics.PrometheusMetrics
	eventRecorder       record.EventRecorder
	adjuster            *adjuster.Adjuster
}

// NewEventReceiver creates a new Receiver
func NewEventReceiver(clusterAccessor cluster.Accessor, metricsPoller *metrics.PrometheusMetrics, eventRecorder record.EventRecorder) *Receiver {
	adj, err := adjuster.New()
	if err != nil {
		log.Fatalf("unable to initialise Adjuster: %v", err)
	}

	statefulsetAccessor := &statefulSetAccessor{clusterAccessor: clusterAccessor}
	return &Receiver{
		clusterAccessor:     clusterAccessor,
		statefulSetAccessor: statefulsetAccessor,
		eventRecorder:       eventRecorder,
		adjuster:            adj,
		metricsPoller:       metricsPoller,
	}
}

// Receive receives operator events and delegates their processing to the appropriate handler
func (r *Receiver) Receive(event *dispatcher.Event) {
	logger := r.eventLogger(event)
	logger.Debug("Event received")
	operations := r.operationsToExecute(event)
	logger.Infof("Event will trigger %d operations: %v", len(operations), operations)

	for _, operation := range operations {
		logger.Debugf("Executing operation %s", operation.String())
		operation.Execute()
	}
	logger.Debug("Event complete")
}

func (r *Receiver) operationsToExecute(event *dispatcher.Event) []Operation {
	logger := r.eventLogger(event)

	switch event.Kind {
	case AddService:
		return []Operation{r.newAddService(event.Data.(*v1alpha1.Cassandra))}
	case AddCluster:
		return r.operationsForAddCluster(event.Data.(*v1alpha1.Cassandra))
	case DeleteCluster:
		return r.operationsForDeleteCluster(event.Data.(*v1alpha1.Cassandra))
	case UpdateCluster:
		update := event.Data.(ClusterUpdate)
		if r.clusterDoesNotExistOrIsBeingDeleted(update.NewCluster) {
			logOperationNotAllowed(logger)
			return nil
		}
		return r.operationsForUpdateCluster(update)
	case GatherMetrics:
		cassandra := event.Data.(*v1alpha1.Cassandra)
		if r.clusterDoesNotExistOrIsBeingDeleted(cassandra) {
			logOperationNotAllowed(logger)
			return nil
		}
		return []Operation{r.newGatherMetrics(cassandra)}
	case UpdateCustomConfig:
		configMapChange := event.Data.(ConfigMapChange)
		cassandra := configMapChange.Cassandra
		if r.clusterDoesNotExistOrIsBeingDeleted(cassandra) {
			logOperationNotAllowed(logger)
			return nil
		}
		return []Operation{r.newUpdateCustomConfig(cassandra, configMapChange.ConfigMap)}
	case AddCustomConfig:
		configMapChange := event.Data.(ConfigMapChange)
		cassandra := configMapChange.Cassandra
		if r.clusterDoesNotExistOrIsBeingDeleted(cassandra) {
			logOperationNotAllowed(logger)
			return nil
		}
		return []Operation{r.newAddCustomConfig(cassandra, configMapChange.ConfigMap)}
	case DeleteCustomConfig:
		configMapChange := event.Data.(ConfigMapChange)
		cassandra := configMapChange.Cassandra
		if r.clusterDoesNotExistOrIsBeingDeleted(cassandra) {
			logOperationNotAllowed(logger)
			return nil
		}
		return []Operation{r.newDeleteCustomConfig(cassandra)}
	default:
		logger.Error("Event type is not supported")
	}

	return nil
}

func (r *Receiver) operationsForAddCluster(cassandra *v1alpha1.Cassandra) []Operation {
	operations := []Operation{r.newAddCluster(cassandra)}
	if cassandra.Spec.Snapshot != nil {
		operations = append(operations, r.newAddSnapshot(cassandra))
		if v1alpha1helpers.HasRetentionPolicyEnabled(cassandra.Spec.Snapshot) {
			operations = append(operations, r.newAddSnapshotCleanup(cassandra))
		}
	}
	return operations
}

func (r *Receiver) operationsForDeleteCluster(cassandra *v1alpha1.Cassandra) []Operation {
	operations := []Operation{r.newDeleteCluster(cassandra)}
	if cassandra.Spec.Snapshot != nil {
		operations = append(operations, r.newDeleteSnapshot(cassandra))
		if v1alpha1helpers.HasRetentionPolicyEnabled(cassandra.Spec.Snapshot) {
			operations = append(operations, r.newDeleteSnapshotCleanup(cassandra))
		}
	}
	return operations
}

func (r *Receiver) operationsForUpdateCluster(clusterUpdate ClusterUpdate) []Operation {
	var operations []Operation
	oldCluster := clusterUpdate.OldCluster
	newCluster := clusterUpdate.NewCluster

	operations = append(operations, r.newUpdateCluster(clusterUpdate))
	if newCluster.Spec.Snapshot == nil && oldCluster.Spec.Snapshot != nil {
		operations = append(operations, r.newDeleteSnapshot(clusterUpdate.NewCluster))
		if v1alpha1helpers.HasRetentionPolicyEnabled(oldCluster.Spec.Snapshot) {
			operations = append(operations, r.newDeleteSnapshotCleanup(clusterUpdate.NewCluster))
		}
	} else if newCluster.Spec.Snapshot != nil && oldCluster.Spec.Snapshot != nil {
		if !v1alpha1helpers.HasRetentionPolicyEnabled(newCluster.Spec.Snapshot) && v1alpha1helpers.HasRetentionPolicyEnabled(oldCluster.Spec.Snapshot) {
			operations = append(operations, r.newDeleteSnapshotCleanup(clusterUpdate.NewCluster))
		}
		if v1alpha1helpers.SnapshotPropertiesUpdated(oldCluster.Spec.Snapshot, newCluster.Spec.Snapshot) {
			operations = append(operations, r.newUpdateSnapshot(newCluster))
		}
		if v1alpha1helpers.SnapshotCleanupPropertiesUpdated(oldCluster.Spec.Snapshot, newCluster.Spec.Snapshot) {
			operations = append(operations, r.newUpdateSnapshotCleanup(newCluster))
		}
	} else if newCluster.Spec.Snapshot != nil && oldCluster.Spec.Snapshot == nil {
		operations = append(operations, r.newAddSnapshot(clusterUpdate.NewCluster))
		if v1alpha1helpers.HasRetentionPolicyEnabled(newCluster.Spec.Snapshot) {
			operations = append(operations, r.newAddSnapshotCleanup(clusterUpdate.NewCluster))
		}
	}
	return operations
}

func (r *Receiver) eventLogger(event *dispatcher.Event) *log.Entry {
	logger := log.WithFields(
		log.Fields{
			"type":   event.Kind,
			"key":    event.Key,
			"id":     event.ID,
			"logger": "receiver.go",
		},
	)
	return logger
}

func logOperationNotAllowed(logger *log.Entry) {
	logger.Warn("Operation is not allowed. Cluster does not exist or is being deleted")
}

func (r *Receiver) clusterDoesNotExistOrIsBeingDeleted(clusterToFind *v1alpha1.Cassandra) bool {
	cassandra, err := r.clusterAccessor.GetCassandraForCluster(clusterToFind.Namespace, clusterToFind.Name)
	return errors.IsNotFound(err) || cassandra.DeletionTimestamp != nil
}
