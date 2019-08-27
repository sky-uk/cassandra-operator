package operations

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
)

// AddSnapshotOperation describes what the operator does when a snapshot schedule is added for a cluster
type AddSnapshotOperation struct {
	cassandra       *v1alpha1.Cassandra
	clusterAccessor cluster.Accessor
	eventRecorder   record.EventRecorder
}

// Execute performs the operation
func (o *AddSnapshotOperation) Execute() {
	o.addSnapshotJob(cluster.New(o.cassandra))
}

func (o *AddSnapshotOperation) addSnapshotJob(c *cluster.Cluster) {
	_, err := o.clusterAccessor.CreateCronJobForCluster(c, c.CreateSnapshotJob())
	if err != nil {
		log.Errorf("Error while creating snapshot creation job for cluster %s: %v", c.QualifiedName(), err)
		return
	}
	o.eventRecorder.Eventf(c.Definition(), v1.EventTypeNormal, cluster.ClusterSnapshotCreationScheduleEvent, "Snapshot creation scheduled for cluster %s", c.QualifiedName())
}

func (o *AddSnapshotOperation) String() string {
	return fmt.Sprintf("add snapshot schedule for cluster %s", o.cassandra.QualifiedName())
}
