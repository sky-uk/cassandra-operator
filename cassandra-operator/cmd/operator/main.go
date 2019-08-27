package main

import (
	"fmt"
	"k8s.io/api/batch/v1beta1"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	logr "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	kscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/client/clientset/versioned"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/dispatcher"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/metrics"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/operator/operations"
)

var (
	metricPollInterval   time.Duration
	metricRequestTimeout time.Duration
	controllerSyncPeriod time.Duration
	logLevel             string
	clusters             map[types.NamespacedName]*v1alpha1.Cassandra
	receiver             *operations.Receiver

	rootCmd = &cobra.Command{
		Use:               "cassandra-operator",
		Short:             "Operator for provisioning Cassandra clusters.",
		PersistentPreRunE: handleArgs,
		RunE:              startOperator,
	}

	scheme = runtime.NewScheme()
)

func init() {
	err := v1alpha1.AddToScheme(scheme)
	if err != nil {
		panic(err)
	}
	err = kscheme.AddToScheme(scheme)
	if err != nil {
		panic(err)
	}

	clusters = make(map[types.NamespacedName]*v1alpha1.Cassandra)

	rootCmd.PersistentFlags().DurationVar(&metricPollInterval, "metric-poll-interval", 5*time.Second, "Poll interval between cassandra nodes metrics retrieval")
	rootCmd.PersistentFlags().DurationVar(&metricRequestTimeout, "metric-request-timeout", 2*time.Second, "Time limit for cassandra node metrics requests")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "should be one of: debug, info, warn, error, fatal, panic")
	rootCmd.PersistentFlags().DurationVar(&controllerSyncPeriod, "controller-sync-period", 5*time.Minute, "Minimum frequency at which watched resources are reconciled")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func handleArgs(_ *cobra.Command, _ []string) error {
	var isPositive = func(duration time.Duration) bool {
		currentTime := time.Now()
		return currentTime.Add(duration).After(currentTime)
	}
	var isNegative = func(duration time.Duration) bool {
		currentTime := time.Now()
		return currentTime.Add(duration).Before(currentTime)
	}

	if !isPositive(metricPollInterval) {
		return fmt.Errorf("invalid metric-poll-interval, it must be a positive duration")
	}

	if isNegative(controllerSyncPeriod) {
		return fmt.Errorf("invalid controller-sync-period, it must not be a negative duration")
	}

	level, err := logr.ParseLevel(logLevel)
	if err != nil {
		return fmt.Errorf("invalid log-level")
	}
	logr.SetLevel(level)

	return nil
}

func startOperator(_ *cobra.Command, _ []string) error {
	entryLog := logr.WithFields(
		logr.Fields{
			"logger": "main.go",
		},
	)

	kubeConfig := config.GetConfigOrDie()

	kubeClientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		entryLog.Fatalf("Unable to obtain clientset: %v", err)
	}

	cassandraClientset := versioned.NewForConfigOrDie(kubeConfig)

	ns := os.Getenv("OPERATOR_NAMESPACE")
	if ns == "" {
		entryLog.Info("Operator listening for changes in any namespace")
	} else {
		entryLog.Infof("Operator listening for changes in namespace: %s", ns)
	}

	// Setup a Manager
	entryLog.Info("Setting up manager")
	mgr, err := manager.New(kubeConfig, manager.Options{SyncPeriod: &controllerSyncPeriod, Scheme: scheme, Namespace: ns})
	if err != nil {
		entryLog.Fatalf("Unable to set up overall controller manager: %v", err)
	}

	eventRecorder := cluster.NewEventRecorder(kubeClientset, scheme)
	clusterAccessor := cluster.NewAccessor(kubeClientset, cassandraClientset, eventRecorder)

	metricsPoller := metrics.NewMetrics(kubeClientset.CoreV1(), &metrics.Config{RequestTimeout: metricPollInterval})
	startServer(metricsPoller)

	receiver = operations.NewEventReceiver(
		clusterAccessor,
		metricsPoller,
		eventRecorder,
	)

	stopCh := make(chan struct{})

	// Setup a new controller to reconcile ReplicaSets
	entryLog.Info("Setting up controller")
	c, err := controller.New("cassandra", mgr, controller.Options{
		Reconciler: NewReconciler(clusters, mgr.GetClient(), eventRecorder, dispatcher.New(receiver.Receive, stopCh)),
	})
	if err != nil {
		entryLog.Fatalf("Unable to set up Cassandra reconciler controller: %v", err)
	}

	// Watch Cassandras and enqueue Cassandra object key
	if err := c.Watch(&source.Kind{Type: &v1alpha1.Cassandra{}}, &handler.EnqueueRequestForObject{}); err != nil {
		entryLog.Fatalf("Unable to watch Cassandras: %v", err)
	}

	// Watch StatefulSets and enqueue owning Cassandra key
	enqueueRequestsForObjectsOwnedByCassandra := &handler.EnqueueRequestForOwner{OwnerType: &v1alpha1.Cassandra{}, IsController: true}
	if err := c.Watch(&source.Kind{Type: &appsv1.StatefulSet{}}, enqueueRequestsForObjectsOwnedByCassandra); err != nil {
		entryLog.Fatalf("Unable to watch StatefulSets: %v", err)
	}

	// Watch CronJobs and enqueue owning Cassandra key
	if err := c.Watch(&source.Kind{Type: &v1beta1.CronJob{}}, enqueueRequestsForObjectsOwnedByCassandra); err != nil {
		entryLog.Fatalf("Unable to watch CronJobs: %v", err)
	}

	// Watch Service and enqueue owning Cassandra key
	if err := c.Watch(&source.Kind{Type: &corev1.Service{}}, enqueueRequestsForObjectsOwnedByCassandra); err != nil {
		entryLog.Fatalf("Unable to watch Services: %v", err)
	}

	// Watch ConfigMaps and enqueue a likely cluster
	err = c.Watch(
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
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		entryLog.Fatalf("Unable to run manager: %v", err)
	}

	return nil
}

func startServer(metricsPoller *metrics.PrometheusMetrics) {
	entryLog := logr.WithField("name", "metrics")

	startMetricPolling(metricsPoller)
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

func startMetricPolling(metricsPoller *metrics.PrometheusMetrics) {
	go func() {
		for {
			for _, c := range clusters {
				receiver.Receive(&dispatcher.Event{Kind: operations.GatherMetrics, Key: c.QualifiedName(), Data: c})
			}
			time.Sleep(metricPollInterval)
		}
	}()
}

func livenessAndReadinessCheck(resp http.ResponseWriter, _ *http.Request) {
	resp.WriteHeader(http.StatusNoContent)
}
