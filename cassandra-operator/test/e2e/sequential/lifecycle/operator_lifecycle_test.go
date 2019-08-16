package lifecycle

import (
	"encoding/json"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/operator"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test"
	. "github.com/sky-uk/cassandra-operator/cassandra-operator/test/e2e"
	"io"
	"io/ioutil"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		AClusterWithName(clusterName).AndRacks([]v1alpha1.Rack{RackWithEmptyDir("a", 1)}).UsingEmptyDir().Exists()
	})

	JustAfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			PrintDiagnosis(Namespace, testStartTime, clusterName)
		}
	})

	AfterEach(func() {
		DeleteCassandraResourcesForClusters(Namespace, clusterName)
	})

	It("should detect a cluster exists if pods are the only resources which still exist for it", func() {
		// given
		serviceDoesNotExistFor(clusterName)
		statefulSetsDoNotExistFor(clusterName)

		// when
		theOperatorIsRestarted()

		// then
		metricsAreReportedForCluster(clusterName)

		// and
		Expect(HeadlessServiceForCluster(Namespace, clusterName)()).To(BeNil())
		Expect(StatefulSetsForCluster(Namespace, clusterName)()).To(BeNil())
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

func metricsAreReportedForCluster(clusterName string) {
	Eventually(OperatorMetrics(Namespace), NodeStartDuration, CheckInterval).Should(ReportAClusterWith([]MetricAssertion{
		ClusterSizeMetric(Namespace, clusterName, 1),
		LiveAndNormalNodeMetric(Namespace, clusterName, PodName(clusterName, "a", 0), "a", 1),
	}))
}

func theOperatorIsRestarted() {
	operatorPodNameBeforeRestart := operatorPodName()()

	err := KubeClientset.CoreV1().Pods(Namespace).Delete(operatorPodNameBeforeRestart, &metaV1.DeleteOptions{})
	Expect(err).ToNot(HaveOccurred())

	Eventually(PodExists(Namespace, operatorPodNameBeforeRestart), NodeTerminationDuration, CheckInterval).ShouldNot(BeTrue())
	Eventually(operatorPodName(), NodeStartDuration, CheckInterval).ShouldNot(BeEmpty())
	operatorPodNameAfterRestart := operatorPodName()()
	Eventually(PodReadinessStatus(Namespace, operatorPodNameAfterRestart), NodeStartDuration, CheckInterval).Should(BeTrue())
}

func operatorPodName() func() string {
	return func() string {
		operatorListOptions := metaV1.ListOptions{LabelSelector: "app=cassandra-operator"}
		operatorPods, err := KubeClientset.CoreV1().Pods(Namespace).List(operatorListOptions)
		Expect(err).ToNot(HaveOccurred())
		Expect(operatorPods.Items).To(HaveLen(1))

		return operatorPods.Items[0].Name
	}
}

func serviceDoesNotExistFor(clusterName string) {
	err := KubeClientset.CoreV1().Services(Namespace).Delete(clusterName, &metaV1.DeleteOptions{})
	Expect(err).ToNot(HaveOccurred())
}

func statefulSetsDoNotExistFor(clusterName string) {
	listOptions := metaV1.ListOptions{LabelSelector: fmt.Sprintf("sky.uk/cassandra-operator=%s", clusterName)}
	orphanDependencies := metaV1.DeletePropagationOrphan
	deleteOptions := &metaV1.DeleteOptions{PropagationPolicy: &orphanDependencies}
	err := KubeClientset.AppsV1beta1().StatefulSets(Namespace).DeleteCollection(deleteOptions, listOptions)
	Expect(err).ToNot(HaveOccurred())
	Eventually(StatefulSetsForCluster(Namespace, clusterName), time.Minute, CheckInterval).Should(BeEmpty())
}
