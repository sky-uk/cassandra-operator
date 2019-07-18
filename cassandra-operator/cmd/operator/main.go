package main

import (
	"fmt"
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
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
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
	logLevel             string
	allowEmptyDir        bool
	clusters             map[string]*cluster.Cluster
	receiver             *operations.Receiver

	rootCmd = &cobra.Command{
		Use:               "cassandra-operator",
		Short:             "Operator for provisioning Cassandra clusters.",
		PersistentPreRunE: handleArgs,
		RunE:              startOperator,
	}

	scheme = runtime.NewScheme()
	log    = logf.Log.WithName("cassandra-operator")
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

	clusters = make(map[string]*cluster.Cluster)

	rootCmd.PersistentFlags().DurationVar(&metricPollInterval, "metric-poll-interval", 5*time.Second, "Poll interval between cassandra nodes metrics retrieval")
	rootCmd.PersistentFlags().DurationVar(&metricRequestTimeout, "metric-request-timeout", 2*time.Second, "Time limit for cassandra node metrics requests")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "should be one of: debug, info, warn, error, fatal, panic")
	rootCmd.PersistentFlags().BoolVar(&allowEmptyDir, "allow-empty-dir", false, "Set to true in order to allow creation of clusters which use emptyDir storage")
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

	if !isPositive(metricPollInterval) {
		return fmt.Errorf("invalid metric-poll-interval, it must be a positive integer")
	}

	_, err := logr.ParseLevel(logLevel)
	if err != nil {
		return fmt.Errorf("invalid log-level")
	}

	return nil
}

func startOperator(_ *cobra.Command, _ []string) error {
	logf.SetLogger(zap.Logger(false))
	entryLog := log.WithName("entrypoint")

	kubeConfig := config.GetConfigOrDie()

	kubeClientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		entryLog.Error(err, "Unable to obtain clientset")
		os.Exit(1)
	}

	cassandraClientset := versioned.NewForConfigOrDie(kubeConfig)

	ns := os.Getenv("OPERATOR_NAMESPACE")
	if ns == "" {
		entryLog.Info("Operator listening for changes in any namespace")
	} else {
		entryLog.Info("Operator listening for changes in namespace", "namespace", ns)
	}

	// Setup a Manager
	entryLog.Info("setting up manager")
	mgr, err := manager.New(kubeConfig, manager.Options{Scheme: scheme, Namespace: ns})
	if err != nil {
		entryLog.Error(err, "unable to set up overall controller manager")
		os.Exit(1)
	}

	eventRecorder := cluster.NewEventRecorder(kubeClientset, scheme)
	clusterAccessor := cluster.NewAccessor(kubeClientset, cassandraClientset, eventRecorder)

	metricsPoller := metrics.NewMetrics(kubeClientset.CoreV1(), &metrics.Config{RequestTimeout: metricPollInterval})
	startServer(metricsPoller)

	receiver = operations.NewEventReceiver(
		clusters,
		clusterAccessor,
		metricsPoller,
		eventRecorder,
	)

	stopCh := make(chan struct{})

	// Setup a new controller to reconcile ReplicaSets
	entryLog.Info("Setting up controller")
	c, err := controller.New("cassandra", mgr, controller.Options{
		Reconciler: &reconcileCassandra{
			client:             mgr.GetClient(),
			log:                log.WithName("reconciler"),
			eventDispatcher:    dispatcher.New(receiver.Receive, stopCh),
			eventRecorder:      eventRecorder,
			previousCassandras: map[string]*v1alpha1.Cassandra{},
			previousConfigMaps: map[string]*corev1.ConfigMap{},
		},
	})
	if err != nil {
		entryLog.Error(err, "unable to set up individual controller")
		os.Exit(1)
	}

	// Watch Cassandras and enqueue Cassandra object key
	if err := c.Watch(&source.Kind{Type: &v1alpha1.Cassandra{}}, &handler.EnqueueRequestForObject{}); err != nil {
		entryLog.Error(err, "unable to watch Cassandras")
		os.Exit(1)
	}

	// Watch StatefulSets and enqueue owning Cassandra key
	if err := c.Watch(&source.Kind{Type: &appsv1.StatefulSet{}},
		&handler.EnqueueRequestForOwner{OwnerType: &v1alpha1.Cassandra{}, IsController: true}); err != nil {
		entryLog.Error(err, "unable to watch StatefulSets")
		os.Exit(1)
	}

	// Watch Pods and enqueue owning Cassandra key
	if err := c.Watch(&source.Kind{Type: &corev1.Pod{}},
		&handler.EnqueueRequestForOwner{OwnerType: &v1alpha1.Cassandra{}, IsController: true}); err != nil {
		entryLog.Error(err, "unable to watch Pods")
		os.Exit(1)
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
		})

	if err != nil {
		entryLog.Error(err, "unable to watch ConfigMaps")
		os.Exit(1)
	}

	entryLog.Info("starting manager")
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		entryLog.Error(err, "unable to run manager")
		os.Exit(1)
	}

	return nil
}

func startServer(metricsPoller *metrics.PrometheusMetrics) {
	entryLog := log.WithName("metrics")

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
	entryLog := log.WithName("metrics")
	go func() {
		for {
			for _, c := range clusters {
				if c.Online {
					entryLog.Info("Sending request for metrics", "cluster", c.QualifiedName())
					receiver.Receive(&dispatcher.Event{Kind: operations.GatherMetrics, Key: c.QualifiedName(), Data: c})
				}
			}
			time.Sleep(metricPollInterval)
		}
	}()
}

func livenessAndReadinessCheck(resp http.ResponseWriter, _ *http.Request) {
	resp.WriteHeader(http.StatusNoContent)
}
