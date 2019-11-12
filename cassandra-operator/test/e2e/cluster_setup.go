package e2e

import (
	"fmt"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test/apis"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/api/resource"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	coreV1 "k8s.io/api/core/v1"
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

type ClusterBuilder struct {
	clusterName         string
	datacenter          *string
	clusterSpec         *v1alpha1.CassandraSpec
	racks               []v1alpha1.Rack
	podSpec             *v1alpha1.Pod
	snapshot            *v1alpha1.Snapshot
	extraConfigFile     *ExtraConfigFile
	withoutCustomConfig bool
}

//
// ClusterBuilder
//

func AClusterWithName(clusterName string) *ClusterBuilder {
	return &ClusterBuilder{clusterName: clusterName}
}

func (c *ClusterBuilder) AndRacks(racks []v1alpha1.Rack) *ClusterBuilder {
	c.racks = racks
	return c
}

func (c *ClusterBuilder) WithoutRacks() *ClusterBuilder {
	return c.AndRacks([]v1alpha1.Rack{})
}

func (c *ClusterBuilder) AndCustomConfig(extraConfigFile *ExtraConfigFile) *ClusterBuilder {
	c.extraConfigFile = extraConfigFile
	return c
}

func (c *ClusterBuilder) AndClusterSpec(clusterSpec *v1alpha1.CassandraSpec) *ClusterBuilder {
	c.clusterSpec = clusterSpec
	return c
}

func (c *ClusterBuilder) AndPodSpec(podSpec *v1alpha1.Pod) *ClusterBuilder {
	c.podSpec = podSpec
	return c
}

func (c *ClusterBuilder) ForDatacenter(datacenter string) *ClusterBuilder {
	c.datacenter = &datacenter
	return c
}

func (c *ClusterBuilder) WithoutCustomConfig() *ClusterBuilder {
	c.withoutCustomConfig = true
	return c
}

func (c *ClusterBuilder) AndScheduledSnapshot(snapshot *v1alpha1.Snapshot) *ClusterBuilder {
	c.snapshot = snapshot
	return c
}

func (c *ClusterBuilder) IsDefined() {
	if c.clusterSpec == nil {
		if c.podSpec == nil {
			c.podSpec = PodSpec().Build()
		}

		c.clusterSpec = &v1alpha1.CassandraSpec{}
		c.clusterSpec.Racks = c.racks
		c.clusterSpec.Snapshot = c.snapshot
		c.clusterSpec.Datacenter = c.datacenter
		c.clusterSpec.Pod = *c.podSpec
	}

	if !c.withoutCustomConfig {
		_, err := customCassandraConfigMap(Namespace, c.clusterName, false, c.extraConfigFile)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}
	_, err := cassandraResource(Namespace, c.clusterName, c.clusterSpec)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
}

func (c *ClusterBuilder) Exists() {
	c.IsDefined()
	EventuallyClusterIsCreatedWithRacks(Namespace, c.clusterName, c.racks)
	log.Infof("Created cluster %s", c.clusterName)
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
	return apis.ARack(rackName, replicas).
		WithZone(fmt.Sprintf("%s%s", dataCenterRegion, rackName)).
		WithStorage(
			apis.APersistentVolume().
				OfSize(podStorageSize).
				WithStorageClass(fmt.Sprintf("%s%s", storageClassPrefix, rackName))).
		Build()
}

func RackWithEmptyDir(rackName string, replicas int32) v1alpha1.Rack {
	return apis.ARack(rackName, replicas).
		WithZone(fmt.Sprintf("%s%s", dataCenterRegion, rackName)).
		WithEmptyDir().
		Build()
}

func PodSpec() *apis.PodSpecBuilder {
	return apis.APod().
		WithImageName(CassandraImageName).
		WithBootstrapperImageName(CassandraBootstrapperImageName).
		WithSidecarImageName(CassandraSidecarImageName).
		WithResources(&coreV1.ResourceRequirements{
			Requests: coreV1.ResourceList{
				coreV1.ResourceMemory: resource.MustParse(PodMemory),
				coreV1.ResourceCPU:    resource.MustParse(PodCPU),
			},
			Limits: coreV1.ResourceList{
				coreV1.ResourceMemory: resource.MustParse(PodMemory),
			},
		}).
		WithCassandraInitialDelay(CassandraInitialDelay).
		WithCassandraLivenessPeriod(CassandraLivenessPeriod).
		WithCassandraLivenessProbeFailureThreshold(CassandraLivenessProbeFailureThreshold).
		WithCassandraLivenessTimeout(CassandraLivenessTimeout).
		WithCassandraReadinessPeriod(CassandraReadinessPeriod).
		WithCassandraReadinessProbeFailureThreshold(CassandraReadinessProbeFailureThreshold).
		WithCassandraReadinessTimeout(CassandraReadinessTimeout)
}

func SnapshotSchedule(cron string) *v1alpha1.Snapshot {
	return &v1alpha1.Snapshot{
		Image:    CassandraSnapshotImageName,
		Schedule: cron,
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

func customCassandraConfigMap(namespace, clusterName string, update bool, extraFiles ...*ExtraConfigFile) (*coreV1.ConfigMap, error) {
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
	var cm *coreV1.ConfigMap
	if update {
		cm, err = cmClient.Get(fmt.Sprintf("%s-config", clusterName), metaV1.GetOptions{})
		if err != nil {
			return nil, err
		}
		cm.Data = configData

		return cmClient.Update(cm)
	} else {
		cm = &coreV1.ConfigMap{
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
