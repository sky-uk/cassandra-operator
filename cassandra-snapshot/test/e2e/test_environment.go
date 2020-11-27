package e2e

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc" // required for connectivity into dev cluster
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"os/exec"
	"time"
)

const (
	NodeStartDuration       = 120 * time.Second
	NodeTerminationDuration = 60 * time.Second
)

var (
	KubeClientset           *kubernetes.Clientset
	UseMockedImage          bool
	CassandraImageName      string
	CassandraReadinessProbe *v1.Probe
	RenameSnapshotCmd       string
	kubeContext             string
	kubeconfigLocation      string
	ImageUnderTest          string
	Namespace               string
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
		kubeconfigLocation = fmt.Sprintf("%s/.kube/config", os.Getenv("HOME"))
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

	ImageUnderTest = os.Getenv("IMAGE_UNDER_TEST")
	if ImageUnderTest == "" {
		panic("IMAGE_UNDER_TEST must be supplied")
	}

	UseMockedImage = os.Getenv("USE_MOCK") == "true"
	if UseMockedImage {
		CassandraImageName = os.Getenv("FAKE_CASSANDRA_IMAGE")
		if CassandraImageName == "" {
			panic("FAKE_CASSANDRA_IMAGE must be supplied")
		}

		CassandraReadinessProbe = &v1.Probe{
			Handler: v1.Handler{
				Exec: &v1.ExecAction{
					Command: []string{"curl", "localhost:7070"},
				},
			},
		}
		RenameSnapshotCmd = "sed -i \"s/^\\w\\+ /%s /g\" /tmp/snapshots"
	} else {
		CassandraImageName = "cassandra:3.11"
		CassandraReadinessProbe = &v1.Probe{
			Handler: v1.Handler{
				Exec: &v1.ExecAction{
					Command: []string{"cqlsh", "-e", "select * from system_auth.roles"},
				},
			},
		}
		RenameSnapshotCmd = "find /var/lib/cassandra/data/ -type d -path \"*/snapshots/*\" | xargs -I {} sh -c 'snapshot_name={}; snapshot_dir=$(dirname $snapshot_name); mv $snapshot_name $snapshot_dir/%s' \\;"
	}

	Namespace = os.Getenv("NAMESPACE")
	if Namespace == "" {
		Namespace = "test-cassandra-operator"
	}

	log.Infof("Running tests using Cassandra image: %s in namespace: %s", CassandraImageName, Namespace)
}

func Kubectl(namespace, podName string, command ...string) (*exec.Cmd, []byte, error) {
	argList := []string{
		fmt.Sprintf("--kubeconfig=%s", kubeconfigLocation),
		fmt.Sprintf("--context=%s", kubeContext),
		fmt.Sprintf("--namespace=%s", namespace),
		"exec",
		podName,
	}

	for _, word := range command {
		argList = append(argList, word)
	}

	cmd := exec.Command("kubectl", argList...)
	output, err := cmd.CombinedOutput()
	return cmd, output, err
}
