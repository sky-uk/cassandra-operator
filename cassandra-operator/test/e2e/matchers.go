package e2e

import (
	"fmt"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/ptr"
	"math"
	"reflect"
	"strings"
	"time"

	"github.com/onsi/gomega/types"
	"k8s.io/api/apps/v1beta2"
	batch "k8s.io/api/batch/v1beta1"
	coreV1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

//
// Each matcher
//
func Each(subMatcher types.GomegaMatcher) types.GomegaMatcher {
	return &each{subMatcher, ""}
}

type each struct {
	embeddedMatcher               types.GomegaMatcher
	embeddedMatcherFailureMessage string
}

func (matcher *each) Match(actual interface{}) (bool, error) {
	arr := reflect.ValueOf(actual)

	if arr.Kind() != reflect.Slice {
		fmt.Printf("expected %v, got %v", reflect.Slice, arr.Kind())
		return false, fmt.Errorf("expected %v, got %v", reflect.Slice, arr.Kind())
	}

	if arr.Len() == 0 {
		fmt.Printf("zero-length slice")
		return false, fmt.Errorf("zero-length slice")
	}

	for i := 0; i < arr.Len(); i++ {
		actualElement := arr.Index(i).Interface()
		success, err := matcher.embeddedMatcher.Match(actualElement)
		if !success || err != nil {
			matcher.embeddedMatcherFailureMessage = matcher.embeddedMatcher.FailureMessage(actualElement)
			return success, err
		}
	}

	return true, nil
}

func (matcher *each) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Embedded matcher %v failed with message: %s", reflect.ValueOf(matcher.embeddedMatcher).Type(), matcher.embeddedMatcherFailureMessage)
}

func (matcher *each) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Embedded matcher %v failed with message: %s", reflect.ValueOf(matcher.embeddedMatcher).Type(), matcher.embeddedMatcherFailureMessage)
}

//
// BeCreatedWithServiceName matcher
//
func BeCreatedWithServiceName(expected string) types.GomegaMatcher {
	return &statefulSetShouldBeCreated{expected}
}

type statefulSetShouldBeCreated struct {
	serviceName string
}

func (matcher *statefulSetShouldBeCreated) Match(actual interface{}) (success bool, err error) {
	lr := actual.(*kubernetesResource)
	statefulSet := lr.Resource.(v1beta2.StatefulSet)

	if statefulSet.Spec.ServiceName != matcher.serviceName {
		return false, fmt.Errorf("expected service name to be %s, found %s", matcher.serviceName, statefulSet.Spec.ServiceName)
	}

	return true, nil
}

func (matcher *statefulSetShouldBeCreated) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected service name %s, Actual service name: %s", matcher.serviceName, actual)
}

func (matcher *statefulSetShouldBeCreated) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected service name %s, Actual service name: %s", matcher.serviceName, actual)
}

//
// HaveLabel matcher
//
func HaveLabel(key, value string) types.GomegaMatcher {
	return &haveLabel{key: key, value: value}
}

type haveLabel struct {
	key   string
	value string
}

func (matcher *haveLabel) Match(actual interface{}) (success bool, err error) {
	lr := actual.(*kubernetesResource)
	value, ok := lr.Labels()[matcher.key]
	return ok && value == matcher.value, nil
}

func (matcher *haveLabel) FailureMessage(actual interface{}) (message string) {
	lr := actual.(*kubernetesResource)
	return fmt.Sprintf("Expected label key: %s, value: %s. Actual labels: %s", matcher.key, matcher.value, lr.Labels())
}

func (matcher *haveLabel) NegatedFailureMessage(actual interface{}) (message string) {
	lr := actual.(*kubernetesResource)
	return fmt.Sprintf("Expected label key: %s, value: %s. Actual labels: %s", matcher.key, matcher.value, lr.Labels())
}

// HaveEvent matcher
func HaveEvent(expected EventExpectation) types.GomegaMatcher {
	return &haveEvent{expected}
}

type EventExpectation struct {
	Type                 string
	Reason               string
	Message              string
	LastTimestampCloseTo *time.Time
}

type haveEvent struct {
	expected EventExpectation
}

