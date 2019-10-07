package operations

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
)

// AddServiceOperation describes what the operator does when creating a cluster
type AddServiceOperation struct {
	clusterAccessor cluster.Accessor
	cassandra       *v1alpha1.Cassandra
}

// Execute performs the operation
func (o *AddServiceOperation) Execute() error {
	c := cluster.New(o.cassandra)

	_, err := o.clusterAccessor.CreateServiceForCluster(c)
	if err != nil {
		return fmt.Errorf("error while creating headless service for cluster %s: %v", c.QualifiedName(), err)
	}
	log.Infof("Headless service created for cluster : %s", c.QualifiedName())
	return nil
}

func (o *AddServiceOperation) String() string {
	return fmt.Sprintf("add service %s", o.cassandra.QualifiedName())
}
