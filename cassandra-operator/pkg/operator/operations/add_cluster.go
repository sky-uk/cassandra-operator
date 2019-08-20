package operations

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"strings"
)

// AddClusterOperation describes what the operator does when creating a cluster
type AddClusterOperation struct {
	clusterAccessor     *cluster.Accessor
	statefulSetAccessor *statefulSetAccessor
	cassandra           *v1alpha1.Cassandra
}

// Execute performs the operation
func (o *AddClusterOperation) Execute() {
	log.Infof("New Cassandra cluster definition added: %s.%s", o.cassandra.Namespace, o.cassandra.Name)
	configMap := o.clusterAccessor.FindCustomConfigMap(o.cassandra.Namespace, o.cassandra.Name)
	if configMap != nil {
		log.Infof("Found custom config map for cluster %s.%s", o.cassandra.Namespace, o.cassandra.Name)
	}

	c := cluster.New(o.cassandra)

	foundResources := o.clusterAccessor.FindExistingResourcesFor(c)
	if len(foundResources) > 0 {
		log.Infof("Resources already found for cluster %s, not attempting to recreate: %s", c.QualifiedName(), strings.Join(foundResources, ","))
	} else {
		_, err := o.clusterAccessor.CreateServiceForCluster(c)
		if err != nil {
			log.Errorf("Error while creating headless service for cluster %s: %v", c.QualifiedName(), err)
			return
		}
		log.Infof("Headless service created for cluster : %s", c.QualifiedName())

		err = o.statefulSetAccessor.registerStatefulSets(c, configMap)
		if err != nil {
			log.Errorf("Error while creating stateful sets for cluster %s: %v", c.QualifiedName(), err)
			return
		}
	}
}

func (o *AddClusterOperation) String() string {
	return fmt.Sprintf("add cluster %s", o.cassandra.QualifiedName())
}