func (matcher *haveEvent) Match(actual interface{}) (success bool, err error) {
	var matchFound bool
	for _, event := range actual.([]coreV1.Event) {
		if event.Type == matcher.expected.Type &&
			event.Reason == string(matcher.expected.Reason) &&
			(event.Message == "" || strings.Contains(event.Message, matcher.expected.Message)) &&
			(matcher.expected.LastTimestampCloseTo == nil || matcher.lastTimestampIsCloseTo(event.LastTimestamp)) {
			matchFound = true
		}
	}
	return matchFound, nil
}

func (matcher *haveEvent) lastTimestampIsCloseTo(eventTimestamp v1.Time) bool {
	return math.Abs(eventTimestamp.Sub(*matcher.expected.LastTimestampCloseTo).Seconds()) < 120
}

func (matcher *haveEvent) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected event %+v. Actual event: %+v", matcher.expected, actual)
}

func (matcher *haveEvent) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected event %+v. Actual event: %+v", matcher.expected, actual)
}

//
// HaveInitContainer matcher
//
func HaveInitContainer(initContainerName string, expectation ContainerSpecExpectation) types.GomegaMatcher {
	return &haveContainer{
		containerName: initContainerName,
		expectation:   expectation,
		containers: func(p coreV1.Pod) []coreV1.Container {
			return p.Spec.InitContainers
		},
	}
}

//
// HaveContainer matcher
//
func HaveContainer(containerName string, expectation ContainerSpecExpectation) types.GomegaMatcher {
	return &haveContainer{
		containerName: containerName,
		expectation:   expectation,
		containers: func(p coreV1.Pod) []coreV1.Container {
			return p.Spec.Containers
		},
	}
}

type ContainerSpecExpectation interface {
	matchContainerSpec(container coreV1.Container) (bool, error)
}

type ContainerImageExpectation struct {
	ImageName string
}

type ContainerExpectation struct {
	ImageName                      string
	ContainerPorts                 map[string]int
	MemoryRequest                  string
	MemoryLimit                    string
	CPURequest                     string
	LivenessProbeTimeout           time.Duration
	LivenessProbeFailureThreshold  int32
	LivenessProbePeriod            time.Duration
	LivenessProbeInitialDelay      time.Duration
	ReadinessProbeTimeout          time.Duration
	ReadinessProbeFailureThreshold int32
	ReadinessProbeSuccessThreshold int32
	ReadinessProbePeriod           time.Duration
	ReadinessProbeInitialDelay     time.Duration
}

type haveContainer struct {
	containerName string
	expectation   ContainerSpecExpectation
	containers    func(coreV1.Pod) []coreV1.Container
}

func (matcher *haveContainer) Match(actual interface{}) (success bool, err error) {
	lr := actual.(*kubernetesResource)
	pod := lr.Resource.(coreV1.Pod)
	containers := matcher.containers(pod)
	var container coreV1.Container
	containerNames := sets.NewString()
	for _, c := range containers {
		containerName := c.Name
		containerNames.Insert(containerName)
		if containerName == matcher.containerName {
			container = c
			break
		}
	}

	if !containerNames.Has(matcher.containerName) {
		return false, fmt.Errorf("expected a container with name %s, found %v", matcher.containerName, containerNames)
	}

	result, err := matcher.expectation.matchContainerSpec(container)
	if err != nil {
		return result, fmt.Errorf("expectation error for container %s: %v", matcher.containerName, err)
	}
	return result, err
}

func (matcher *haveContainer) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Actual pod: %s", actual.(*kubernetesResource).Resource)
}

func (matcher *haveContainer) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Actual pod: %s", actual.(*kubernetesResource).Resource)
}

