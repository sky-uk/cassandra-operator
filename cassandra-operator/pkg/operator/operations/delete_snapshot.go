package operations

import (
	"fmt"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
)

// DeleteSnapshotOperation describes what the operator does when a Snapshot schedule is removed for a cluster
type DeleteSnapshotOperation struct {
	cassandra       *v1alpha1.Cassandra
	clusterAccessor cluster.Accessor
	eventRecorder   record.EventRecorder
}

// Execute performs the operation
func (o *DeleteSnapshotOperation) Execute() (bool, error) {
	qualifiedName := o.cassandra.QualifiedName()
	job, err := o.clusterAccessor.FindCronJobForCluster(o.cassandra, cluster.SnapshotCronJob)
	if err != nil {
		return false, fmt.Errorf("error while retrieving snapshot job list for cluster %s: %v", qualifiedName, err)
	}

	if job != nil {
		err = o.clusterAccessor.DeleteCronJob(job)
		if err != nil {
			return false, fmt.Errorf("error while deleting snapshot job %s for cluster %s: %v", job.Name, qualifiedName, err)
		}
		o.eventRecorder.Eventf(o.cassandra, v1.EventTypeNormal, cluster.ClusterSnapshotCreationUnscheduleEvent, "Snapshot creation unscheduled for cluster %s", qualifiedName)
	}
	return false, nil
}

func (o *DeleteSnapshotOperation) String() string {
	return fmt.Sprintf("delete snapshot schedule for cluster %s", o.cassandra.QualifiedName())
}
