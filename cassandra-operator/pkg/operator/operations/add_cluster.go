package operations

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
)

// AddClusterOperation describes what the operator does when creating a cluster
type AddClusterOperation struct {
	clusterAccessor     cluster.Accessor
	statefulSetAccessor *statefulSetAccessor
	cassandra           *v1alpha1.Cassandra
}

// Execute performs the operation
func (o *AddClusterOperation) Execute() (bool, error) {
	log.Infof("New Cassandra cluster definition added: %s.%s", o.cassandra.Namespace, o.cassandra.Name)
	configMap := o.clusterAccessor.FindCustomConfigMap(o.cassandra.Namespace, o.cassandra.Name)
	if configMap != nil {
		log.Infof("Found custom config map for cluster %s.%s", o.cassandra.Namespace, o.cassandra.Name)
	}

	c := cluster.New(o.cassandra)
	err := o.statefulSetAccessor.registerStatefulSets(c, configMap)
	if err == cluster.ErrReconciliationInterrupted {
		return true, nil
	}

	if err != nil {
		return false, fmt.Errorf("error while creating stateful sets for cluster %s: %v", c.QualifiedName(), err)
	}
	return false, nil
}

func (o *AddClusterOperation) String() string {
	return fmt.Sprintf("add cluster %s", o.cassandra.QualifiedName())
}