func (expectation *ContainerExpectation) matchContainerSpec(container coreV1.Container) (bool, error) {
	if !strings.Contains(container.Image, expectation.ImageName) {
		return false, fmt.Errorf("expected to use image %s, actual %s", expectation.ImageName, container.Image)
	}

	if len(container.Ports) != len(expectation.ContainerPorts) {
		return false, fmt.Errorf("expected number of container ports to be %d, actual %d", len(expectation.ContainerPorts), len(container.Ports))
	}

	if container.Resources.Requests.Memory().String() != expectation.MemoryRequest {
		return false, fmt.Errorf("expected memory request to be %s, actual %s", expectation.MemoryRequest, container.Resources.Requests.Memory().String())
	}

	if container.Resources.Limits.Memory().String() != expectation.MemoryLimit {
		return false, fmt.Errorf("expected memory limit to be %s, actual %s", expectation.MemoryLimit, container.Resources.Limits.Memory().String())
	}

	if container.Resources.Requests.Cpu().String() != expectation.CPURequest {
		return false, fmt.Errorf("expected cpu request to be %s, actual %s", expectation.CPURequest, container.Resources.Requests.Cpu().String())
	}

	if container.LivenessProbe.TimeoutSeconds != int32(expectation.LivenessProbeTimeout.Seconds()) {
		return false, fmt.Errorf("expected liveness timeout to be %d, actual %d", int32(expectation.LivenessProbeTimeout.Seconds()), container.LivenessProbe.TimeoutSeconds)
	}

	if container.LivenessProbe.FailureThreshold != expectation.LivenessProbeFailureThreshold {
		return false, fmt.Errorf("expected liveness failure threshold to be %d, actual %d", expectation.LivenessProbeFailureThreshold, container.LivenessProbe.FailureThreshold)
	}

	if container.LivenessProbe.SuccessThreshold != 1 {
		return false, fmt.Errorf("expected liveness success threshold to be 1, actual %d", container.LivenessProbe.SuccessThreshold)
	}

	if container.LivenessProbe.PeriodSeconds != int32(expectation.LivenessProbePeriod.Seconds()) {
		return false, fmt.Errorf("expected liveness period to be %d, actual %d", int32(expectation.LivenessProbePeriod.Seconds()), container.LivenessProbe.PeriodSeconds)
	}

	if container.LivenessProbe.InitialDelaySeconds != int32(expectation.LivenessProbeInitialDelay.Seconds()) {
		return false, fmt.Errorf("expected liveness initial delay to be %d, actual %d", int32(expectation.LivenessProbeInitialDelay.Seconds()), container.LivenessProbe.InitialDelaySeconds)
	}

	if container.ReadinessProbe.TimeoutSeconds != int32(expectation.ReadinessProbeTimeout.Seconds()) {
		return false, fmt.Errorf("expected readiness timeout to be %d, actual %d", int32(expectation.ReadinessProbeTimeout.Seconds()), container.ReadinessProbe.TimeoutSeconds)
	}

	if container.ReadinessProbe.FailureThreshold != expectation.ReadinessProbeFailureThreshold {
		return false, fmt.Errorf("expected readiness failure threshold to be %d, actual %d", expectation.ReadinessProbeFailureThreshold, container.ReadinessProbe.FailureThreshold)
	}

	if container.ReadinessProbe.SuccessThreshold != expectation.ReadinessProbeSuccessThreshold {
		return false, fmt.Errorf("expected readiness success threshold to be %d, actual %d", expectation.ReadinessProbeSuccessThreshold, container.ReadinessProbe.SuccessThreshold)
	}

	if container.ReadinessProbe.PeriodSeconds != int32(expectation.ReadinessProbePeriod.Seconds()) {
		return false, fmt.Errorf("expected readiness period to be %d, actual %d", int32(expectation.ReadinessProbePeriod.Seconds()), container.ReadinessProbe.PeriodSeconds)
	}

	if container.ReadinessProbe.InitialDelaySeconds != int32(expectation.ReadinessProbeInitialDelay.Seconds()) {
		return false, fmt.Errorf("expected readiness initial delay to be %d, actual %d", int32(expectation.ReadinessProbeInitialDelay.Seconds()), container.ReadinessProbe.InitialDelaySeconds)
	}

	for _, port := range container.Ports {
		expectedPort, ok := expectation.ContainerPorts[port.Name]
		if !ok {
			return false, fmt.Errorf("unexpected ports %s found", port.Name)
		}

		if port.ContainerPort != int32(expectedPort) {
			return false, fmt.Errorf("expected port %s to be %d, actual %d", port.Name, expectedPort, port.ContainerPort)
		}
	}

	return true, nil
}

