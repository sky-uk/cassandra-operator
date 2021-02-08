package modification

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test"
	. "github.com/sky-uk/cassandra-operator/cassandra-operator/test/e2e"
)

var (
	multipleNodeCluster *TestCluster
	testStartTime       time.Time
)

func TestInvalidModification(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "E2E Suite (Invalid Modification Tests)", test.CreateSequentialReporters("e2e_invalid_modification"))
}

func defineCluster(multipleNodeClusterName string) *TestCluster {
	return &TestCluster{
		Name:  multipleNodeClusterName,
		Racks: []v1alpha1.Rack{RackWithEmptyDir("a", 2), RackWithEmptyDir("b", 1)},
	}
}

func createCluster(multipleNodeCluster *TestCluster) {
	AClusterWithName(multipleNodeCluster.Name).AndRacks(multipleNodeCluster.Racks).Exists()
}

var _ = SequentialTestBeforeSuite(func() {
	multipleNodeCluster = defineCluster(AClusterName())
	createCluster(multipleNodeCluster)
})

var _ = Context("forbidden cluster modifications", func() {

	var podEvents *PodEventLog
	var podWatcher watch.Interface
	var expectedFailedValidations int

	BeforeEach(func() {
		testStartTime = time.Now()
		podEvents, podWatcher = WatchPodEvents(Namespace, multipleNodeCluster.Name)
		currentFailedValidations, err := GetMetric(OperatorMetrics(Namespace), "cassandra_failed_validation_total")
		Expect(err).Should(BeNil())
		expectedFailedValidations = 1 + currentFailedValidations

	})

	JustAfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			PrintDiagnosis(Namespace, testStartTime, multipleNodeCluster.Name)
		}
	})

	AfterEach(func() {
		podWatcher.Stop()
	})

	It("should not allow the number of pods per rack to be scaled down as this is unsupported", func() {
		// when
		modificationTime := time.Now()
		TheRackReplicationIsChangedTo(Namespace, multipleNodeCluster.Name, "a", 1)

		// then
		By("recording a warning that scale-down operations are not supported")
		Eventually(CassandraEventsFor(Namespace, multipleNodeCluster.Name), EventPublicationTimeout, CheckInterval).Should(HaveEvent(EventExpectation{
			Type:                 coreV1.EventTypeWarning,
			Reason:               cluster.InvalidChangeEvent,
			Message:              "spec.Racks.a.Replicas: Forbidden: This field can not be decremented",
			LastTimestampCloseTo: &modificationTime,
		}))

		By("incrementing the failed validation metric for this cluster")
		Eventually(OperatorMetrics(Namespace), 60*time.Second, CheckInterval).Should(ReportAClusterWith([]MetricAssertion{
			FailedValidationMetric(Namespace, multipleNodeCluster.Name, expectedFailedValidations),
		}))

		By("not changing the number of pods in the rack")
		Expect(RacksForCluster(Namespace, multipleNodeCluster.Name)()).Should(And(
			HaveLen(2),
			HaveKeyWithValue("a", []string{PodName(multipleNodeCluster.Name, "a", 0), PodName(multipleNodeCluster.Name, "a", 1)}),
			HaveKeyWithValue("b", []string{PodName(multipleNodeCluster.Name, "b", 0)}),
		))
	})

	It("should reject any change where any other property is modified", func() {
		// when
		modificationTime := time.Now()
		TheImageImmutablePropertyIsChangedTo(Namespace, multipleNodeCluster.Name, "another-image")

		// then
		By("recording a warning event about the forbidden change")
		Eventually(CassandraEventsFor(Namespace, multipleNodeCluster.Name), EventPublicationTimeout, CheckInterval).Should(HaveEvent(EventExpectation{
			Type:                 coreV1.EventTypeWarning,
			Reason:               cluster.InvalidChangeEvent,
			Message:              "spec.Pod.Image: Forbidden: This field can not be changed",
			LastTimestampCloseTo: &modificationTime,
		}))
		By("incrementing the failed validation metric for this cluster")
		Eventually(OperatorMetrics(Namespace), 60*time.Second, CheckInterval).Should(ReportAClusterWith([]MetricAssertion{
			FailedValidationMetric(Namespace, multipleNodeCluster.Name, expectedFailedValidations),
		}))
		By("not restarting any pods")
		Expect(podEvents.PodsStartedEventCount(PodName(multipleNodeCluster.Name, "a", 0))).To(Equal(1))
	})

	It("should reject deletion of any racks as unsupported", func() {
		// when
		modificationTime := time.Now()
		ARackIsRemovedFromCluster(Namespace, multipleNodeCluster.Name, "b")

		// then
		By("recording a warning event about the forbidden rack deletion")
		Eventually(CassandraEventsFor(Namespace, multipleNodeCluster.Name), EventPublicationTimeout, CheckInterval).Should(HaveEvent(EventExpectation{
			Type:                 coreV1.EventTypeWarning,
			Reason:               cluster.InvalidChangeEvent,
			Message:              "spec.Racks: Forbidden: Rack deletion is not supported",
			LastTimestampCloseTo: &modificationTime,
		}))
		By("incrementing the failed validation metric for this cluster")
		Eventually(OperatorMetrics(Namespace), 60*time.Second, CheckInterval).Should(ReportAClusterWith([]MetricAssertion{
			FailedValidationMetric(Namespace, multipleNodeCluster.Name, expectedFailedValidations),
		}))
	})

	It("should reject any env changes with reserved variable names", func() {
		// when
		modificationTime := time.Now()
		modifiedEnvVars := []v1alpha1.CassEnvVar{
			v1alpha1.CassEnvVar{Name: "EXTRA_CLASSPATH", Value: "some_value"}, // Purposely setting a reserved variable name
		}
		TheClusterPodEnvVarsAreChangedTo(Namespace, multipleNodeCluster.Name, &modifiedEnvVars)

		// then
		By("recording a warning event about the forbidden change")

		Eventually(CassandraEventsFor(Namespace, multipleNodeCluster.Name), EventPublicationTimeout, CheckInterval).Should(HaveEvent(EventExpectation{
			Type:                 coreV1.EventTypeWarning,
			Reason:               cluster.InvalidChangeEvent,
			Message:              "spec.Pod.Env: Forbidden: spec.Pod.env cannot contain reserved variable with Name: EXTRA_CLASSPATH",
			LastTimestampCloseTo: &modificationTime,
		}))
		By("incrementing the failed validation metric for this cluster")
		Eventually(OperatorMetrics(Namespace), 60*time.Second, CheckInterval).Should(ReportAClusterWith([]MetricAssertion{
			FailedValidationMetric(Namespace, multipleNodeCluster.Name, expectedFailedValidations),
		}))
		By("not restarting any pods")
		Expect(podEvents.PodsStartedEventCount(PodName(multipleNodeCluster.Name, "a", 0))).To(Equal(1))
	})
})
