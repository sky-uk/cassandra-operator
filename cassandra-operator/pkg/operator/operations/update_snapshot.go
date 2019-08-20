package operations

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"k8s.io/api/batch/v1beta1"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
)

// UpdateSnapshotOperation describes what the operator does when the Snapshot spec is updated for a cluster
type UpdateSnapshotOperation struct {
	cassandra       *v1alpha1.Cassandra
	clusterAccessor *cluster.Accessor
	eventRecorder   record.EventRecorder
}

// Execute performs the operation
func (o *UpdateSnapshotOperation) Execute() {
	job, err := o.clusterAccessor.FindCronJobForCluster(o.cassandra, fmt.Sprintf("app=%s", o.cassandra.SnapshotJobName()))
	if err != nil {
		log.Errorf("Error while retrieving snapshot job for cluster %s: %v", o.cassandra.QualifiedName(), err)
	}

	if job != nil {
		o.updateSnapshotJob(job)
	}
}

func (o *UpdateSnapshotOperation) updateSnapshotJob(snapshotJob *v1beta1.CronJob) {
	c := cluster.New(o.cassandra)
	snapshotJob.Spec.Schedule = o.cassandra.Spec.Snapshot.Schedule
	snapshotJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0] = *c.CreateSnapshotContainer(o.cassandra.Spec.Snapshot)
	err := o.clusterAccessor.UpdateCronJob(snapshotJob)
	if err != nil {
		log.Errorf("Error while updating snapshot snapshotJob %s for cluster %s: %v", snapshotJob.Name, o.cassandra.QualifiedName(), err)
		return
	}
	o.eventRecorder.Eventf(o.cassandra, v1.EventTypeNormal, cluster.ClusterSnapshotCreationModificationEvent, "Snapshot creation modified for cluster %s", o.cassandra.QualifiedName())
}

func (o *UpdateSnapshotOperation) String() string {
	return fmt.Sprintf("update snapshot schedule for cluster %s", o.cassandra.QualifiedName())
}