func (expectation *ContainerImageExpectation) matchContainerSpec(container coreV1.Container) (bool, error) {
	if !strings.Contains(container.Image, expectation.ImageName) {
		return false, fmt.Errorf("expected to use image %s, actual %s", expectation.ImageName, container.Image)
	}
	return true, nil
}

//
// HaveDifferentRevisionTo matcher
//
func HaveDifferentRevisionTo(rackRevisions map[string]string) types.GomegaMatcher {
	return &haveDifferentRevision{rackRevisions: rackRevisions}
}

type haveDifferentRevision struct {
	rackRevisions map[string]string
}

func (m *haveDifferentRevision) Match(actual interface{}) (success bool, err error) {
	pod := m.pod(actual)
	unexpectedRevision := m.unexpectedRevisionForPod(pod)

	return unexpectedRevision != pod.Labels[v1beta2.StatefulSetRevisionLabel], nil
}

func (m *haveDifferentRevision) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected to be different revision to: %s. Actual revision: %s.", m.unexpectedRevisionForPod(m.pod(actual)), m.pod(actual).Labels[v1beta2.StatefulSetRevisionLabel])
}

func (m *haveDifferentRevision) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected to be same revision as: %s. Actual revision: %s.", m.unexpectedRevisionForPod(m.pod(actual)), m.pod(actual).Labels[v1beta2.StatefulSetRevisionLabel])
}

func (m *haveDifferentRevision) pod(actual interface{}) *coreV1.Pod {
	pod := actual.(*kubernetesResource).Resource.(coreV1.Pod)
	return &pod
}

func (m *haveDifferentRevision) unexpectedRevisionForPod(pod *coreV1.Pod) string {
	return m.rackRevisions[pod.Labels["rack"]]
}

//
// HaveStorageCapacity matcher
//
func HaveStorageCapacity(value string) types.GomegaMatcher {
	return &haveStorageCapacity{value: value}
}

type haveStorageCapacity struct {
	value string
}

func (matcher *haveStorageCapacity) Match(actual interface{}) (success bool, err error) {
	lr := actual.(*kubernetesResource)
	storage := lr.Resource.(coreV1.PersistentVolumeClaim).Spec.Resources.Requests[coreV1.ResourceStorage]
	return storage.String() == matcher.value, nil
}

func (matcher *haveStorageCapacity) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected capacity %s. Actual capacity: %s", matcher.value, actual)
}

func (matcher *haveStorageCapacity) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected capacity %s. Actual capacity: %s", matcher.value, actual)
}

//
// HaveEmptyDirVolumeMountAtPath
//
func HaveEmptyDirVolumeMountAtPath(path string) types.GomegaMatcher {
	return &haveEmptyDirVolumeMount{path: path, containerName: "cassandra"}
}

type haveEmptyDirVolumeMount struct {
	path          string
	containerName string
}

func (matcher *haveEmptyDirVolumeMount) Match(actual interface{}) (success bool, err error) {
	lr := actual.(*kubernetesResource)
	pod := lr.Resource.(coreV1.Pod)
	mount := matcher.findMount(pod)
	if mount == nil {
		return false, nil
	}

	volume := matcher.findVolume(mount.Name, pod)
	if volume == nil {
		return false, nil
	} else if volume.EmptyDir == nil {
		return false, nil
	}
	return true, nil
}

func (matcher *haveEmptyDirVolumeMount) findVolume(volumeName string, pod coreV1.Pod) *coreV1.Volume {
	for _, volume := range pod.Spec.Volumes {
		if volume.Name == volumeName {
			return &volume
		}
	}
	return nil
}

func (matcher *haveEmptyDirVolumeMount) findMount(pod coreV1.Pod) *coreV1.VolumeMount {
	for _, container := range pod.Spec.Containers {
		if container.Name == matcher.containerName {
			for _, mount := range container.VolumeMounts {
				if mount.MountPath == matcher.path {
					return &mount
				}
			}
		}
	}
	return nil
}

