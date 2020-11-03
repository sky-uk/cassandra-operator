package operations

import (
	"fmt"
	"github.com/hashicorp/go-multierror"
	log "github.com/sirupsen/logrus"

	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/metrics"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/operator/operations/adjuster"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
)

// Operation describes a single unit of work
type Operation interface {
	// Execute actually performs the operation.
	// stop will be true if subsequent operations should not be performed as a consequence of something which happened
	// during this operation.
	Execute() (stop bool, err error)
	// Human-readable description of the operation
	String() string
}

// OperationComposer takes a sequence of Operations and executes them one by one.
type OperationComposer interface {
	// Execute the supplied operations in order.
	// stopped will be true upon return if not all operations were executed.
	// err wraps all errors which arose in the execution of the operations.
	Execute(operations []Operation) (stopped bool, err error)
}

// NewOperationComposer returns an OperationComposer
func NewOperationComposer() OperationComposer {
	return &operationComposer{}
}

type operationComposer struct{}

func (c *operationComposer) Execute(operations []Operation) (bool, error) {
	stopped := false
	allErrors := &multierror.Error{}
	for _, operation := range operations {
		log.Debugf("Executing operation %s", operation.String())
		stop, err := operation.Execute()
		if err != nil {
			allErrors = multierror.Append(allErrors, fmt.Errorf("error while executing operation %s: %v", operation.String(), err))
		}

		if stop {
			stopped = true
			log.Debugf("Operation execution interrupted during %s. No further operations will be executed.", operation.String())
			break
		}
	}

	return stopped, allErrors.ErrorOrNil()
}

// OperationFactory creates Operation for each operation supported by the operator
type OperationFactory interface {
	NewAddCluster(cassandra *v1alpha1.Cassandra) Operation
	NewAddService(cassandra *v1alpha1.Cassandra) Operation
	NewDeleteCluster(cassandra *v1alpha1.Cassandra) Operation
	NewDeleteSnapshot(cassandra *v1alpha1.Cassandra) Operation
	NewDeleteSnapshotCleanup(cassandra *v1alpha1.Cassandra) Operation
	NewAddSnapshot(cassandra *v1alpha1.Cassandra) Operation
	NewAddSnapshotCleanup(cassandra *v1alpha1.Cassandra) Operation
	NewUpdateCluster(oldCassandra, newCassandra *v1alpha1.Cassandra) Operation
	NewUpdateSnapshot(cassandra *v1alpha1.Cassandra) Operation
	NewUpdateSnapshotCleanup(cassandra *v1alpha1.Cassandra) Operation
	NewGatherMetrics(cassandra *v1alpha1.Cassandra) Operation
	NewUpdateCustomConfig(cassandra *v1alpha1.Cassandra, configMap *v1.ConfigMap) Operation
	NewAddCustomConfig(cassandra *v1alpha1.Cassandra, configMap *v1.ConfigMap) Operation
	NewDeleteCustomConfig(cassandra *v1alpha1.Cassandra) Operation
}

// OperationFactoryImpl implements OperationFactory
type OperationFactoryImpl struct {
	clusterAccessor     cluster.Accessor
	statefulSetAccessor *statefulSetAccessor
	metricsReporter     metrics.ClusterMetricsReporter
	eventRecorder       record.EventRecorder
	adjuster            *adjuster.Adjuster
}

// NewOperationFactory creates a new OperationFactory
func NewOperationFactory(clusterAccessor cluster.Accessor, metricsReporter metrics.ClusterMetricsReporter, eventRecorder record.EventRecorder, adjuster *adjuster.Adjuster) OperationFactory {
	return &OperationFactoryImpl{
		clusterAccessor:     clusterAccessor,
		statefulSetAccessor: &statefulSetAccessor{clusterAccessor: clusterAccessor},
		metricsReporter:     metricsReporter,
		eventRecorder:       eventRecorder,
		adjuster:            adjuster,
	}
}

// NewAddCluster creates a AddCluster Operation
func (o *OperationFactoryImpl) NewAddCluster(cassandra *v1alpha1.Cassandra) Operation {
	return &AddClusterOperation{
		clusterAccessor:     o.clusterAccessor,
		statefulSetAccessor: o.statefulSetAccessor,
		cassandra:           cassandra,
	}
}

// NewAddService creates a AddService Operation
func (o *OperationFactoryImpl) NewAddService(cassandra *v1alpha1.Cassandra) Operation {
	return &AddServiceOperation{
		clusterAccessor: o.clusterAccessor,
		cassandra:       cassandra,
	}
}

// NewDeleteCluster creates a DeleteCluster Operation
func (o *OperationFactoryImpl) NewDeleteCluster(cassandra *v1alpha1.Cassandra) Operation {
	return &DeleteClusterOperation{
		cassandra:       cassandra,
		metricsReporter: o.metricsReporter,
	}
}

