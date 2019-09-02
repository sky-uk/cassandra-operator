package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc" // required for connectivity into dev cluster
	"k8s.io/client-go/tools/clientcmd"

	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/client/clientset/versioned"
)

const (
	CheckInterval = 5 * time.Second
	// max number of 1Gi mem nodes that can fit within the namespace resource quota
	MaxCassandraNodesPerNamespace = 6
)

var (
	KubeClientset                           *kubernetes.Clientset
	kubeconfigLocation                      string
	CassandraClientset                      *versioned.Clientset
	kubeContext                             string
	UseMockedImage                          bool
	CassandraImageName                      string
	CassandraBootstrapperImageName          string
	CassandraSidecarImageName               string
	CassandraSnapshotImageName              string
	CassandraInitialDelay                   int32
	CassandraLivenessPeriod                 int32
	CassandraLivenessProbeFailureThreshold  int32
	CassandraReadinessPeriod                int32
	CassandraReadinessProbeFailureThreshold int32
	NodeStartDuration                       time.Duration
	NodeRestartDuration                     time.Duration
	NodeTerminationDuration                 time.Duration
	Namespace                               string
)

func init() {
	kubeContext = os.Getenv("KUBE_CONTEXT")
	if kubeContext == "ignore" {
		// This option is provided to allow the test code to be built without running any tests.
		return
	}

	if kubeContext == "" {
		kubeContext = "kind"
	}

	kubeconfigLocation = os.Getenv("KUBECONFIG")
	if kubeconfigLocation == "" {
		kubeconfigLocation = fmt.Sprintf("%s/.kube/kind-config-kind", os.Getenv("HOME"))
	}

	var err error
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{Precedence: []string{kubeconfigLocation}},
		&clientcmd.ConfigOverrides{CurrentContext: kubeContext},
	).ClientConfig()

	if err != nil {
		log.Fatalf("Unable to obtain out-of-cluster config: %v", err)
	}

	KubeClientset = kubernetes.NewForConfigOrDie(config)
	CassandraClientset = versioned.NewForConfigOrDie(config)

	UseMockedImage = os.Getenv("USE_MOCK") == "true"
	podStartTimeoutEnvValue := os.Getenv("POD_START_TIMEOUT")
	if podStartTimeoutEnvValue == "" {
		// long time needed because volumes have been seen to take several minutes to attach
		podStartTimeoutEnvValue = "5m"
	}

	if UseMockedImage {
		CassandraImageName = os.Getenv("FAKE_CASSANDRA_IMAGE")
		if CassandraImageName == "" {
			panic("FAKE_CASSANDRA_IMAGE must be supplied")
		}
		CassandraInitialDelay = 1
		CassandraLivenessPeriod = 1
		CassandraReadinessPeriod = 1
		CassandraLivenessProbeFailureThreshold = 3
		CassandraReadinessProbeFailureThreshold = 3
	} else {
		CassandraImageName = v1alpha1.DefaultCassandraImage
		CassandraInitialDelay = 30
		CassandraLivenessPeriod = 30
		CassandraReadinessPeriod = 15
		CassandraLivenessProbeFailureThreshold = 4  // allow 2mins
		CassandraReadinessProbeFailureThreshold = 8 // allow 2mins
	}

	NodeStartDuration, err = time.ParseDuration(podStartTimeoutEnvValue)
	if err != nil {
		panic(fmt.Sprintf("Invalid pod start timeout specified %v", err))
	}

	NodeTerminationDuration = NodeStartDuration
	NodeRestartDuration = NodeStartDuration * 2
	CassandraBootstrapperImageName = getEnvOrDefault("CASSANDRA_BOOTSTRAPPER_IMAGE", v1alpha1.DefaultCassandraBootstrapperImage)
	CassandraSidecarImageName = getEnvOrDefault("CASSANDRA_SIDECAR_IMAGE", v1alpha1.DefaultCassandraSidecarImage)
	CassandraSnapshotImageName = getEnvOrDefault("CASSANDRA_SNAPSHOT_IMAGE", v1alpha1.DefaultCassandraSnapshotImage)

	Namespace = os.Getenv("NAMESPACE")
	if Namespace == "" {
		Namespace = "test-cassandra-operator"
	}

	log.Infof(
		"Running tests with Kubernetes context: %s, namespace: %s, Cassandra image: %s, bootstrapper image: %s, snapshot image: %s, sidecar image: %s, node start duration: %s",
		kubeContext,
		Namespace,
		CassandraImageName,
		CassandraBootstrapperImageName,
		CassandraSnapshotImageName,
		CassandraSidecarImageName,
		NodeStartDuration,
	)
}

func KubectlOutputAsString(namespace string, args ...string) string {
	command, outputBytes, err := Kubectl(namespace, args...)
	if err != nil {
		return fmt.Sprintf("command was %v.\nOutput was:\n%s\n. Error: %v", command, outputBytes, err)
	}
	return strings.TrimSpace(string(outputBytes))
}

func Kubectl(namespace string, args ...string) (*exec.Cmd, []byte, error) {
	argList := []string{
		fmt.Sprintf("--kubeconfig=%s", kubeconfigLocation),
		fmt.Sprintf("--context=%s", kubeContext),
		fmt.Sprintf("--namespace=%s", namespace),
	}

	for _, word := range args {
		argList = append(argList, word)
	}

	cmd := exec.Command("kubectl", argList...)
	output, err := cmd.CombinedOutput()
	return cmd, output, err
}

func getEnvOrDefault(envKey, defaultValue string) string {
	envValue := os.Getenv(envKey)
	if envValue != "" {
		return envValue
	}
	return defaultValue
}