func (matcher *haveEmptyDirVolumeMount) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected emptyDir volume mounted at path %s. Actual: %s", matcher.path, actual)
}

func (matcher *haveEmptyDirVolumeMount) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Not expected emptyDir volume mounted at path %s. Actual: %s", matcher.path, actual)
}

//
// HavePersistentVolumeMountAtPath
//
func HavePersistentVolumeMountAtPath(path string) types.GomegaMatcher {
	return &havePersistentVolumeMountAtPath{path: path, containerName: "cassandra"}
}

type havePersistentVolumeMountAtPath struct {
	path          string
	containerName string
}

func (matcher *havePersistentVolumeMountAtPath) Match(actual interface{}) (success bool, err error) {
	lr := actual.(*kubernetesResource)
	ss := lr.Resource.(v1beta2.StatefulSet)
	mount := matcher.findMount(ss.Spec.Template.Spec.Containers)
	if mount == nil {
		return false, nil
	}
	if claim := matcher.findClaim(mount.Name, &ss); claim == nil {
		return false, nil
	}
	return true, nil
}

func (matcher *havePersistentVolumeMountAtPath) findMount(containers []coreV1.Container) *coreV1.VolumeMount {
	for _, container := range containers {
		if container.Name == matcher.containerName {
			for _, mount := range container.VolumeMounts {
				if mount.MountPath == matcher.path {
					return &mount
				}
			}
		}
	}
	return nil
}

func (matcher *havePersistentVolumeMountAtPath) findClaim(name string, statefulSet *v1beta2.StatefulSet) *coreV1.PersistentVolumeClaim {
	for _, volumeClaim := range statefulSet.Spec.VolumeClaimTemplates {
		if volumeClaim.Name == name {
			return &volumeClaim
		}
	}
	return nil
}

func (matcher *havePersistentVolumeMountAtPath) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected persistent volume mounted at path %s. Actual: %v", matcher.path, actual)
}

func (matcher *havePersistentVolumeMountAtPath) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Not expected persistent volume mounted at path %s. Actual: %v", matcher.path, actual)
}

//
// ReportAClusterWith matcher
//
func ReportAClusterWith(expected interface{}) types.GomegaMatcher {
	// Metric assertions are disabled in mock mode as the mock does not provide accurate stubs for Jolokia
	if UseMockedImage {
		return &reportClusterMetrics{assertions: []MetricAssertion{}}
	}
	return &reportClusterMetrics{assertions: expected.([]MetricAssertion)}
}

type MetricAssertion struct {
	metricToFind  string
	expectPresent bool
}

func (m MetricAssertion) String() string {
	if m.expectPresent {
		return fmt.Sprintf("'%s':present", m.metricToFind)
	}

	return fmt.Sprintf("'%s':absent", m.metricToFind)
}

type MetricsExpectation struct {
	MetricsToCheck func() []MetricAssertion
}

type reportClusterMetrics struct {
	assertions []MetricAssertion
}

func (m *reportClusterMetrics) Match(actual interface{}) (success bool, err error) {
	metricsAsString := actual.(string)
	for _, metricAssertion := range m.assertions {
		if strings.Contains(metricsAsString, metricAssertion.metricToFind) != metricAssertion.expectPresent {
			return false, nil
		}
	}
	return true, nil
}

func (m *reportClusterMetrics) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected: %v. \nActual response body: %s", m.assertions, actual)
}

func (m *reportClusterMetrics) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected: %v. \nActual response body: %s", m.assertions, actual)
}

// Metrics specifiers

func LiveAndNormalNodeMetric(namespace, clusterName, node string, rack string, value int) MetricAssertion {
	return presentMetrics(fmt.Sprintf("cassandra_node_status{cluster=\"%s\",liveness=\"up\",namespace=\"%s\",pod=\"%s\",rack=\"%s\",state=\"normal\"} %d", clusterName, namespace, node, rack, value))
}

func DownAndNormalNodeMetric(namespace, clusterName, node string, rack string, value int) MetricAssertion {
	return presentMetrics(fmt.Sprintf("cassandra_node_status{cluster=\"%s\",liveness=\"down\",namespace=\"%s\",pod=\"%s\",rack=\"%s\",state=\"normal\"} %d", clusterName, namespace, node, rack, value))
}

