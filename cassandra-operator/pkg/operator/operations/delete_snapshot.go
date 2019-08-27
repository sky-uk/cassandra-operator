package operations

import (
	"fmt"
	log "github.com/sirupsen/logrus"
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
func (o *DeleteSnapshotOperation) Execute() {
	qualifiedName := o.cassandra.QualifiedName()
	job, err := o.clusterAccessor.FindCronJobForCluster(o.cassandra, fmt.Sprintf("app=%s", o.cassandra.SnapshotJobName()))
	if err != nil {
		log.Errorf("Error while retrieving snapshot job list for cluster %s: %v", qualifiedName, err)
	}

	if job != nil {
		err = o.clusterAccessor.DeleteCronJob(job)
		if err != nil {
			log.Errorf("Error while deleting snapshot job %s for cluster %s: %v", job.Name, qualifiedName, err)
		}
		o.eventRecorder.Eventf(o.cassandra, v1.EventTypeNormal, cluster.ClusterSnapshotCreationUnscheduleEvent, "Snapshot creation unscheduled for cluster %s", qualifiedName)
	}
}

func (o *DeleteSnapshotOperation) String() string {
	return fmt.Sprintf("delete snapshot schedule for cluster %s", o.cassandra.QualifiedName())
}
