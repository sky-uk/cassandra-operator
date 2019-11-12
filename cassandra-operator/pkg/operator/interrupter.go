package operator

import (
	"context"
	"fmt"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sync"
)

// Interrupter is a controller whose sole purpose is to check whether any subsequent changes have been made to a cluster
// which is currently under modification by the Reconciler, and interrupt the Reconciler so that the newer changes can
// be effected.
//
// It works by using a map of channels, keyed on a namespace-qualified cluster names, which is shared with the polling
// loop which waits for stateful set provisioning to complete. When the interrupter closes the channel for a cluster,
// the polling loop will exit early with a return value that specifies the polling was interrupted, rather than having
// completed successfully.
type Interrupter struct {
	activeReconciliations *sync.Map
	client                client.Client
	objectFactory         objectReferenceFactory
	eventRecorder         record.EventRecorder
}

// NewInterrupter creates a new Interrupter
func NewInterrupter(activeReconciliations *sync.Map, client client.Client, recorder record.EventRecorder) *Interrupter {
	return &Interrupter{
		activeReconciliations: activeReconciliations,
		client:                client,
		objectFactory:         &defaultReferenceFactory{},
		eventRecorder:         recorder,
	}
}

// Reconcile checks whether the definition of the cluster referenced in the reconcile.Request has been modified, and
// if so, will interrupt any ongoing reconciliation that cluster.
func (i *Interrupter) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := newContext(request)
	logger := ctx.logger

	clusterName := fmt.Sprintf("%s.%s", request.Namespace, request.Name)

	targetCassandra := i.objectFactory.newCassandra()
	err := i.client.Get(context.TODO(), request.NamespacedName, targetCassandra)
	if unexpectedError(err) {
		return retryReconciliation("unable to fetch Cassandra definition", err)
	}

	targetConfigMap := i.objectFactory.newConfigMap()
	err = i.client.Get(context.TODO(), types.NamespacedName{Namespace: request.Namespace, Name: fmt.Sprintf("%s-config", request.Name)}, targetConfigMap)
	if unexpectedError(err) {
		return retryReconciliation("unable to fetch Cassandra custom config definition", err)
	}

	if activeReconciliation, ok := i.activeReconciliations.Load(clusterName); ok {
		currentCassandraVersion := activeReconciliation.(*cluster.ActiveReconciliation).CassandraRevision
		currentConfigMapVersion := activeReconciliation.(*cluster.ActiveReconciliation).ConfigMapRevision

		logger.Debugf(
			"Cassandra versions: [%s:%s], configmap versions: [%s:%s]",
			currentCassandraVersion,
			targetCassandra.ResourceVersion,
			currentConfigMapVersion,
			targetConfigMap.ResourceVersion,
		)

		if currentCassandraVersion != targetCassandra.ResourceVersion || currentConfigMapVersion != targetConfigMap.ResourceVersion {
			logger.Info("Cluster is currently being reconciled, will interrupt as the cluster definition has been changed")
			i.eventRecorder.Eventf(
				targetCassandra,
				v1.EventTypeNormal,
				cluster.ReconciliationInterruptedEvent,
				"Reconciliation interrupted for cluster %s",
				clusterName,
			)
			activeReconciliation.(*cluster.ActiveReconciliation).Interrupt()
		}
	}

	return completeReconciliation()
}
