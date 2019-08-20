package operations

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/operator/operations/adjuster"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
)

// UpdateClusterOperation describes what the operator does when the Cassandra spec is updated for a cluster
type UpdateClusterOperation struct {
	adjuster            *adjuster.Adjuster
	eventRecorder       record.EventRecorder
	statefulSetAccessor *statefulSetAccessor
	clusterAccessor     *cluster.Accessor
	update              ClusterUpdate
}

// Execute performs the operation
func (o *UpdateClusterOperation) Execute() {
	oldCluster := o.update.OldCluster
	newCluster := o.update.NewCluster

	log.Infof("Cluster definition has been updated for cluster %s.%s", oldCluster.Namespace, oldCluster.Name)
	c := cluster.New(newCluster)

	clusterChanges, err := o.adjuster.ChangesForCluster(oldCluster, newCluster)
	if err != nil {
		log.Warnf("unable to generate patch for cluster %s.%s: %v", newCluster.Namespace, newCluster.Name, err)
		o.eventRecorder.Eventf(oldCluster, v1.EventTypeWarning, cluster.InvalidChangeEvent, "unable to generate patch for cluster %s.%s: %v", newCluster.Namespace, newCluster.Name, err)
		return
	}

	if len(clusterChanges) == 0 {
		log.Infof("No changes are required to be applied for cluster %s", oldCluster.QualifiedName())
		return
	}

	log.Debugf("%d cluster changes will be applied: \n%v", len(clusterChanges), clusterChanges)
	for _, clusterChange := range clusterChanges {
		switch clusterChange.ChangeType {
		case adjuster.UpdateRack:
			err := o.statefulSetAccessor.patchStatefulSet(c, &clusterChange)
			if err != nil {
				log.Error(err)
				return
			}
		case adjuster.AddRack:
			log.Infof("Adding new rack %s to cluster %s", clusterChange.Rack.Name, newCluster.QualifiedName())

			customConfigMap := o.clusterAccessor.FindCustomConfigMap(newCluster.Namespace, newCluster.Name)
			if err := o.statefulSetAccessor.registerStatefulSet(c, &clusterChange.Rack, customConfigMap); err != nil {
				log.Errorf("Error while creating stateful sets for added rack %s in cluster %s: %v", clusterChange.Rack.Name, newCluster.QualifiedName(), err)
				return
			}
		default:
			message := fmt.Sprintf("Change type '%s' isn't supported for cluster %s", clusterChange.ChangeType, newCluster.QualifiedName())
			log.Error(message)
			o.eventRecorder.Event(oldCluster, v1.EventTypeWarning, cluster.InvalidChangeEvent, message)
		}
	}
}

func (o *UpdateClusterOperation) String() string {
	return fmt.Sprintf("update cluster %s", o.update.NewCluster.QualifiedName())
}
