package helpers

import (
	"fmt"
	"reflect"

	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/ptr"
)

// IsAReservedEnvVar is used to determine if an Environment Variable has been reserved.
func IsAReservedEnvVar(envVar string) bool {
	isReserved := map[string]bool{"EXTRA_CLASSPATH": true}
	return isReserved[envVar]
}

// ImageScheme is used to determine default names for operator components docker images
type ImageScheme interface {
	defaultName(imageName string) *string
}

// TemplatedImageScheme constructs an image name based on a version and repository path
type TemplatedImageScheme struct {
	ImageVersion   string
	RepositoryPath string
}

func (i *TemplatedImageScheme) defaultName(imageName string) *string {
	return ptr.String(fmt.Sprintf("%s/%s:%s", i.RepositoryPath, imageName, i.ImageVersion))
}

// skyUkLatestImageScheme constructs an image name using skyuk repository latest image
type skyUkLatestImageScheme struct {
}

func (sky *skyUkLatestImageScheme) defaultName(imageName string) *string {
	return ptr.String(fmt.Sprintf("skyuk/%s:latest", imageName))
}

func NewControllerRef(c *v1alpha1.Cassandra) metav1.OwnerReference {
	return *metav1.NewControllerRef(c, schema.GroupVersionKind{
		Group:   cassandra.GroupName,
		Version: cassandra.Version,
		Kind:    cassandra.Kind,
	})
}

func IsAPersistentVolumeClaim(storage v1alpha1.Storage) bool {
	return storage.PersistentVolumeClaim != nil
}

func IsAReservedVolumePath(path string) bool {
	return path == v1alpha1.ConfigurationVolumeMountPath || path == v1alpha1.ExtraLibVolumeMountPath
}

// SnapshotPropertiesUpdated returns false when snapshot1 and snapshot2 have the same properties disregarding retention policy
func SnapshotPropertiesUpdated(snapshot1 *v1alpha1.Snapshot, snapshot2 *v1alpha1.Snapshot) bool {
	return snapshot1.Schedule != snapshot2.Schedule ||
		!reflect.DeepEqual(snapshot1.TimeoutSeconds, snapshot2.TimeoutSeconds) ||
		!reflect.DeepEqual(snapshot1.Keyspaces, snapshot2.Keyspaces) ||
		!reflect.DeepEqual(snapshot1.Resources, snapshot2.Resources)
}

// SnapshotCleanupPropertiesUpdated returns false snapshot1 and snapshot2 have the same retention policy
func SnapshotCleanupPropertiesUpdated(snapshot1 *v1alpha1.Snapshot, snapshot2 *v1alpha1.Snapshot) bool {
	return snapshot1.RetentionPolicy != nil && snapshot2.RetentionPolicy != nil &&
		(snapshot1.RetentionPolicy.CleanupSchedule != snapshot2.RetentionPolicy.CleanupSchedule ||
			!reflect.DeepEqual(snapshot1.RetentionPolicy.CleanupTimeoutSeconds, snapshot2.RetentionPolicy.CleanupTimeoutSeconds) ||
			!reflect.DeepEqual(snapshot1.RetentionPolicy.RetentionPeriodDays, snapshot2.RetentionPolicy.RetentionPeriodDays) ||
			!reflect.DeepEqual(snapshot1.RetentionPolicy.Resources, snapshot2.RetentionPolicy.Resources))
}

func SetDefaultsForCassandra(clusterDefinition *v1alpha1.Cassandra, imageDefaultScheme ImageScheme) {
	setDefaultsForDatacenter(clusterDefinition)
	setDefaultsForSnapshot(clusterDefinition.Spec.Snapshot)
	setDefaultsForImages(clusterDefinition, imageDefaultScheme)
	setDefaultsForProbes(clusterDefinition)
	setDefaultsForStorage(clusterDefinition)
}

func setDefaultsForStorage(clusterDefinition *v1alpha1.Cassandra) {
	for i := range clusterDefinition.Spec.Racks {
		for j := range clusterDefinition.Spec.Racks[i].Storage {
			if clusterDefinition.Spec.Racks[i].Storage[j].PersistentVolumeClaim != nil && clusterDefinition.Spec.Racks[i].Storage[j].PersistentVolumeClaim.AccessModes == nil {
				clusterDefinition.Spec.Racks[i].Storage[j].PersistentVolumeClaim.AccessModes = []coreV1.PersistentVolumeAccessMode{coreV1.ReadWriteOnce}
			}
			if clusterDefinition.Spec.Racks[i].Storage[j].Path == nil {
				clusterDefinition.Spec.Racks[i].Storage[j].Path = ptr.String(v1alpha1.DefaultStorageVolumeMountPath)
			}
		}
	}
}

func setDefaultsForDatacenter(clusterDefinition *v1alpha1.Cassandra) {
	if clusterDefinition.Spec.Datacenter == nil {
		clusterDefinition.Spec.Datacenter = ptr.String(v1alpha1.DefaultDatacenterName)
	}
}

func setDefaultsForSnapshot(snapshot *v1alpha1.Snapshot) {
	if snapshot == nil {
		return
	}
	if snapshot.TimeoutSeconds == nil {
		snapshot.TimeoutSeconds = ptr.Int32(v1alpha1.DefaultSnapshotTimeoutSeconds)
	}

	if snapshot.RetentionPolicy != nil {
		setDefaultsForRetentionPolicy(snapshot.RetentionPolicy)
	}
}