func ClusterSizeMetric(namespace, clusterName string, value uint) MetricAssertion {
	return presentMetrics(fmt.Sprintf("cassandra_cluster_size{cluster=\"%s\",namespace=\"%s\"} %d", clusterName, namespace, value))
}

func presentMetrics(metricToFind string) MetricAssertion {
	return MetricAssertion{metricToFind, true}
}

//
// ReportNoClusterMetricsFor matcher
//
func ReportNoClusterMetricsFor(namespace string, clusterName string) types.GomegaMatcher {
	return &noClusterMetrics{namespace: namespace, clusterName: clusterName}
}

type noClusterMetrics struct {
	namespace   string
	clusterName string
}

func (m *noClusterMetrics) Match(actual interface{}) (success bool, err error) {
	metricsAsString := actual.(string)
	if strings.Contains(metricsAsString, fmt.Sprintf("cluster=\"%s\",namespace=\"%s\"", m.clusterName, m.namespace)) {
		return false, nil
	}
	return true, nil
}

func (m *noClusterMetrics) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected no metrics for cluster %s.%s, but found some. \nActual response body: %s", m.namespace, m.clusterName, actual)
}

func (m *noClusterMetrics) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected no metrics for cluster %s.%s, but found some. \nActual response body: %s", m.namespace, m.clusterName, actual)
}

//
// HaveJVMArg matcher
//
func HaveJVMArg(expectedArg string) types.GomegaMatcher {
	return &haveJVMArg{expectedArg: expectedArg}
}

type haveJVMArg struct {
	expectedArg  string
	actualOutput string
}

func (m *haveJVMArg) Match(actual interface{}) (success bool, err error) {
	lr := actual.(*kubernetesResource)
	pod := lr.Resource.(coreV1.Pod)

	var commandToRun string

	if UseMockedImage {
		commandToRun = "cat /etc/cassandra/jvm.options"
	} else {
		commandToRun = "ps auxww | grep cassandra"
	}

	command, rawOutput, err := Kubectl(pod.Namespace, "exec", pod.Name, "--", "sh", "-c", commandToRun)

	if err != nil {
		return false, fmt.Errorf("command was %v.\nOutput of exec was:\n%s\n. Error: %v", command, rawOutput, err)
	}

	m.actualOutput = string(rawOutput)
	return strings.Contains(m.actualOutput, m.expectedArg), nil
}

func (m *haveJVMArg) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected JVM arg %s. Actual %s", m.expectedArg, m.actualOutput)
}

func (m *haveJVMArg) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected JVM arg %s. Actual %s", m.expectedArg, m.actualOutput)
}

//
// HaveVolumeForConfigMap matcher
//
func HaveVolumeForConfigMap(configMapName string) types.GomegaMatcher {
	return &configMapVolumeAssertion{configMapName}
}

type configMapVolumeAssertion struct {
	configMapName string
}

func (m *configMapVolumeAssertion) Match(actual interface{}) (success bool, err error) {
	lr := actual.(*kubernetesResource)
	pod := lr.Resource.(coreV1.Pod)
	for _, volume := range pod.Spec.Volumes {
		if volume.VolumeSource.ConfigMap != nil {
			return volume.VolumeSource.ConfigMap.LocalObjectReference.Name == m.configMapName, nil
		}
	}
	return false, nil
}

func (m *configMapVolumeAssertion) FailureMessage(actual interface{}) (message string) {
	lr := actual.(*kubernetesResource)
	pod := lr.Resource.(coreV1.Pod)
	return fmt.Sprintf("Expected a volume referencing the configMap with name: %s. Actual volumes: %v", m.configMapName, pod.Spec.Volumes)
}

func (m *configMapVolumeAssertion) NegatedFailureMessage(actual interface{}) (message string) {
	lr := actual.(*kubernetesResource)
	pod := lr.Resource.(coreV1.Pod)
	return fmt.Sprintf("Expected a volume referencing the configMap with name: %s not to exist. Actual volumes: %v", m.configMapName, pod.Spec.Volumes)
}

