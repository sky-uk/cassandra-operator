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
	clusterAccessor     *cluster.Accessor
	statefulSetAccessor *statefulSetAccessor
	metricsPoller       *metrics.PrometheusMetrics
	eventRecorder       record.EventRecorder
	adjuster            *adjuster.Adjuster
}

// NewEventReceiver creates a new Receiver
func NewEventReceiver(clusterAccessor *cluster.Accessor, metricsPoller *metrics.PrometheusMetrics, eventRecorder record.EventRecorder) *Receiver {
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
	logger := log.WithFields(
		log.Fields{
			"type": event.Kind,
			"key":  event.Key,
		},
	)
	logger.Debugf("Event received")
	operations := r.operationsToExecute(event)
	logger.Infof("Event will trigger %d operations", len(operations))

	for _, operation := range operations {
		logger.Debugf("Executing operation %s", operation.String())
		operation.Execute()
	}
}

func (r *Receiver) operationsToExecute(event *dispatcher.Event) []Operation {
	switch event.Kind {
	case AddCluster:
		return r.operationsForAddCluster(event.Data.(*v1alpha1.Cassandra))
	case DeleteCluster:
		return r.operationsForDeleteCluster(event.Data.(*v1alpha1.Cassandra))
	case UpdateCluster:
		return r.operationsForUpdateCluster(event.Data.(ClusterUpdate))
	case GatherMetrics:
		return []Operation{r.newGatherMetrics(event.Data.(*v1alpha1.Cassandra))}
	case UpdateCustomConfig:
		configMapChange := event.Data.(ConfigMapChange)
		return []Operation{r.newUpdateCustomConfig(configMapChange.Cassandra, configMapChange.ConfigMap)}
	case AddCustomConfig:
		configMapChange := event.Data.(ConfigMapChange)
		return []Operation{r.newAddCustomConfig(configMapChange.Cassandra, configMapChange.ConfigMap)}
	case DeleteCustomConfig:
		configMapChange := event.Data.(ConfigMapChange)
		return []Operation{r.newDeleteCustomConfig(configMapChange.Cassandra)}
	default:
		log.Errorf("Event type %s is not supported", event.Kind)
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
