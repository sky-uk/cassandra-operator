package event

import (
	log "github.com/sirupsen/logrus"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	v1alpha1helpers "github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1/helpers"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/metrics"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/operator/operations"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/operator/operations/adjuster"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
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
type Receiver interface {
	Receive(event *Event) error
}

// OperatorEventReceiver implements Receiver
type OperatorEventReceiver struct {
	clusterAccessor   cluster.Accessor
	operationFactory  operations.OperationFactory
	operationComposer operations.OperationComposer
}

// NewEventReceiver creates a new OperatorEventReceiver
func NewEventReceiver(clusterAccessor cluster.Accessor, metricsPoller *metrics.PrometheusMetrics, eventRecorder record.EventRecorder) Receiver {
	return &OperatorEventReceiver{
		clusterAccessor:   clusterAccessor,
		operationComposer: operations.NewOperationComposer(),
		operationFactory:  operations.NewOperationFactory(clusterAccessor, metricsPoller, eventRecorder, adjuster.New()),
	}
}

// Receive receives operator events and delegates their processing to the appropriate handler
func (r *OperatorEventReceiver) Receive(event *Event) error {
	logger := r.eventLogger(event)
	logger.Debug("Event received")
	operationsToRun := r.operationsToExecute(event)
	logger.Infof("Event will trigger %d operations: %v", len(operationsToRun), operationsToRun)

	stopped, err := r.operationComposer.Execute(operationsToRun)
	if stopped {
		logger.Debug("Event complete - not all operations were executed")
	} else {
		logger.Debug("Event complete")
	}

	return err
}

func (r *OperatorEventReceiver) operationsToExecute(event *Event) []operations.Operation {
	logger := r.eventLogger(event)

	switch event.Kind {
	case AddService:
		return []operations.Operation{r.operationFactory.NewAddService(event.Data.(*v1alpha1.Cassandra))}
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
		return []operations.Operation{r.operationFactory.NewGatherMetrics(cassandra)}
	case UpdateCustomConfig:
		configMapChange := event.Data.(ConfigMapChange)
		cassandra := configMapChange.Cassandra
		if r.clusterDoesNotExistOrIsBeingDeleted(cassandra) {
			logOperationNotAllowed(logger)
			return nil
		}
		return []operations.Operation{r.operationFactory.NewUpdateCustomConfig(cassandra, configMapChange.ConfigMap)}
	case AddCustomConfig:
		configMapChange := event.Data.(ConfigMapChange)
		cassandra := configMapChange.Cassandra
		if r.clusterDoesNotExistOrIsBeingDeleted(cassandra) {
			logOperationNotAllowed(logger)
			return nil
		}
		return []operations.Operation{r.operationFactory.NewAddCustomConfig(cassandra, configMapChange.ConfigMap)}
	case DeleteCustomConfig:
		configMapChange := event.Data.(ConfigMapChange)
		cassandra := configMapChange.Cassandra
		if r.clusterDoesNotExistOrIsBeingDeleted(cassandra) {
			logOperationNotAllowed(logger)
			return nil
		}
		return []operations.Operation{r.operationFactory.NewDeleteCustomConfig(cassandra)}
	default:
		logger.Error("Event type is not supported")
	}

	return nil
}

func (r *OperatorEventReceiver) operationsForAddCluster(cassandra *v1alpha1.Cassandra) []operations.Operation {
	ops := []operations.Operation{r.operationFactory.NewAddCluster(cassandra)}
	if cassandra.Spec.Snapshot != nil {
		ops = append(ops, r.operationFactory.NewAddSnapshot(cassandra))
		if cassandra.Spec.Snapshot.RetentionPolicy != nil {
			ops = append(ops, r.operationFactory.NewAddSnapshotCleanup(cassandra))
		}
	}
	return ops
}

func (r *OperatorEventReceiver) operationsForDeleteCluster(cassandra *v1alpha1.Cassandra) []operations.Operation {
	ops := []operations.Operation{r.operationFactory.NewDeleteCluster(cassandra)}
	if cassandra.Spec.Snapshot != nil {
		ops = append(ops, r.operationFactory.NewDeleteSnapshot(cassandra))
		if cassandra.Spec.Snapshot.RetentionPolicy != nil {
			ops = append(ops, r.operationFactory.NewDeleteSnapshotCleanup(cassandra))
		}
	}
	return ops
}

func (r *OperatorEventReceiver) operationsForUpdateCluster(clusterUpdate ClusterUpdate) []operations.Operation {
	var ops []operations.Operation
	oldCluster := clusterUpdate.OldCluster
	newCluster := clusterUpdate.NewCluster

	ops = append(ops, r.operationFactory.NewUpdateCluster(clusterUpdate.OldCluster, clusterUpdate.NewCluster))
	if newCluster.Spec.Snapshot == nil && oldCluster.Spec.Snapshot != nil {
		ops = append(ops, r.operationFactory.NewDeleteSnapshot(clusterUpdate.NewCluster))
		if oldCluster.Spec.Snapshot.RetentionPolicy != nil {
			ops = append(ops, r.operationFactory.NewDeleteSnapshotCleanup(clusterUpdate.NewCluster))
		}
	} else if newCluster.Spec.Snapshot != nil && oldCluster.Spec.Snapshot != nil {
		if newCluster.Spec.Snapshot.RetentionPolicy == nil && oldCluster.Spec.Snapshot.RetentionPolicy != nil {
			ops = append(ops, r.operationFactory.NewDeleteSnapshotCleanup(clusterUpdate.NewCluster))
		}
		if v1alpha1helpers.SnapshotPropertiesUpdated(oldCluster.Spec.Snapshot, newCluster.Spec.Snapshot) {
			ops = append(ops, r.operationFactory.NewUpdateSnapshot(newCluster))
		}
		if v1alpha1helpers.SnapshotCleanupPropertiesUpdated(oldCluster.Spec.Snapshot, newCluster.Spec.Snapshot) {
			ops = append(ops, r.operationFactory.NewUpdateSnapshotCleanup(newCluster))
		}
	} else if newCluster.Spec.Snapshot != nil && oldCluster.Spec.Snapshot == nil {
		ops = append(ops, r.operationFactory.NewAddSnapshot(clusterUpdate.NewCluster))
		if newCluster.Spec.Snapshot.RetentionPolicy != nil {
			ops = append(ops, r.operationFactory.NewAddSnapshotCleanup(clusterUpdate.NewCluster))
		}
	}
	return ops
}

func (r *OperatorEventReceiver) eventLogger(event *Event) *log.Entry {
	logger := log.WithFields(
		log.Fields{
			"type":   event.Kind,
			"key":    event.Key,
			"logger": "receiver.go",
		},
	)
	return logger
}

func logOperationNotAllowed(logger *log.Entry) {
	logger.Warn("Operation is not allowed. Cluster does not exist or is being deleted")
}

func (r *OperatorEventReceiver) clusterDoesNotExistOrIsBeingDeleted(clusterToFind *v1alpha1.Cassandra) bool {
	cassandra, err := r.clusterAccessor.GetCassandraForCluster(clusterToFind.Namespace, clusterToFind.Name)
	return errors.IsNotFound(err) || cassandra.DeletionTimestamp != nil
}
