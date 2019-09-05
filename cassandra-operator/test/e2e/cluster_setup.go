package e2e

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/ptr"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PodMemory          = "1Gi"
	PodCPU             = "0"
	podStorageSize     = "1Gi"
	dataCenterRegion   = "eu-west-1"
	storageClassPrefix = "standard-zone-"
)

type ExtraConfigFile struct {
	Name    string
	Content string
}

type TestCluster struct {
	Name                string
	Racks               []v1alpha1.Rack
	ExtraConfigFileName string
	SnapshotConfig      *v1alpha1.Snapshot
}

func AClusterName() string {
	clusterName := fmt.Sprintf("mycluster-%s", randomString(5))
	log.Infof("Generating cluster name %s", clusterName)
	return clusterName
}

func PodName(clusterName, rack string, count int) string {
	return fmt.Sprintf("%s-%s-%d", clusterName, rack, count)
}

func Rack(rackName string, replicas int32) v1alpha1.Rack {
	return v1alpha1.Rack{
		Name:         rackName,
		Replicas:     replicas,
		StorageClass: fmt.Sprintf("%s%s", storageClassPrefix, rackName),
		Zone:         fmt.Sprintf("%s%s", dataCenterRegion, rackName),
	}
}

func RackWithEmptyDir(rackName string, replicas int32) v1alpha1.Rack {
	return v1alpha1.Rack{
		Name:     rackName,
		Replicas: replicas,
	}
}

func SnapshotSchedule(cron string) *v1alpha1.Snapshot {
	return &v1alpha1.Snapshot{
		Image:    &CassandraSnapshotImageName,
		Schedule: cron,
	}
}

func clusterDefaultSpec() *v1alpha1.CassandraSpec {
	return &v1alpha1.CassandraSpec{
		Racks:       []v1alpha1.Rack{},
		UseEmptyDir: ptr.Bool(false),
		Pod: v1alpha1.Pod{
			BootstrapperImage: &CassandraBootstrapperImageName,
			SidecarImage:      &CassandraSidecarImageName,
			Image:             &CassandraImageName,
			Resources: coreV1.ResourceRequirements{
				Requests: coreV1.ResourceList{
					coreV1.ResourceMemory: resource.MustParse(PodMemory),
					coreV1.ResourceCPU:    resource.MustParse(PodCPU),
				},
				Limits: coreV1.ResourceList{
					coreV1.ResourceMemory: resource.MustParse(PodMemory),
				},
			},
			StorageSize: resource.MustParse(podStorageSize),
			LivenessProbe: &v1alpha1.Probe{
				FailureThreshold:    ptr.Int32(CassandraLivenessProbeFailureThreshold),
				InitialDelaySeconds: ptr.Int32(CassandraInitialDelay),
				PeriodSeconds:       ptr.Int32(CassandraLivenessPeriod),
			},
			ReadinessProbe: &v1alpha1.Probe{
				FailureThreshold:    ptr.Int32(CassandraReadinessProbeFailureThreshold),
				InitialDelaySeconds: ptr.Int32(CassandraInitialDelay),
				PeriodSeconds:       ptr.Int32(CassandraReadinessPeriod),
			},
		},
	}
}

func cassandraResource(namespace, clusterName string, clusterSpec *v1alpha1.CassandraSpec) (*v1alpha1.Cassandra, error) {
	cassandraClient := CassandraClientset.CoreV1alpha1().Cassandras(namespace)
	return cassandraClient.Create(&v1alpha1.Cassandra{
		ObjectMeta: metaV1.ObjectMeta{
			Name: clusterName,
		},
		Spec: *clusterSpec,
	})
}

func customCassandraConfigMap(namespace, clusterName string, extraFiles ...*ExtraConfigFile) (*coreV1.ConfigMap, error) {
	configData := make(map[string]string)

	for _, extraFile := range extraFiles {
		if extraFile != nil {
			configData[extraFile.Name] = extraFile.Content
		}
	}

	fileContent, err := readFileContent(defaultJVMOptionsLocation())
	if err != nil {
		return nil, err
	}
	configData["jvm.options"] = fileContent

	cmClient := KubeClientset.CoreV1().ConfigMaps(namespace)
	cm := &coreV1.ConfigMap{
		ObjectMeta: metaV1.ObjectMeta{
			Name: fmt.Sprintf("%s-config", clusterName),
			Labels: map[string]string{
				cluster.OperatorLabel: clusterName,
			},
		},
		Data: configData,
	}

	return cmClient.Create(cm)
}

func defaultJVMOptionsLocation() string {
	_, currentFilename, _, _ := runtime.Caller(0)
	testDir, err := absolutePathOf("test", currentFilename)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	return fmt.Sprintf("%s%s%s", testDir, string(filepath.Separator), "jvm.options")
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

func DefaultJvmOptionsWithLine(lineToAppend string) string {
	fileContent, err := readFileContent(defaultJVMOptionsLocation())
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	return strings.Replace(fmt.Sprintf("%s\n%s\n", fileContent, lineToAppend), "\n", "\\n", -1)
}

func readFileContent(fileName string) (string, error) {
	bytes, err := ioutil.ReadFile(fileName)
	if err != nil {
		return "", err
	}

	fileContent := string(bytes)
	return fileContent, err
}
