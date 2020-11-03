package operations

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/metrics"
)

// DeleteClusterOperation describes what the operator does when deleting a cluster
type DeleteClusterOperation struct {
	cassandra       *v1alpha1.Cassandra
	metricsReporter metrics.ClusterMetricsReporter
}

// Execute performs the operation
func (o *DeleteClusterOperation) Execute() (bool, error) {
	log.Infof("Cassandra cluster definition deleted for cluster: %s.%s", o.cassandra.Namespace, o.cassandra.Name)

	o.metricsReporter.DeleteMetrics(o.cassandra)
	return false, nil
}

func (o *DeleteClusterOperation) String() string {
	return fmt.Sprintf("delete cluster %s", o.cassandra.QualifiedName())
}
