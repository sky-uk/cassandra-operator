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

type ClusterBuilder struct {
	clusterName         string
	racks               []v1alpha1.Rack
	extraConfigFile     *ExtraConfigFile
	clusterSpec         *v1alpha1.CassandraSpec
	withoutCustomConfig bool
	snapshot            *v1alpha1.Snapshot
}

type ClusterSpecBuilder struct {
	podResources *coreV1.ResourceRequirements
}

type RackSpecBuilder struct {
	rackName string
	replicas int32
	storages []v1alpha1.Storage
}

type EmptyDirStorageBuilder struct {
	path *string
}

type PersistentVolumeStorageBuilder struct {
	path         *string
	storageClass string
	storageSize  string
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
		c.clusterSpec = AClusterSpec().Build()
	}

	c.clusterSpec.Racks = c.racks
	c.clusterSpec.Snapshot = c.snapshot

	if !c.withoutCustomConfig {
		_, err := customCassandraConfigMap(Namespace, c.clusterName, c.extraConfigFile)
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

//
// ClusterSpecBuilder
//

func AClusterSpec() *ClusterSpecBuilder {
	return &ClusterSpecBuilder{}
}

func (s *ClusterSpecBuilder) WithPodResources(podResources *coreV1.ResourceRequirements) *ClusterSpecBuilder {
	s.podResources = podResources
	return s
}

func (s *ClusterSpecBuilder) Build() *v1alpha1.CassandraSpec {
	spec := clusterDefaultSpec()
	if s.podResources != nil {
		spec.Pod.Resources = *s.podResources
	}
	return spec
}

//
// StorageBuilder
//

type StorageBuilder interface {
	Build() v1alpha1.Storage
}

//
// PersistentVolumeStorageBuilder
//

func APersistentVolume() *PersistentVolumeStorageBuilder {
	return &PersistentVolumeStorageBuilder{}
}

func (pv *PersistentVolumeStorageBuilder) AtPath(path string) *PersistentVolumeStorageBuilder {
	pv.path = ptr.String(path)
	return pv
}

func (pv *PersistentVolumeStorageBuilder) WithStorageClass(storageClass string) *PersistentVolumeStorageBuilder {
	pv.storageClass = storageClass
	return pv
}

func (pv *PersistentVolumeStorageBuilder) OfSize(storageSize string) *PersistentVolumeStorageBuilder {
	pv.storageSize = storageSize
	return pv
}

func (pv *PersistentVolumeStorageBuilder) Build() v1alpha1.Storage {
	if pv.storageSize == "" {
		pv.storageSize = podStorageSize
	}
	return v1alpha1.Storage{
		Path: pv.path,
		StorageSource: v1alpha1.StorageSource{
			PersistentVolumeClaim: &coreV1.PersistentVolumeClaimSpec{
				StorageClassName: ptr.String(pv.storageClass),
				Resources: coreV1.ResourceRequirements{
					Requests: coreV1.ResourceList{
						coreV1.ResourceStorage: resource.MustParse(pv.storageSize),
					},
				},
			},
		},
	}
}

//
// EmptyDirStorageBuilder
//

func AnEmptyDir() *EmptyDirStorageBuilder {
	return &EmptyDirStorageBuilder{}
}

func (e *EmptyDirStorageBuilder) AtPath(path string) *EmptyDirStorageBuilder {
	e.path = ptr.String(path)
	return e
}

func (e *EmptyDirStorageBuilder) Build() v1alpha1.Storage {
	return v1alpha1.Storage{
		Path: e.path,
		StorageSource: v1alpha1.StorageSource{
			EmptyDir: &coreV1.EmptyDirVolumeSource{},
		},
	}
}

//
// RackSpecBuilder
//

func ARack(name string, replicas int32) *RackSpecBuilder {
	return &RackSpecBuilder{rackName: name, replicas: replicas}
}

func (r *RackSpecBuilder) WithEmptyDir() *RackSpecBuilder {
	return r.WithStorage(AnEmptyDir())
}

func (r *RackSpecBuilder) WithPersistentVolume() *RackSpecBuilder {
	return r.WithStorage(APersistentVolume().WithStorageClass(fmt.Sprintf("%s%s", storageClassPrefix, r.rackName)))
}

func (r *RackSpecBuilder) WithStorage(storageBuilder StorageBuilder) *RackSpecBuilder {
	storage := storageBuilder.Build()
	r.storages = append(r.storages, storage)
	return r
}

func (r *RackSpecBuilder) Build() v1alpha1.Rack {
	return v1alpha1.Rack{
		Name:     r.rackName,
		Replicas: r.replicas,
		Zone:     fmt.Sprintf("%s%s", dataCenterRegion, r.rackName),
		Storage:  r.storages,
	}
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
	return ARack(rackName, replicas).WithPersistentVolume().Build()
}

func RackWithEmptyDir(rackName string, replicas int32) v1alpha1.Rack {
	return ARack(rackName, replicas).WithEmptyDir().Build()
}

func SnapshotSchedule(cron string) *v1alpha1.Snapshot {
	return &v1alpha1.Snapshot{
		Image:    &CassandraSnapshotImageName,
		Schedule: cron,
	}
}

func clusterDefaultSpec() *v1alpha1.CassandraSpec {
	return &v1alpha1.CassandraSpec{
		Racks: []v1alpha1.Rack{},
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