func setDefaultsForRetentionPolicy(rp *v1alpha1.RetentionPolicy) {
	if rp.RetentionPeriodDays == nil {
		rp.RetentionPeriodDays = ptr.Int32(v1alpha1.DefaultRetentionPolicyRetentionPeriodDays)
	}
	if rp.CleanupTimeoutSeconds == nil {
		rp.CleanupTimeoutSeconds = ptr.Int32(v1alpha1.DefaultRetentionPolicyCleanupTimeoutSeconds)
	}
}

func setDefaultsForImages(clusterDefinition *v1alpha1.Cassandra, imageScheme ImageScheme) {
	if imageScheme == nil {
		imageScheme = &skyUkLatestImageScheme{}
	}

	if clusterDefinition.Spec.Pod.Image == nil {
		clusterDefinition.Spec.Pod.Image = ptr.String(v1alpha1.DefaultCassandraImage)
	}
	if clusterDefinition.Spec.Pod.BootstrapperImage == nil {
		clusterDefinition.Spec.Pod.BootstrapperImage = imageScheme.defaultName(v1alpha1.DefaultCassandraBootstrapperImageName)
	}
	if clusterDefinition.Spec.Pod.Sidecar.Image == nil {
		clusterDefinition.Spec.Pod.Sidecar.Image = imageScheme.defaultName(v1alpha1.DefaultCassandraSidecarImageName)
	}
	if clusterDefinition.Spec.Snapshot != nil && clusterDefinition.Spec.Snapshot.Image == nil {
		clusterDefinition.Spec.Snapshot.Image = imageScheme.defaultName(v1alpha1.DefaultCassandraSnapshotImageName)
	}

}

func setDefaultsForProbes(clusterDefinition *v1alpha1.Cassandra) {
	if clusterDefinition.Spec.Pod.LivenessProbe == nil {
		defaultProbe := defaultLivenessProbe()
		clusterDefinition.Spec.Pod.LivenessProbe = &defaultProbe
	} else {
		livenessProbe := clusterDefinition.Spec.Pod.LivenessProbe
		mergeProbeDefaults(livenessProbe, defaultLivenessProbe())
	}

	if clusterDefinition.Spec.Pod.ReadinessProbe == nil {
		defaultProbe := defaultReadinessProbe()
		clusterDefinition.Spec.Pod.ReadinessProbe = &defaultProbe
	} else {
		readinessProbe := clusterDefinition.Spec.Pod.ReadinessProbe
		mergeProbeDefaults(readinessProbe, defaultReadinessProbe())
	}
}

func mergeProbeDefaults(configuredProbe *v1alpha1.Probe, defaultProbe v1alpha1.Probe) {
	if configuredProbe.TimeoutSeconds == nil {
		configuredProbe.TimeoutSeconds = defaultProbe.TimeoutSeconds
	}

	if configuredProbe.SuccessThreshold == nil {
		configuredProbe.SuccessThreshold = defaultProbe.SuccessThreshold
	}

	if configuredProbe.FailureThreshold == nil {
		configuredProbe.FailureThreshold = defaultProbe.FailureThreshold
	}

	if configuredProbe.InitialDelaySeconds == nil {
		configuredProbe.InitialDelaySeconds = defaultProbe.InitialDelaySeconds
	}

	if configuredProbe.PeriodSeconds == nil {
		configuredProbe.PeriodSeconds = defaultProbe.PeriodSeconds
	}
}

func defaultReadinessProbe() v1alpha1.Probe {
	return v1alpha1.Probe{
		FailureThreshold:    ptr.Int32(3),
		InitialDelaySeconds: ptr.Int32(30),
		PeriodSeconds:       ptr.Int32(15),
		SuccessThreshold:    ptr.Int32(1),
		TimeoutSeconds:      ptr.Int32(5),
	}
}

func defaultLivenessProbe() v1alpha1.Probe {
	return v1alpha1.Probe{
		FailureThreshold:    ptr.Int32(3),
		InitialDelaySeconds: ptr.Int32(30),
		PeriodSeconds:       ptr.Int32(30),
		SuccessThreshold:    ptr.Int32(1),
		TimeoutSeconds:      ptr.Int32(5),
	}
}

type MatchedRack struct {
	Old v1alpha1.Rack
	New v1alpha1.Rack
}

func MatchRacks(oldCluster, newCluster *v1alpha1.CassandraSpec) (addedRacks []v1alpha1.Rack, matchedRacks []MatchedRack, removedRacks []v1alpha1.Rack) {
	for _, oldRack := range oldCluster.Racks {
		if foundRack, ok := findRack(oldRack, newCluster.Racks); ok {
			matchedRacks = append(matchedRacks, MatchedRack{Old: oldRack, New: *foundRack})
		} else {
			removedRacks = append(removedRacks, oldRack)
		}
	}
	for _, newClusterRack := range newCluster.Racks {
		if _, ok := findRack(newClusterRack, oldCluster.Racks); !ok {
			addedRacks = append(addedRacks, newClusterRack)
		}
	}
	return addedRacks, matchedRacks, removedRacks
}

func findRack(rackToFind v1alpha1.Rack, racks []v1alpha1.Rack) (*v1alpha1.Rack, bool) {
	for _, rack := range racks {
		if rack.Name == rackToFind.Name {
			return &rack, true
		}
	}
	return nil, false
}
