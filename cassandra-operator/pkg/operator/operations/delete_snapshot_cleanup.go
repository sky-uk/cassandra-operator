package operations

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
)

// DeleteSnapshotCleanupOperation describes what the operator does when a snapshot cleanup definition is removed
type DeleteSnapshotCleanupOperation struct {
	cassandra       *v1alpha1.Cassandra
	clusterAccessor cluster.Accessor
	eventRecorder   record.EventRecorder
}

// Execute performs the operation
func (o *DeleteSnapshotCleanupOperation) Execute() {
	qualifiedName := o.cassandra.QualifiedName()
	job, err := o.clusterAccessor.FindCronJobForCluster(o.cassandra, fmt.Sprintf("app=%s", o.cassandra.SnapshotCleanupJobName()))
	if err != nil {
		log.Errorf("Error while retrieving snapshot cleanup job for cluster %s: %v", qualifiedName, err)
	}

	if job != nil {
		err = o.clusterAccessor.DeleteCronJob(job)
		if err != nil {
			log.Errorf("Error while deleting snapshot cleanup job %s for cluster %s: %v", job.Name, qualifiedName, err)
		}
		o.eventRecorder.Eventf(o.cassandra, v1.EventTypeNormal, cluster.ClusterSnapshotCleanupUnscheduleEvent, "Snapshot cleanup unscheduled for cluster %s", qualifiedName)
	}
}

func (o *DeleteSnapshotCleanupOperation) String() string {
	return fmt.Sprintf("delete snapshot cleanup schedule for cluster %s", o.cassandra.QualifiedName())
}
