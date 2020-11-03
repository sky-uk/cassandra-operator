package operator

import (
	"context"
	"fmt"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/client/clientset/versioned"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/metrics"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/operator/event"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	kscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"net/http"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"strings"
	"sync"
	"time"
)

// The Operator itself.
type Operator struct {
	clusters            map[types.NamespacedName]*v1alpha1.Cassandra
	config              *Config
	eventReceiver       event.Receiver
	manager             manager.Manager
	reconcileController controller.Controller
	interruptController controller.Controller
	stopCh              chan struct{}
}

// The Config for the Operator
type Config struct {
	MetricPollInterval    time.Duration
	MetricRequestDuration time.Duration
	ControllerSyncPeriod  time.Duration
	Namespace             string
	Version               string
	RepositoryPath        string
}

const maxConcurrentReconciles = 100

// New creates a new Operator.
func New(kubeClientset *kubernetes.Clientset, cassandraClientset *versioned.Clientset, operatorConfig *Config, kubeConfig *rest.Config) (*Operator, error) {
	clusters := make(map[types.NamespacedName]*v1alpha1.Cassandra)
	stopCh := make(chan struct{})
	scheme := registerScheme()
	var activeReconciliations sync.Map

	metricsReporter := metrics.NewMetrics(kubeClientset.CoreV1(), &metrics.Config{RequestTimeout: operatorConfig.MetricRequestDuration})
	eventRecorder := cluster.NewEventRecorder(kubeClientset, scheme)
	clusterAccessor := cluster.NewAccessor(kubeClientset, cassandraClientset, eventRecorder, &activeReconciliations)
	eventReceiver := event.NewEventReceiver(
		clusterAccessor,
		metricsReporter,
		eventRecorder,
	)

	mgr, err := manager.New(kubeConfig, manager.Options{SyncPeriod: &operatorConfig.ControllerSyncPeriod, Scheme: scheme, Namespace: operatorConfig.Namespace})
	if err != nil {
		return nil, fmt.Errorf("unable to set up overall controller manager: %v", err)
	}

	reconciler, err := controller.New("cassandra", mgr, controller.Options{
		Reconciler:              NewReconciler(clusters, mgr.GetClient(), eventRecorder, eventReceiver, operatorConfig, metricsReporter),
		MaxConcurrentReconciles: maxConcurrentReconciles,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to set up Cassandra reconciler reconcileController: %v", err)
	}

	interrupter, err := controller.New("interrupter", mgr, controller.Options{
		Reconciler:              NewInterrupter(&activeReconciliations, mgr.GetClient(), eventRecorder),
		MaxConcurrentReconciles: maxConcurrentReconciles,
	})

	return &Operator{
		config:              operatorConfig,
		clusters:            clusters,
		eventReceiver:       eventReceiver,
		reconcileController: reconciler,
		interruptController: interrupter,
		manager:             mgr,
		stopCh:              stopCh,
	}, nil
}

func registerScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	if err != nil {
		panic(err)
	}
	err = kscheme.AddToScheme(scheme)
	if err != nil {
		panic(err)
	}
	return scheme
}

// Run starts the Operator.
func (o *Operator) Run() {
	entryLog := log.WithField("logger", "operator.go")

	if o.config.Namespace == "" {
		entryLog.Info("Operator listening for changes in any namespace")
	} else {
		entryLog.Infof("Operator listening for changes in namespace: %s", o.config.Namespace)
	}

	entryLog.Info("Starting server")
	o.startServer()

	entryLog.Info("Setting up ReconcileController")
	o.setupController(o.reconcileController, o.manager.GetClient(), entryLog)

	entryLog.Info("Setting up InterruptController")
	o.setupController(o.interruptController, o.manager.GetClient(), entryLog)

	entryLog.Info("Starting manager")
	if err := o.manager.Start(signals.SetupSignalHandler()); err != nil {
		entryLog.Fatalf("Unable to run manager: %v", err)
	}

	<-o.stopCh
	entryLog.Info("Operator shutting down")
}

func (o *Operator) setupController(controller controller.Controller, client client.Client, entryLog *log.Entry) {
	// Watch Cassandras and enqueue Cassandra object key
	if err := controller.Watch(&source.Kind{Type: &v1alpha1.Cassandra{}}, &handler.EnqueueRequestForObject{}); err != nil {
		entryLog.Fatalf("Unable to watch Cassandras: %v", err)
	}

	// Watch StatefulSets and enqueue owning Cassandra key
	enqueueRequestsForObjectsOwnedByCassandra := &handler.EnqueueRequestForOwner{OwnerType: &v1alpha1.Cassandra{}, IsController: true}
	if err := controller.Watch(&source.Kind{Type: &appsv1.StatefulSet{}}, enqueueRequestsForObjectsOwnedByCassandra); err != nil {
		entryLog.Fatalf("Unable to watch StatefulSets: %v", err)
	}

	// Watch CronJobs and enqueue owning Cassandra key
	if err := controller.Watch(&source.Kind{Type: &v1beta1.CronJob{}}, enqueueRequestsForObjectsOwnedByCassandra); err != nil {
		entryLog.Fatalf("Unable to watch CronJobs: %v", err)
	}

	// Watch Service and enqueue owning Cassandra key
	if err := controller.Watch(&source.Kind{Type: &corev1.Service{}}, enqueueRequestsForObjectsOwnedByCassandra); err != nil {
		entryLog.Fatalf("Unable to watch Services: %v", err)
	}

	// Watch ConfigMaps and enqueue the corresponding cluster
	err := controller.Watch(
		&source.Kind{Type: &corev1.ConfigMap{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
				if !strings.HasSuffix(a.Meta.GetName(), "-config") {
					// nothing to do: not looking like a ConfigMap for a Cassandra cluster
					return []reconcile.Request{}
				}

				clusterName := strings.TrimSuffix(a.Meta.GetName(), "-config")
				err := client.Get(context.TODO(), types.NamespacedName{Namespace: a.Meta.GetNamespace(), Name: clusterName}, &v1alpha1.Cassandra{})
				// attempt to reconcile on unknown error
				if errors.IsNotFound(err) {
					// nothing to do: no associated Cassandra definition found
					return []reconcile.Request{}
				}

				return []reconcile.Request{
					{NamespacedName: types.NamespacedName{
						Name:      clusterName,
						Namespace: a.Meta.GetNamespace(),
					}},
				}
			}),
		},
	)
	if err != nil {
		entryLog.Fatalf("Unable to watch ConfigMaps: %v", err)
	}
}

func (o *Operator) startServer() {
	entryLog := log.WithField("logger", "operator.go")

	o.startMetricPolling()
	statusCheck := newStatusCheck()
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/live", livenessAndReadinessCheck)
	http.HandleFunc("/ready", livenessAndReadinessCheck)
	http.HandleFunc("/status", statusCheck.statusPage)
	go func() {
		entryLog.Error(http.ListenAndServe(":9090", nil), "metrics")
		os.Exit(0)
	}()
}

func (o *Operator) startMetricPolling() {
	go func() {
		for {
			for _, c := range o.clusters {
				// Use the receiver directly as this is a regular event that should not be queued behind other operations
				o.eventReceiver.Receive(&event.Event{Kind: event.GatherMetrics, Key: c.QualifiedName(), Data: c})
			}
			time.Sleep(o.config.MetricPollInterval)
		}
	}()
}

func livenessAndReadinessCheck(resp http.ResponseWriter, _ *http.Request) {
	resp.WriteHeader(http.StatusNoContent)
}
