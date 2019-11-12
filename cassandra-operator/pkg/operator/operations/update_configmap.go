package operations

import (
	"fmt"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/operator/operations/adjuster"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
)

// UpdateCustomConfigOperation describes what the operator does when a configmap is updated for a cluster
type UpdateCustomConfigOperation struct {
	cassandra           *v1alpha1.Cassandra
	configMap           *v1.ConfigMap
	eventRecorder       record.EventRecorder
	adjuster            *adjuster.Adjuster
	statefulSetAccessor *statefulSetAccessor
}

// Execute performs the operation
func (o *UpdateCustomConfigOperation) Execute() (bool, error) {
	c := cluster.New(o.cassandra)
	o.eventRecorder.Eventf(o.cassandra, v1.EventTypeNormal, cluster.ClusterUpdateEvent, "Custom config updated for cluster %s", o.cassandra.QualifiedName())
	for _, rack := range o.cassandra.Spec.Racks {
		patchChange := o.adjuster.CreateConfigMapHashPatchForRack(&rack, o.configMap)
		err := o.statefulSetAccessor.patchStatefulSet(c, patchChange)
		if err == cluster.ErrReconciliationInterrupted {
			return true, nil
		}

		if err != nil {
			return false, fmt.Errorf("error while attempting to update rack %s in cluster %s as a result of a custom config change. No further updates will be applied: %v", rack.Name, o.cassandra.QualifiedName(), err)
		}
	}
	return false, nil
}

func (o *UpdateCustomConfigOperation) String() string {
	return fmt.Sprintf("update custom config for cluster %s", o.cassandra.QualifiedName())
}