//
// HaveAnnotation matcher
//
func HaveAnnotation(expected interface{}) types.GomegaMatcher {
	return &haveAnnotation{assertion: expected.(string)}
}

type haveAnnotation struct {
	assertion string
}

func (m *haveAnnotation) Match(actual interface{}) (success bool, err error) {
	actualAnnotations := podAnnotations(actual)
	_, ok := actualAnnotations[m.assertion]
	return ok, nil
}

func (m *haveAnnotation) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected annotation: %s. \nActual annotations: %s", m.assertion, podAnnotationNames(actual))
}

func (m *haveAnnotation) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected annotation to be absent: %s. \nActual annotations: %s", m.assertion, podAnnotationNames(actual))
}

//
// HaveAnnotationValue matcher
//
func HaveAnnotationValue(expected interface{}) types.GomegaMatcher {
	return &haveAnnotationValue{assertion: expected.(AnnotationValueAssertion)}
}

type AnnotationValueAssertion struct {
	Name  string
	Value string
}

func (m AnnotationValueAssertion) String() string {
	return fmt.Sprintf("'%s' set to '%s'", m.Name, m.Value)
}

type haveAnnotationValue struct {
	assertion AnnotationValueAssertion
}

func (m *haveAnnotationValue) Match(actual interface{}) (success bool, err error) {
	actualAnnotations := podAnnotations(actual)
	value, ok := actualAnnotations[m.assertion.Name]
	return ok && value == m.assertion.Value, nil
}

func (m *haveAnnotationValue) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected annotation %s to have value: %s. \nActual annotations: %s", m.assertion.Name, m.assertion.Value, podAnnotations(actual))
}

func (m *haveAnnotationValue) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected annotation %s to not have value: %s. \nActual annotations: %s", m.assertion.Name, m.assertion.Value, podAnnotations(actual))
}

func podAnnotations(actual interface{}) map[string]string {
	lr := actual.(*kubernetesResource)
	pod := lr.Resource.(coreV1.Pod)
	return pod.Annotations
}

func podAnnotationNames(actual interface{}) []string {
	annotations := podAnnotations(actual)
	annotationNames := make([]string, len(annotations))

	i := 0
	for k := range annotations {
		annotationNames[i] = k
		i++
	}

	return annotationNames
}

// HaveJobSpec
func HaveJobSpec(expected interface{}) types.GomegaMatcher {
	return &haveScheduleOptions{expected.(*JobExpectation)}
}

type JobExpectation struct {
	Schedule         string
	ContainerImage   string
	ContainerCommand []string
}

type haveScheduleOptions struct {
	expected *JobExpectation
}

func (m *haveScheduleOptions) Match(actual interface{}) (success bool, err error) {
	job := m.job(actual)
	if job.Spec.Schedule != m.expected.Schedule {
		return false, fmt.Errorf("expected resource schedule to be %s for resource %s, but was %s", m.expected.Schedule, job.Name, job.Spec.Schedule)
	}

	if job.Spec.ConcurrencyPolicy != batch.ForbidConcurrent {
		return false, fmt.Errorf("expected resource's ConcurrencyPolicy to be ForbidConcurrent, was %s", job.Spec.ConcurrencyPolicy)
	}

	containers := job.Spec.JobTemplate.Spec.Template.Spec.Containers
	if len(containers) != 1 {
		return false, fmt.Errorf("expected resource '%s' to have exactly 1 container, but found %d: %v", job.Name, len(containers), containers)
	}

	container := containers[0]
	if !strings.Contains(container.Image, m.expected.ContainerImage) {
		return false, fmt.Errorf("expected container image for resource '%s' to contain %s, but was %s", job.Name, m.expected.ContainerImage, container.Image)
	}

	if !reflect.DeepEqual(container.Command, m.expected.ContainerCommand) {
		return false, fmt.Errorf("expected container command for resource '%s' to be %v, but was %v", job.Name, m.expected.ContainerCommand, container.Command)
	}

	return true, nil
}

func (m *haveScheduleOptions) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected resource to have spec: %v. \nActual resource: %v", m.expected, m.job(actual))
}

