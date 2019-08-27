package operations

import (
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"k8s.io/api/core/v1"
)

// Operation describes a single unit of work
type Operation interface {
	// Execute actually performs the operation
	Execute()
	// Human-readable description of the operation
	String() string
}

func (r *Receiver) newAddCluster(cassandra *v1alpha1.Cassandra) Operation {
	return &AddClusterOperation{
		clusterAccessor:     r.clusterAccessor,
		statefulSetAccessor: r.statefulSetAccessor,
		cassandra:           cassandra,
	}
}

func (r *Receiver) newAddService(cassandra *v1alpha1.Cassandra) Operation {
	return &AddServiceOperation{
		clusterAccessor: r.clusterAccessor,
		cassandra:       cassandra,
	}
}

func (r *Receiver) newDeleteCluster(cassandra *v1alpha1.Cassandra) Operation {
	return &DeleteClusterOperation{
		clusterAccessor: r.clusterAccessor,
		cassandra:       cassandra,
		metricsPoller:   r.metricsPoller,
	}
}

func (r *Receiver) newDeleteSnapshot(cassandra *v1alpha1.Cassandra) Operation {
	return &DeleteSnapshotOperation{
		cassandra:       cassandra,
		clusterAccessor: r.clusterAccessor,
		eventRecorder:   r.eventRecorder,
	}
}

func (r *Receiver) newDeleteSnapshotCleanup(cassandra *v1alpha1.Cassandra) Operation {
	return &DeleteSnapshotCleanupOperation{
		cassandra:       cassandra,
		clusterAccessor: r.clusterAccessor,
		eventRecorder:   r.eventRecorder,
	}
}

func (r *Receiver) newAddSnapshot(cassandra *v1alpha1.Cassandra) Operation {
	return &AddSnapshotOperation{
		cassandra:       cassandra,
		clusterAccessor: r.clusterAccessor,
		eventRecorder:   r.eventRecorder,
	}
}

func (r *Receiver) newAddSnapshotCleanup(cassandra *v1alpha1.Cassandra) Operation {
	return &AddSnapshotCleanupOperation{
		cassandra:       cassandra,
		clusterAccessor: r.clusterAccessor,
		eventRecorder:   r.eventRecorder,
	}
}

func (r *Receiver) newUpdateCluster(update ClusterUpdate) Operation {
	return &UpdateClusterOperation{
		adjuster:            r.adjuster,
		eventRecorder:       r.eventRecorder,
		statefulSetAccessor: r.statefulSetAccessor,
		clusterAccessor:     r.clusterAccessor,
		update:              update,
	}
}

func (r *Receiver) newUpdateSnapshot(cassandra *v1alpha1.Cassandra) Operation {
	return &UpdateSnapshotOperation{
		cassandra:       cassandra,
		clusterAccessor: r.clusterAccessor,
		eventRecorder:   r.eventRecorder,
	}
}

func (r *Receiver) newUpdateSnapshotCleanup(cassandra *v1alpha1.Cassandra) Operation {
	return &UpdateSnapshotCleanupOperation{
		cassandra:       cassandra,
		clusterAccessor: r.clusterAccessor,
		eventRecorder:   r.eventRecorder,
	}
}

func (r *Receiver) newGatherMetrics(cassandra *v1alpha1.Cassandra) Operation {
	return &GatherMetricsOperation{metricsPoller: r.metricsPoller, cassandra: cassandra}
}

func (r *Receiver) newUpdateCustomConfig(cassandra *v1alpha1.Cassandra, configMap *v1.ConfigMap) Operation {
	return &UpdateCustomConfigOperation{
		cassandra:           cassandra,
		configMap:           configMap,
		eventRecorder:       r.eventRecorder,
		statefulSetAccessor: r.statefulSetAccessor,
		adjuster:            r.adjuster,
	}
}

func (r *Receiver) newAddCustomConfig(cassandra *v1alpha1.Cassandra, configMap *v1.ConfigMap) Operation {
	return &AddCustomConfigOperation{
		cassandra:           cassandra,
		configMap:           configMap,
		eventRecorder:       r.eventRecorder,
		statefulSetAccessor: r.statefulSetAccessor,
		adjuster:            r.adjuster,
	}
}

func (r *Receiver) newDeleteCustomConfig(cassandra *v1alpha1.Cassandra) Operation {
	return &DeleteCustomConfigOperation{
		cassandra:           cassandra,
		eventRecorder:       r.eventRecorder,
		statefulSetAccessor: r.statefulSetAccessor,
	}
}
