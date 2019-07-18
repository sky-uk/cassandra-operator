package main

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	v1alpha1helpers "github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1/helpers"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1/validation"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/dispatcher"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/operator/operations"
)

type reconcileCassandra struct {
	// client can be used to retrieve objects from the APIServer.
	client client.Client
	log    logr.Logger

	eventRecorder record.EventRecorder

	eventDispatcher    dispatcher.Dispatcher
	previousCassandras map[string]*v1alpha1.Cassandra
	previousConfigMaps map[string]*corev1.ConfigMap
}

// Implement reconcile.Reconciler so the controller can reconcile objects
var _ reconcile.Reconciler = &reconcileCassandra{}

func (r *reconcileCassandra) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	fmt.Println(request)

	// set up a convinient log object so we don't have to type request over and over again
	log := r.log.WithValues("request", request)

	clusterID := fmt.Sprintf("%s.%s", request.Namespace, request.Name)

	cass := &v1alpha1.Cassandra{}
	err := r.client.Get(context.TODO(), request.NamespacedName, cass)
	if errors.IsNotFound(err) {
		log.Error(nil, "Could not find Cassandra")

		deletedCass := &v1alpha1.Cassandra{ObjectMeta: metav1.ObjectMeta{Name: request.Name, Namespace: request.Namespace}}
		r.eventDispatcher.Dispatch(&dispatcher.Event{Kind: operations.DeleteCluster, Key: clusterID, Data: deletedCass})
		delete(r.previousCassandras, request.NamespacedName.String())
		return reconcile.Result{}, nil
	}

	if err != nil {
		log.Error(err, "Could not fetch Cassandra")
		return reconcile.Result{}, err
	}

	v1alpha1helpers.SetDefaultsForCassandra(cass)
	err = validation.ValidateCassandra(cass).ToAggregate()
	if err != nil {
		log.Error(err, "validation error")
		return reconcile.Result{}, err
	}

	newConfigmap := &corev1.ConfigMap{}
	configMapNamespacedName := types.NamespacedName{Name: cass.CustomConfigMapName(), Namespace: request.Namespace}
	err = r.client.Get(context.TODO(), configMapNamespacedName, newConfigmap)
	if errors.IsNotFound(err) {
		log.Info("Could not find ConfigMap")

		oldConfigmap, ok := r.previousConfigMaps[configMapNamespacedName.String()]
		if ok {
			r.eventDispatcher.Dispatch(&dispatcher.Event{Kind: operations.DeleteCustomConfig, Key: clusterID, Data: oldConfigmap})
			delete(r.previousConfigMaps, configMapNamespacedName.String())
		}
	} else if err != nil {
		log.Error(err, "Could not fetch ConfigMap")
		return reconcile.Result{}, err
	} else {
		log.Info("got a new configmap", "configMap", newConfigmap)

		if oldConfigmap, ok := r.previousConfigMaps[configMapNamespacedName.String()]; ok {
			// we've seen this before, check if it's updated
			if !reflect.DeepEqual(oldConfigmap.Data, newConfigmap.Data) {
				r.eventDispatcher.Dispatch(&dispatcher.Event{Kind: operations.UpdateCustomConfig, Key: clusterID, Data: newConfigmap})
			}
		} else {
			// we've not seen this before, add it
			r.previousConfigMaps[configMapNamespacedName.String()] = newConfigmap.DeepCopy()
			r.eventDispatcher.Dispatch(&dispatcher.Event{Kind: operations.AddCustomConfig, Key: clusterID, Data: newConfigmap})
		}
	}

	if cass.Annotations == nil {
		cass.Annotations = map[string]string{}
	}
	_, ok := cass.Annotations["reconciled.cassandra.core.sky.uk"]
	if !ok {
		// cassandra has not been created
		r.eventDispatcher.Dispatch(&dispatcher.Event{Kind: operations.AddCluster, Key: clusterID, Data: cass})
		cass.Annotations["reconciled.cassandra.core.sky.uk"] = "true"
	} else {
		previousCassandra, ok := r.previousCassandras[request.NamespacedName.String()]
		if !ok {
			return reconcile.Result{}, fmt.Errorf("couldn't find a previousCassandra")
		}
		err = validation.ValidateCassandraUpdate(previousCassandra, cass).ToAggregate()
		if err != nil {
			r.eventRecorder.Event(previousCassandra, corev1.EventTypeWarning, cluster.InvalidChangeEvent, err.Error())
		}
		r.eventDispatcher.Dispatch(&dispatcher.Event{
			Kind: operations.UpdateCluster,
			Key:  clusterID,
			Data: operations.ClusterUpdate{OldCluster: previousCassandra, NewCluster: cass},
		})
	}

	err = r.client.Update(context.TODO(), cass)
	if err != nil {
		log.Error(err, "Could not write Cassandra")
		return reconcile.Result{}, err
	}

	r.previousCassandras[request.NamespacedName.String()] = cass.DeepCopy()

	return reconcile.Result{}, nil
}
