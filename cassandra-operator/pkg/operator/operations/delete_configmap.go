package operations

import (
	"fmt"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
)

// DeleteCustomConfigOperation describes what the operator does when a configmap is removed for a cluster
type DeleteCustomConfigOperation struct {
	cassandra           *v1alpha1.Cassandra
	configMap           *v1.ConfigMap
	eventRecorder       record.EventRecorder
	statefulSetAccessor *statefulSetAccessor
}

// Execute performs the operation
func (o *DeleteCustomConfigOperation) Execute() (bool, error) {
	c := cluster.New(o.cassandra)
	o.eventRecorder.Eventf(o.cassandra, v1.EventTypeNormal, cluster.ClusterUpdateEvent, "Custom config deleted for cluster %s", o.cassandra.QualifiedName())
	for _, rack := range o.cassandra.Spec.Racks {
		err := o.statefulSetAccessor.updateStatefulSet(c, o.configMap, &rack, c.RemoveCustomConfigVolumeFromStatefulSet)
		if err == cluster.ErrReconciliationInterrupted {
			return true, nil
		}

		if err != nil {
			return false, fmt.Errorf("unable to remove custom configMap from statefulSet for rack %s in cluster %s: %v. Other racks will not be updated", rack.Name, o.cassandra.QualifiedName(), err)
		}
	}
	return false, nil
}

func (o *DeleteCustomConfigOperation) String() string {
	return fmt.Sprintf("remove custom config for cluster %s", o.cassandra.QualifiedName())
}
