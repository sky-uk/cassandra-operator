package e2e

import (
	"fmt"
	"github.com/onsi/gomega"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/ptr"
	"io/ioutil"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const (
	OperatorLabel = "cassandra-snapshot-test"
)

var (
	securityContext = &v1.PodSecurityContext{
		RunAsUser:  ptr.Int64(cluster.UserID),
		RunAsGroup: ptr.Int64(cluster.GroupID),
		FSGroup:    ptr.Int64(cluster.GroupID),
	}
)

func PodIsReady(podToCheck *v1.Pod) func() (bool, error) {
	return func() (bool, error) {
		pod, err := KubeClientset.CoreV1().Pods(Namespace).Get(podToCheck.Name, metaV1.GetOptions{})
		if err != nil {
			return false, err
		}

		for _, condition := range pod.Status.Conditions {
			if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	}
}

func PodIsTerminatedSuccessfully(podToCheck *v1.Pod) func() (bool, error) {
	return func() (bool, error) {
		successfullyTerminatedCount, _ := getTerminatedContainerCount(podToCheck)
		return successfullyTerminatedCount == len(podToCheck.Spec.Containers), nil
	}
}

func PodIsTerminatedUnsuccessfully(podToCheck *v1.Pod) func() (bool, error) {
	return func() (bool, error) {
		_, unsuccessfullyTerminatedCount := getTerminatedContainerCount(podToCheck)
		return unsuccessfullyTerminatedCount == len(podToCheck.Spec.Containers), nil
	}
}

func getTerminatedContainerCount(podToCheck *v1.Pod) (int, int) {
	pod, err := KubeClientset.CoreV1().Pods(Namespace).Get(podToCheck.Name, metaV1.GetOptions{})
	if err != nil {
		return 0, 0
	}

	successfullyTerminatedCount := 0
	unsuccessfullyTerminatedCount := 0
	for _, condition := range pod.Status.ContainerStatuses {
		if terminatedState := condition.State.Terminated; terminatedState != nil {
			if terminatedState.ExitCode == 0 {
				successfullyTerminatedCount++
			} else {
				unsuccessfullyTerminatedCount++
			}
		}
	}

	return successfullyTerminatedCount, unsuccessfullyTerminatedCount
}

func RunCommandInCassandraSnapshotPod(clusterName, command string, arg ...string) *v1.Pod {
	var commandToRun []string
	commandToRun = append(commandToRun, command)
	commandToRun = append(commandToRun, arg...)

	pod, err := KubeClientset.CoreV1().Pods(Namespace).Create(&v1.Pod{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      fmt.Sprintf("test-command-runner-%s", randomString(5)),
			Namespace: Namespace,
			Labels:    map[string]string{OperatorLabel: clusterName, "test-command-runner": ""},
		},
		Spec: v1.PodSpec{
			SecurityContext:    securityContext,
			ServiceAccountName: "cassandra-snapshot",
			RestartPolicy:      v1.RestartPolicyNever,
			Containers: []v1.Container{
				{
					Name:      "command-runner",
					Image:     ImageUnderTest,
					Command:   commandToRun,
					Resources: resourceRequirementsOf("50Mi"),
				},
			},
		},
	})

	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	return pod
}

func SnapshotListForPod(pod *v1.Pod) ([]Snapshot, error) {
	cmd, output, err := Kubectl(Namespace, pod.Name, "nodetool", "listsnapshots")
	var snapshots []Snapshot
	if err != nil {
		return snapshots, fmt.Errorf("error while executing %v on pod %s: %v", cmd, pod.Name, err)
	}

	// Regex matches the following and captures snapshot name, keyspace name and column family name:
	// Snapshot name    Keyspace name Column family name True size Size on disk
	// another_snapshot system_auth   roles              4.95 KiB  4.98 KiB
	// 1545060459       system_traces events             0 bytes   13 bytes
	re := regexp.MustCompile("^(\\w+) +(\\w+) +(\\w+) +\\d+(?:\\.\\d+)? +\\w+ +\\d+(?:\\.\\d+)? +\\w+")

	for _, snapshotLine := range strings.Split(string(output), "\n") {
		lineAsBytes := []byte(snapshotLine)
		if re.Match(lineAsBytes) {
			submatches := re.FindAllStringSubmatch(snapshotLine, -1)
			snapshot := Snapshot{submatches[0][1], submatches[0][2], submatches[0][3]}
			snapshots = append(snapshots, snapshot)
		}
	}

	return snapshots, nil
}

func BackdateSnapshotsForPods(pods []*v1.Pod, backdatePeriod time.Duration) {
	newSnapshotName := time.Now().Unix() - int64(backdatePeriod.Seconds())
	renameSnapshot(pods, fmt.Sprintf("%d", newSnapshotName))
}

func RenameSnapshotsForPod(pod *v1.Pod, snapshotName string) {
	renameSnapshot([]*v1.Pod{pod}, snapshotName)
}

func renameSnapshot(pods []*v1.Pod, snapshotName string) {
	for _, pod := range pods {
		cmd, output, err := Kubectl(Namespace, pod.Name, "--", "bash", "-c", fmt.Sprintf(RenameSnapshotCmd, snapshotName))
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), fmt.Sprintf("Renaming snapshots failed when executing command in pod: %s,cmd: %v, output: %v, err: %v", pod.Name, cmd, string(output), err))
	}
}

type Snapshot struct {
	Name         string
	Keyspace     string
	ColumnFamily string
}

func cassandraConfigMap(namespace, resourceName string) (*v1.ConfigMap, error) {
	configData := make(map[string]string)
	configDir := cassandraConfigDir()
	configFiles := []string{"jvm.options"}

	for _, configFile := range configFiles {
		fileContent, err := readFileContent(fmt.Sprintf("%s%s%s", configDir, string(filepath.Separator), configFile))
		if err != nil {
			return nil, err
		}
		configData[configFile] = fileContent
	}

	cmClient := KubeClientset.CoreV1().ConfigMaps(namespace)
	cm := &v1.ConfigMap{
		ObjectMeta: metaV1.ObjectMeta{
			Name: fmt.Sprintf("%s-config", resourceName),
			Labels: map[string]string{
				OperatorLabel: resourceName,
			},
		},
		Data: configData,
	}
	return cmClient.Create(cm)
}

func cassandraConfigDir() string {
	_, currentFilename, _, _ := runtime.Caller(0)
	testDir, err := absolutePathOf("test", currentFilename)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	return fmt.Sprintf("%s%s%s", testDir, string(filepath.Separator), "cassandra-config")
}

func absolutePathOf(target, currentDir string) (string, error) {
	path := strings.Split(currentDir, string(filepath.Separator))
	for i := range path {
		if path[i] == target {
			return strings.Join(path[:i+1], string(filepath.Separator)), nil
		}
	}

	return "", fmt.Errorf("target %s does not exist in path %s", target, currentDir)
}

func readFileContent(fileName string) (string, error) {
	bytes, err := ioutil.ReadFile(fileName)
	if err != nil {
		return "", err
	}

	fileContent := string(bytes)
	return fileContent, err
}

func resourceRequirementsOf(quota string) v1.ResourceRequirements {
	return v1.ResourceRequirements{
		Limits: v1.ResourceList{
			v1.ResourceMemory: resource.MustParse(quota),
		},
		Requests: v1.ResourceList{
			v1.ResourceMemory: resource.MustParse(quota),
		},
	}
}
