package lifecycle

import (
	"encoding/json"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/operator"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test"
	. "github.com/sky-uk/cassandra-operator/cassandra-operator/test/e2e"
	"io"
	"io/ioutil"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"net/http"
	"testing"
	"time"
)

var (
	testStartTime time.Time
)

func TestSequential(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "E2E Suite (Lifecycle Tests)", test.CreateSequentialReporters("e2e_lifecycle"))
}

var _ = SequentialTestBeforeSuite(func() {})

var _ = Context("When an operator is restarted", func() {
	var clusterName string

	BeforeEach(func() {
		testStartTime = time.Now()
		clusterName = AClusterName()
	})

	JustAfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			PrintDiagnosis(Namespace, testStartTime, clusterName)
		}
	})

	AfterEach(func() {
		DeleteCassandraResourcesForClusters(Namespace, clusterName)
	})

	It("should detect a cluster exists", func() {
		// given
		AClusterWithName(clusterName).AndRacks([]v1alpha1.Rack{RackWithEmptyDir("a", 1)}).Exists()

		// when
		theOperatorIsRestarted()

		// then
		By("exposing metrics for the cluster")
		Eventually(OperatorMetrics(Namespace), NodeStartDuration, CheckInterval).Should(ReportAClusterWith([]MetricAssertion{
			ClusterSizeMetric(Namespace, clusterName, 1),
			LiveAndNormalNodeMetric(Namespace, clusterName, PodName(clusterName, "a", 0), "a", 1),
		}))
	})

	It("should apply both cluster and custom config changes made while the operator was down", func() {
		// given
		AClusterWithName(clusterName).AndRacks([]v1alpha1.Rack{Rack("a", 1)}).Exists()

		// when
		theOperatorIsScaledDown()
		TheRackReplicationIsChangedTo(Namespace, clusterName, "a", 2)
		TheCustomConfigIsDeletedForCluster(Namespace, clusterName)
		theOperatorIsScaledUp()

		// then
		By("creating a new pod within the cluster rack")
		Eventually(RacksForCluster(Namespace, clusterName), NodeStartDuration, CheckInterval).Should(And(
			HaveLen(1),
			HaveKeyWithValue("a", []string{PodName(clusterName, "a", 0), PodName(clusterName, "a", 1)}),
		))

		By("applying the config changes to each pod")
		Eventually(PodsForCluster(Namespace, clusterName), 2*NodeRestartDuration, CheckInterval).Should(Each(And(
			Not(HaveVolumeForConfigMap(fmt.Sprintf("%s-config", clusterName))),
			Not(HaveAnnotation("clusterConfigHash")),
		)))
		Eventually(PodReadinessStatus(Namespace, PodName(clusterName, "a", 0)), NodeRestartDuration, CheckInterval).Should(BeTrue())
		Eventually(PodReadinessStatus(Namespace, PodName(clusterName, "a", 1)), NodeRestartDuration, CheckInterval).Should(BeTrue())

		By("exposing metrics for the cluster")
		Eventually(OperatorMetrics(Namespace), NodeStartDuration, CheckInterval).Should(ReportAClusterWith([]MetricAssertion{
			ClusterSizeMetric(Namespace, clusterName, 2),
			LiveAndNormalNodeMetric(Namespace, clusterName, PodName(clusterName, "a", 0), "a", 1),
			LiveAndNormalNodeMetric(Namespace, clusterName, PodName(clusterName, "a", 1), "a", 1),
		}))
	})
})

var _ = Context("Operator probes and status", func() {

	Specify("its liveness probe should report OK", func() {
		Expect(theLivenessProbeStatus()).To(Equal(http.StatusNoContent))
	})

	Specify("its readiness probe should report OK", func() {
		Expect(theReadinessProbeStatus()).To(Equal(http.StatusNoContent))
	})

	Specify("its status page reports the version of the Cassandra crd used", func() {
		statusPage, err := theStatusPage()
		Expect(err).To(Not(HaveOccurred()))
		Expect(statusPage.CassandraCrdVersion).To(Equal(cassandra.Version))
	})
})

func theLivenessProbeStatus() (int, error) {
	return probeStatus("live")
}

func theReadinessProbeStatus() (int, error) {
	return probeStatus("ready")
}

func probeStatus(path string) (int, error) {

	response := KubeClientset.CoreV1().Services(Namespace).
		ProxyGet("", "cassandra-operator", "http", path, map[string]string{}).(*rest.Request).
		Do()

	if response.Error() != nil {
		return 0, response.Error()
	}
	var statusCode int
	response.StatusCode(&statusCode)
	return statusCode, nil
}

func theStatusPage() (*operator.Status, error) {
	status := &operator.Status{}

	resp, err := KubeClientset.CoreV1().Services(Namespace).
		ProxyGet("", "cassandra-operator", "http", "status", map[string]string{}).
		Stream()
	if err != nil {
		return nil, err
	}
	err = readJSONResponseInto(resp, status)
	if err != nil {
		return &operator.Status{}, err
	}
	return status, nil
}

func readJSONResponseInto(reader io.Reader, responseHolder interface{}) error {

	bodyAsBytes, err := ioutil.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("error while parsing response body, %v", err)
	}

	if len(bodyAsBytes) > 0 {
		if err := json.Unmarshal(bodyAsBytes, responseHolder); err != nil {
			return fmt.Errorf("error while unmarshalling response. Body %s, %v", string(bodyAsBytes), err)
		}
	}
	return nil
}

func theOperatorIsScaledDown() {
	scaleOperator(0)
}

func theOperatorIsScaledUp() {
	scaleOperator(1)
}

func theOperatorIsRestarted() {
	theOperatorIsScaledDown()
	theOperatorIsScaledUp()
}

func scaleOperator(replicas int32) {
	_, err := KubeClientset.AppsV1().Deployments(Namespace).UpdateScale("cassandra-operator", &autoscalingv1.Scale{
		Spec:       autoscalingv1.ScaleSpec{Replicas: replicas},
		ObjectMeta: metav1.ObjectMeta{Name: "cassandra-operator", Namespace: Namespace},
	})
	Expect(err).ToNot(HaveOccurred())
	Eventually(theDeploymentScale(Namespace, "cassandra-operator"), 30*time.Second, CheckInterval).Should(Equal(replicas))
	log.Infof("Scaled operator to %d replica", replicas)
}

func theDeploymentScale(namespace, deploymentName string) func() (int32, error) {
	return func() (int32, error) {
		scale, err := KubeClientset.AppsV1().Deployments(namespace).GetScale(deploymentName, metav1.GetOptions{})
		if err != nil {
			return 0, err
		}
		return scale.Status.Replicas, nil
	}
}
