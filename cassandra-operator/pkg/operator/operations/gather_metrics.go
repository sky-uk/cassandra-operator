package operations

import (
	"fmt"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/metrics"
)

// GatherMetricsOperation describes what the operator does when gathering metrics
type GatherMetricsOperation struct {
	metricsPoller *metrics.PrometheusMetrics
	cassandra     *v1alpha1.Cassandra
}

// Execute performs the operation
func (o *GatherMetricsOperation) Execute() (bool, error) {
	o.metricsPoller.UpdateMetrics(cluster.New(o.cassandra))
	return false, nil
}

func (o *GatherMetricsOperation) String() string {
	return fmt.Sprintf("gather metrics for cluster %s", o.cassandra.QualifiedName())
}
