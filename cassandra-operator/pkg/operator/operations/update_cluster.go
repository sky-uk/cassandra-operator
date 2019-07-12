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
	cluster             *cluster.Cluster
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
	if err := cluster.CopyInto(o.cluster, newCluster); err != nil {
		log.Errorf("Cluster definition %s.%s is invalid: %v", newCluster.Namespace, newCluster.Name, err)
		return
	}

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

	for _, clusterChange := range clusterChanges {
		switch clusterChange.ChangeType {
		case adjuster.UpdateRack:
			err := o.statefulSetAccessor.patchStatefulSet(o.cluster, &clusterChange)
			if err != nil {
				log.Error(err)
				return
			}
		case adjuster.AddRack:
			log.Infof("Adding new rack %s to cluster %s", clusterChange.Rack.Name, o.cluster.QualifiedName())

			customConfigMap := o.clusterAccessor.FindCustomConfigMap(o.cluster.Namespace(), o.cluster.Name())
			if err := o.statefulSetAccessor.registerStatefulSet(o.cluster, &clusterChange.Rack, customConfigMap); err != nil {
				log.Errorf("Error while creating stateful sets for added rack %s in cluster %s: %v", clusterChange.Rack.Name, o.cluster.QualifiedName(), err)
				return
			}
		default:
			message := fmt.Sprintf("Change type '%s' isn't supported for cluster %s", clusterChange.ChangeType, o.cluster.QualifiedName())
			log.Error(message)
			o.eventRecorder.Event(oldCluster, v1.EventTypeWarning, cluster.InvalidChangeEvent, message)
		}
	}
}

func (o *UpdateClusterOperation) String() string {
	return fmt.Sprintf("update cluster %s", o.cluster.QualifiedName())
}
