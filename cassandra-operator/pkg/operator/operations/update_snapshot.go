package operations

import (
	"fmt"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"k8s.io/api/batch/v1beta1"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
)

// UpdateSnapshotOperation describes what the operator does when the Snapshot spec is updated for a cluster
type UpdateSnapshotOperation struct {
	cassandra       *v1alpha1.Cassandra
	clusterAccessor cluster.Accessor
	eventRecorder   record.EventRecorder
}

// Execute performs the operation
func (o *UpdateSnapshotOperation) Execute() (bool, error) {
	job, err := o.clusterAccessor.FindCronJobForCluster(o.cassandra, fmt.Sprintf("app=%s", o.cassandra.SnapshotJobName()))
	if err != nil {
		return false, fmt.Errorf("error while retrieving snapshot job for cluster %s: %v", o.cassandra.QualifiedName(), err)
	}

	if job != nil {
		return false, o.updateSnapshotJob(job)
	}
	return false, nil
}

func (o *UpdateSnapshotOperation) updateSnapshotJob(snapshotJob *v1beta1.CronJob) error {
	c := cluster.New(o.cassandra)
	snapshotJob.Spec.Schedule = o.cassandra.Spec.Snapshot.Schedule
	snapshotJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0] = *c.CreateSnapshotContainer(o.cassandra.Spec.Snapshot)
	err := o.clusterAccessor.UpdateCronJob(snapshotJob)
	if err != nil {
		return fmt.Errorf("error while updating snapshot snapshotJob %s for cluster %s: %v", snapshotJob.Name, o.cassandra.QualifiedName(), err)
	}
	o.eventRecorder.Eventf(o.cassandra, v1.EventTypeNormal, cluster.ClusterSnapshotCreationModificationEvent, "Snapshot creation modified for cluster %s", o.cassandra.QualifiedName())
	return nil
}

func (o *UpdateSnapshotOperation) String() string {
	return fmt.Sprintf("update snapshot schedule for cluster %s", o.cassandra.QualifiedName())
}