// NewDeleteSnapshot creates a DeleteSnapshot Operation
func (o *OperationFactoryImpl) NewDeleteSnapshot(cassandra *v1alpha1.Cassandra) Operation {
	return &DeleteSnapshotOperation{
		cassandra:       cassandra,
		clusterAccessor: o.clusterAccessor,
		eventRecorder:   o.eventRecorder,
	}
}

// NewDeleteSnapshotCleanup creates a DeleteSnapshotCleanup Operation
func (o *OperationFactoryImpl) NewDeleteSnapshotCleanup(cassandra *v1alpha1.Cassandra) Operation {
	return &DeleteSnapshotCleanupOperation{
		cassandra:       cassandra,
		clusterAccessor: o.clusterAccessor,
		eventRecorder:   o.eventRecorder,
	}
}

// NewAddSnapshot creates a AddSnapshot Operation
func (o *OperationFactoryImpl) NewAddSnapshot(cassandra *v1alpha1.Cassandra) Operation {
	return &AddSnapshotOperation{
		cassandra:       cassandra,
		clusterAccessor: o.clusterAccessor,
		eventRecorder:   o.eventRecorder,
	}
}

// NewAddSnapshotCleanup creates a AddSnapshotCleanup Operation
func (o *OperationFactoryImpl) NewAddSnapshotCleanup(cassandra *v1alpha1.Cassandra) Operation {
	return &AddSnapshotCleanupOperation{
		cassandra:       cassandra,
		clusterAccessor: o.clusterAccessor,
		eventRecorder:   o.eventRecorder,
	}
}

// NewUpdateCluster creates a UpdateCluster Operation
func (o *OperationFactoryImpl) NewUpdateCluster(oldCassandra, newCassandra *v1alpha1.Cassandra) Operation {
	return &UpdateClusterOperation{
		adjuster:            o.adjuster,
		eventRecorder:       o.eventRecorder,
		statefulSetAccessor: o.statefulSetAccessor,
		clusterAccessor:     o.clusterAccessor,
		oldCassandra:        oldCassandra,
		newCassandra:        newCassandra,
		metricsReporter:     o.metricsReporter,
	}
}

// NewUpdateSnapshot creates a UpdateSnapshot Operation
func (o *OperationFactoryImpl) NewUpdateSnapshot(cassandra *v1alpha1.Cassandra) Operation {
	return &UpdateSnapshotOperation{
		cassandra:       cassandra,
		clusterAccessor: o.clusterAccessor,
		eventRecorder:   o.eventRecorder,
	}
}

// NewUpdateSnapshotCleanup creates a UpdateSnapshotCleanup Operation
func (o *OperationFactoryImpl) NewUpdateSnapshotCleanup(cassandra *v1alpha1.Cassandra) Operation {
	return &UpdateSnapshotCleanupOperation{
		cassandra:       cassandra,
		clusterAccessor: o.clusterAccessor,
		eventRecorder:   o.eventRecorder,
	}
}

// NewGatherMetrics creates a GatherMetrics Operation
func (o *OperationFactoryImpl) NewGatherMetrics(cassandra *v1alpha1.Cassandra) Operation {
	return &GatherMetricsOperation{metricsReporter: o.metricsReporter, cassandra: cassandra}
}

// NewUpdateCustomConfig creates a UpdateCustomConfig Operation
func (o *OperationFactoryImpl) NewUpdateCustomConfig(cassandra *v1alpha1.Cassandra, configMap *v1.ConfigMap) Operation {
	return &UpdateCustomConfigOperation{
		cassandra:           cassandra,
		configMap:           configMap,
		eventRecorder:       o.eventRecorder,
		statefulSetAccessor: o.statefulSetAccessor,
		adjuster:            o.adjuster,
	}
}

// NewAddCustomConfig creates a AddCustomConfig Operation
func (o *OperationFactoryImpl) NewAddCustomConfig(cassandra *v1alpha1.Cassandra, configMap *v1.ConfigMap) Operation {
	return &AddCustomConfigOperation{
		cassandra:           cassandra,
		configMap:           configMap,
		eventRecorder:       o.eventRecorder,
		statefulSetAccessor: o.statefulSetAccessor,
		adjuster:            o.adjuster,
	}
}

// NewDeleteCustomConfig creates a DeleteCustomConfig Operation
func (o *OperationFactoryImpl) NewDeleteCustomConfig(cassandra *v1alpha1.Cassandra) Operation {
	return &DeleteCustomConfigOperation{
		cassandra:           cassandra,
		eventRecorder:       o.eventRecorder,
		statefulSetAccessor: o.statefulSetAccessor,
	}
}