func (m *haveScheduleOptions) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected resource not to have spec: %v. \nActual resource: %v", m.expected, m.job(actual))
}

func (m *haveScheduleOptions) job(actual interface{}) batch.CronJob {
	lr := actual.(*kubernetesResource)
	return lr.Resource.(batch.CronJob)
}

//
// BeCreatedOnOrAfter matcher
//
func BeCreatedOnOrAfter(targetTime time.Time) types.GomegaMatcher {
	return &beCreatedOnOrAfter{targetTime: targetTime}
}

type beCreatedOnOrAfter struct {
	targetTime time.Time
}

func (m *beCreatedOnOrAfter) Match(actual interface{}) (success bool, err error) {
	actualCreationTime := m.resource(actual).CreationTimestamp()
	return actualCreationTime.Time.Equal(m.targetTime) || actualCreationTime.After(m.targetTime), nil
}

func (m *beCreatedOnOrAfter) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected resource creation time to be on or after %s. Actual creation time: %s", m.targetTime, m.resource(actual).CreationTimestamp())
}

func (m *beCreatedOnOrAfter) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected resource creation time not to be on or after %s. Actual creation time: %s", m.targetTime, m.resource(actual).CreationTimestamp())
}

func (m *beCreatedOnOrAfter) resource(actual interface{}) *kubernetesResource {
	lr := actual.(*kubernetesResource)
	return lr
}

//
// HaveResourcesRequirements matcher
//
func HaveResourcesRequirements(expected *ResourceRequirementsAssertion) types.GomegaMatcher {
	return haveResourcesRequirements{expected}
}

type ResourceRequirementsAssertion struct {
	ContainerName                                    string
	MemoryRequest, MemoryLimit, CPULimit, CPURequest *string
}

type haveResourcesRequirements struct {
	expected *ResourceRequirementsAssertion
}

func (h haveResourcesRequirements) Match(actual interface{}) (success bool, err error) {
	lr := actual.(*kubernetesResource)
	container := h.findContainer(lr.Resource.(coreV1.Pod), h.expected.ContainerName)

	if container == nil {
		return false, fmt.Errorf("expected a container with name %s", h.expected.ContainerName)
	}

	if equal, err := h.areQuantityEqual(h.expected.MemoryRequest, container.Resources.Requests, coreV1.ResourceMemory); !equal {
		return false, fmt.Errorf("memory request is not as expected: %v", err)
	}

	if equal, err := h.areQuantityEqual(h.expected.MemoryLimit, container.Resources.Limits, coreV1.ResourceMemory); !equal {
		return false, fmt.Errorf("memory limit is not as expected: %v", err)
	}

	if equal, err := h.areQuantityEqual(h.expected.CPURequest, container.Resources.Requests, coreV1.ResourceCPU); !equal {
		return false, fmt.Errorf("cpu request is not as expected: %v", err)
	}

	if equal, err := h.areQuantityEqual(h.expected.CPULimit, container.Resources.Limits, coreV1.ResourceCPU); !equal {
		return false, fmt.Errorf("cpu limit is not as expected: %v", err)
	}

	return true, nil
}

func (h haveResourcesRequirements) areQuantityEqual(expected *string, resourceList coreV1.ResourceList, resourceName coreV1.ResourceName) (bool, error) {
	var actual *string
	if quantity, ok := resourceList[resourceName]; ok {
		actual = ptr.String(quantity.String())
	}

	if !reflect.DeepEqual(expected, actual) {
		return false, fmt.Errorf("expected %s, but got %v", ptr.StringValueOrNil(expected), ptr.StringValueOrNil(actual))
	}
	return true, nil
}

func (h haveResourcesRequirements) findContainer(pod coreV1.Pod, containerName string) *coreV1.Container {
	for _, c := range pod.Spec.Containers {
		if c.Name == containerName {
			return &c
		}
	}
	return nil
}

func (h haveResourcesRequirements) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Actual pod: %s", actual.(*kubernetesResource).Resource)
}

func (h haveResourcesRequirements) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Actual pod: %s", actual.(*kubernetesResource).Resource)
}
