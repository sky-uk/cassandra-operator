package operations

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
)

// AddSnapshotCleanupOperation describes what the operator does when a snapshot cleanup definition is added
type AddSnapshotCleanupOperation struct {
	cassandra       *v1alpha1.Cassandra
	clusterAccessor cluster.Accessor
	eventRecorder   record.EventRecorder
}

// Execute performs the operation
func (o *AddSnapshotCleanupOperation) Execute() {
	o.addSnapshotCleanupJob(cluster.New(o.cassandra))
}

func (o *AddSnapshotCleanupOperation) addSnapshotCleanupJob(c *cluster.Cluster) {
	_, err := o.clusterAccessor.CreateCronJobForCluster(c, c.CreateSnapshotCleanupJob())
	if err != nil {
		log.Errorf("Error while creating snapshot cleanup job for cluster %s: %v", c.QualifiedName(), err)
		return
	}
	o.eventRecorder.Eventf(c.Definition(), v1.EventTypeNormal, cluster.ClusterSnapshotCleanupScheduleEvent, "Snapshot cleanup scheduled for cluster %s", c.QualifiedName())
}

func (o *AddSnapshotCleanupOperation) String() string {
	return fmt.Sprintf("add snapshot cleanup schedule for cluster %s", o.cassandra.QualifiedName())
}
