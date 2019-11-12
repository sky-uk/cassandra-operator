package operations

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
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
	clusterAccessor     cluster.Accessor
	oldCassandra        *v1alpha1.Cassandra
	newCassandra        *v1alpha1.Cassandra
}

// Execute performs the operation
func (o *UpdateClusterOperation) Execute() (bool, error) {
	log.Infof("Cluster definition has been updated for cluster %s.%s", o.oldCassandra.Namespace, o.oldCassandra.Name)
	c := cluster.New(o.newCassandra)

	clusterChanges := o.adjuster.ChangesForCluster(o.oldCassandra, o.newCassandra)
	if len(clusterChanges) == 0 {
		log.Infof("No changes are required to be applied for cluster %s", o.oldCassandra.QualifiedName())
		return false, nil
	}

	log.Debugf("%d cluster changes will be applied: \n%v", len(clusterChanges), clusterChanges)
	for _, clusterChange := range clusterChanges {
		switch clusterChange.ChangeType {
		case adjuster.UpdateRack:
			customConfigMap := o.clusterAccessor.FindCustomConfigMap(o.newCassandra.Namespace, o.newCassandra.Name)
			err := o.statefulSetAccessor.updateStatefulSet(c, customConfigMap, &clusterChange.Rack, c.UpdateStatefulSetToDesiredState)
			if err == cluster.ErrReconciliationInterrupted {
				return true, nil
			}

			if err != nil {
				return false, fmt.Errorf("error while updating rack %s in cluster %s: %v", clusterChange.Rack.Name, o.newCassandra.QualifiedName(), err)
			}
		case adjuster.AddRack:
			log.Infof("Adding new rack %s to cluster %s", clusterChange.Rack.Name, o.newCassandra.QualifiedName())

			customConfigMap := o.clusterAccessor.FindCustomConfigMap(o.newCassandra.Namespace, o.newCassandra.Name)
			err := o.statefulSetAccessor.registerStatefulSet(c, &clusterChange.Rack, customConfigMap)
			if err == cluster.ErrReconciliationInterrupted {
				return true, nil
			}

			if err != nil {
				return false, fmt.Errorf("error while creating stateful sets for added rack %s in cluster %s: %v", clusterChange.Rack.Name, o.newCassandra.QualifiedName(), err)
			}
		default:
			message := fmt.Sprintf("Change type '%s' isn't supported for cluster %s", clusterChange.ChangeType, o.newCassandra.QualifiedName())
			o.eventRecorder.Event(o.oldCassandra, v1.EventTypeWarning, cluster.InvalidChangeEvent, message)
			return false, fmt.Errorf(message)
		}
	}
	return false, nil
}

func (o *UpdateClusterOperation) String() string {
	return fmt.Sprintf("update cluster %s", o.newCassandra.QualifiedName())
}
