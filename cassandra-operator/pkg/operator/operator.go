package operator

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/client/clientset/versioned"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/dispatcher"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/metrics"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/operator/operations"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	kscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"net/http"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"strings"
	"time"
)

// The Operator itself.
type Operator struct {
	clusters      map[types.NamespacedName]*v1alpha1.Cassandra
	config        *Config
	eventReceiver *operations.Receiver
	manager       manager.Manager
	controller    controller.Controller
	stopCh        chan struct{}
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

// New creates a new Operator.
func New(kubeClientset *kubernetes.Clientset, cassandraClientset *versioned.Clientset, operatorConfig *Config, kubeConfig *rest.Config) (*Operator, error) {
	clusters := make(map[types.NamespacedName]*v1alpha1.Cassandra)
	stopCh := make(chan struct{})
	scheme := registerScheme()

	metricsPoller := metrics.NewMetrics(kubeClientset.CoreV1(), &metrics.Config{RequestTimeout: operatorConfig.MetricRequestDuration})
	eventRecorder := cluster.NewEventRecorder(kubeClientset, scheme)
	clusterAccessor := cluster.NewAccessor(kubeClientset, cassandraClientset, eventRecorder)
	receiver := operations.NewEventReceiver(
		clusterAccessor,
		metricsPoller,
		eventRecorder,
	)
	eventDispatcher := dispatcher.New(receiver.Receive, stopCh)

	mgr, err := manager.New(kubeConfig, manager.Options{SyncPeriod: &operatorConfig.ControllerSyncPeriod, Scheme: scheme, Namespace: operatorConfig.Namespace})
	if err != nil {
		return nil, fmt.Errorf("unable to set up overall controller manager: %v", err)
	}

	ctrl, err := controller.New("cassandra", mgr, controller.Options{
		Reconciler: NewReconciler(clusters, mgr.GetClient(), eventRecorder, eventDispatcher, operatorConfig),
	})
	if err != nil {
		return nil, fmt.Errorf("unable to set up Cassandra reconciler controller: %v", err)
	}

	return &Operator{
		config:        operatorConfig,
		clusters:      clusters,
		eventReceiver: receiver,
		controller:    ctrl,
		manager:       mgr,
		stopCh:        stopCh,
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

	// Setup the Controller
	entryLog.Info("Setting up controller")

	// Watch Cassandras and enqueue Cassandra object key
	if err := o.controller.Watch(&source.Kind{Type: &v1alpha1.Cassandra{}}, &handler.EnqueueRequestForObject{}); err != nil {
		entryLog.Fatalf("Unable to watch Cassandras: %v", err)
	}

	// Watch StatefulSets and enqueue owning Cassandra key
	enqueueRequestsForObjectsOwnedByCassandra := &handler.EnqueueRequestForOwner{OwnerType: &v1alpha1.Cassandra{}, IsController: true}
	if err := o.controller.Watch(&source.Kind{Type: &appsv1.StatefulSet{}}, enqueueRequestsForObjectsOwnedByCassandra); err != nil {
		entryLog.Fatalf("Unable to watch StatefulSets: %v", err)
	}

	// Watch CronJobs and enqueue owning Cassandra key
	if err := o.controller.Watch(&source.Kind{Type: &v1beta1.CronJob{}}, enqueueRequestsForObjectsOwnedByCassandra); err != nil {
		entryLog.Fatalf("Unable to watch CronJobs: %v", err)
	}

	// Watch Service and enqueue owning Cassandra key
	if err := o.controller.Watch(&source.Kind{Type: &corev1.Service{}}, enqueueRequestsForObjectsOwnedByCassandra); err != nil {
		entryLog.Fatalf("Unable to watch Services: %v", err)
	}

	// Watch ConfigMaps and enqueue a likely cluster
	err := o.controller.Watch(
		&source.Kind{Type: &corev1.ConfigMap{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
				if !strings.HasSuffix(a.Meta.GetName(), "-config") {
					return []reconcile.Request{}
				}

				clusterName := strings.TrimSuffix(a.Meta.GetName(), "-config")

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

	entryLog.Info("Starting manager")
	if err := o.manager.Start(signals.SetupSignalHandler()); err != nil {
		entryLog.Fatalf("Unable to run manager: %v", err)
	}

	<-o.stopCh
	entryLog.Info("Operator shutting down")
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
				o.eventReceiver.Receive(&dispatcher.Event{Kind: operations.GatherMetrics, Key: c.QualifiedName(), Data: c})
			}
			time.Sleep(o.config.MetricPollInterval)
		}
	}()
}

func livenessAndReadinessCheck(resp http.ResponseWriter, _ *http.Request) {
	resp.WriteHeader(http.StatusNoContent)
}
