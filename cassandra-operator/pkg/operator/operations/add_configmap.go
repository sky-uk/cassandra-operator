package operations

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/operator/operations/adjuster"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
)

// AddCustomConfigOperation describes what the operator does when a configmap is added
type AddCustomConfigOperation struct {
	cassandra           *v1alpha1.Cassandra
	configMap           *v1.ConfigMap
	eventRecorder       record.EventRecorder
	adjuster            *adjuster.Adjuster
	statefulSetAccessor *statefulSetAccessor
}

// Execute performs the operation
func (o *AddCustomConfigOperation) Execute() {
	c := cluster.New(o.cassandra)
	o.eventRecorder.Eventf(o.cassandra, v1.EventTypeNormal, cluster.ClusterUpdateEvent, "Custom config created for cluster %s", o.cassandra.QualifiedName())
	for _, rack := range o.cassandra.Spec.Racks {
		err := o.statefulSetAccessor.updateStatefulSet(c, o.configMap, &rack, c.AddCustomConfigVolumeToStatefulSet)
		if err != nil {
			log.Errorf("unable to add custom configMap to statefulSet for rack %s in cluster %s: %v. Other racks will not be updated", rack.Name, o.cassandra.QualifiedName(), err)
			return
		}
	}
}

func (o *AddCustomConfigOperation) String() string {
	return fmt.Sprintf("add custom config for cluster %s", o.cassandra.QualifiedName())
}
