package cluster

import (
	"context"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/ptr"
	"k8s.io/api/batch/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	"reflect"

	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type snapshotStateFinder interface {
	findSnapshotStateFor(desiredCassandra *v1alpha1.Cassandra) (*v1alpha1.Snapshot, error)
}

// implements snapshotStateFinder
type currentSnapshotStateFinder struct {
	client        client.Client
	objectFactory objectReferenceFactory
}

type snapshotCleanupStateFinder interface {
	findSnapshotCleanupStateFor(desiredCassandra *v1alpha1.Cassandra) (*v1alpha1.RetentionPolicy, error)
}

// implements snapshotCleanupStateFinder
type currentSnapshotCleanupStateFinder struct {
	client        client.Client
	objectFactory objectReferenceFactory
}

func (snap *currentSnapshotStateFinder) findSnapshotStateFor(desiredCassandra *v1alpha1.Cassandra) (*v1alpha1.Snapshot, error) {
	snapshotJob := snap.objectFactory.newCronJob()
	err := snap.client.Get(
		context.TODO(),
		types.NamespacedName{Namespace: desiredCassandra.Namespace, Name: desiredCassandra.SnapshotJobName()},
		snapshotJob,
	)
	if err == nil {
		return snap.buildSnapshotFrom(snapshotJob, desiredCassandra), nil
	}
	return nil, err
}

func (snap *currentSnapshotStateFinder) buildSnapshotFrom(job *v1beta1.CronJob, desiredCassandra *v1alpha1.Cassandra) *v1alpha1.Snapshot {
	c := New(desiredCassandra)
	expectedCommand := c.snapshotCommand()
	jobContainer := job.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
	if reflect.DeepEqual(expectedCommand, jobContainer.Command) {
		return &v1alpha1.Snapshot{
			Image:          ptr.String(jobContainer.Image),
			Schedule:       job.Spec.Schedule,
			TimeoutSeconds: desiredCassandra.Spec.Snapshot.TimeoutSeconds,
			Keyspaces:      desiredCassandra.Spec.Snapshot.Keyspaces,
		}
	}

	// not attempting to guess other properties from the job command line
	return &v1alpha1.Snapshot{
		Image:    ptr.String(jobContainer.Image),
		Schedule: job.Spec.Schedule,
	}
}

func (cleanup *currentSnapshotCleanupStateFinder) findSnapshotCleanupStateFor(desiredCassandra *v1alpha1.Cassandra) (*v1alpha1.RetentionPolicy, error) {
	cleanupJob := cleanup.objectFactory.newCronJob()
	err := cleanup.client.Get(
		context.TODO(),
		types.NamespacedName{Namespace: desiredCassandra.Namespace, Name: desiredCassandra.SnapshotCleanupJobName()},
		cleanupJob,
	)
	if err == nil {
		return cleanup.buildSnapshotCleanupFrom(cleanupJob, desiredCassandra), nil
	}
	return nil, err
}

func (cleanup *currentSnapshotCleanupStateFinder) buildSnapshotCleanupFrom(job *v1beta1.CronJob, desiredCassandra *v1alpha1.Cassandra) *v1alpha1.RetentionPolicy {
	c := New(desiredCassandra)
	expectedCommand := c.snapshotCleanupCommand()
	jobContainer := job.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
	if reflect.DeepEqual(expectedCommand, jobContainer.Command) {
		return &v1alpha1.RetentionPolicy{
			CleanupSchedule:       job.Spec.Schedule,
			CleanupTimeoutSeconds: desiredCassandra.Spec.Snapshot.RetentionPolicy.CleanupTimeoutSeconds,
			RetentionPeriodDays:   desiredCassandra.Spec.Snapshot.RetentionPolicy.RetentionPeriodDays,
		}
	}

	// not attempting to guess other properties from the job command line
	return &v1alpha1.RetentionPolicy{
		CleanupSchedule: job.Spec.Schedule,
	}
}
