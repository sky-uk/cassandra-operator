package main

import (
	"fmt"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/operator"
	"os"
	"time"

	logr "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"

	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/client/clientset/versioned"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var (
	metricPollInterval   time.Duration
	metricRequestTimeout time.Duration
	controllerSyncPeriod time.Duration
	logLevel             string

	rootCmd = &cobra.Command{
		Use:               "cassandra-operator",
		Short:             "Operator for provisioning Cassandra clusters.",
		PersistentPreRunE: handleArgs,
		RunE:              startOperator,
	}
)

func init() {
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
	entryLog := logr.WithField("logger", "main.go")

	kubeConfig, err := config.GetConfig()
	if err != nil {
		entryLog.Fatalf("Unable to get config: %v", err)
	}

	kubeClientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		entryLog.Fatalf("Unable to obtain clientset: %v", err)
	}

	cassandraClientset := versioned.NewForConfigOrDie(kubeConfig)

	operatorConfig := &operator.Config{
		MetricRequestDuration: metricPollInterval,
		MetricPollInterval:    metricPollInterval,
		ControllerSyncPeriod:  controllerSyncPeriod,
		Namespace:             os.Getenv("OPERATOR_NAMESPACE"),
	}

	entryLog.Infof("Starting Cassandra operator with config: %+v", operatorConfig)
	op, err := operator.New(kubeClientset, cassandraClientset, operatorConfig, kubeConfig)
	if err != nil {
		entryLog.Fatalf("Unable to create a new Operator: %v", err)
	}

	op.Run()

	return nil
}
