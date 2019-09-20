package v1alpha1

import (
	"fmt"
	"github.com/google/go-cmp/cmp"
	"reflect"
	"sort"

	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	// required for dep management
	_ "k8s.io/code-generator/cmd/client-gen/types"
)

const (
	NodeServiceAccountName     = "cassandra-node"
	SnapshotServiceAccountName = "cassandra-snapshot"

	// DefaultDatacenterName is the default data center name which each Cassandra pod belongs to
	DefaultDatacenterName = "dc1"

	// DefaultCassandraImage is the name of the default Docker image used on Cassandra pods
	DefaultCassandraImage = "cassandra:3.11"

	// DefaultCassandraBootstrapperImageName is the name of the Docker image used to prepare the configuration for the Cassandra node before it can be started
	DefaultCassandraBootstrapperImageName = "cassandra-bootstrapper"

	// DefaultCassandraSnapshotImageName is the name of the Docker image used to make and cleanup snapshots
	DefaultCassandraSnapshotImageName = "cassandra-snapshot"

	// DefaultCassandraSidecarImageName is the name of the Docker image used to inform liveness/readiness probes
	DefaultCassandraSidecarImageName = "cassandra-sidecar"

	// DefaultSnapshotTimeoutSeconds is the default for Cassandra.Spec.Snapshot.TimeoutSeconds
	DefaultSnapshotTimeoutSeconds = 10

	// DefaultRetentionPolicyRetentionPeriodDays is the default for Cassandra.Spec.Snapshot.RetentionPolicy.RetentionPeriodDays
	DefaultRetentionPolicyRetentionPeriodDays = 7

	// DefaultRetentionPolicyCleanupTimeoutSeconds is the default for Cassandra.Spec.Snapshot.RetentionPolicy.CleanupTimeoutSeconds
	DefaultRetentionPolicyCleanupTimeoutSeconds = 10

	// DefaultStorageVolumeMountPath is the default location for Cassandra data storage
	DefaultStorageVolumeMountPath = "/var/lib/cassandra"

	// ConfigurationVolumeMountPath is the location for Cassandra configuration
	ConfigurationVolumeMountPath = "/etc/cassandra"

	// ExtraLibVolumeMountPath is the location for extra libraries required by the operator to function
	ExtraLibVolumeMountPath = "/extra-lib"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Cassandra defines a Cassandra cluster
type Cassandra struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CassandraSpec   `json:"spec,omitempty"`
	Status CassandraStatus `json:"status,omitempty"`
}

// CassandraSpec is the specification for the Cassandra resource
type CassandraSpec struct {
	// +optional
	Datacenter *string `json:"datacenter,omitempty"`
	Racks      []Rack  `json:"racks"`
	Pod        Pod     `json:"pod"`
	// +optional
	Snapshot *Snapshot `json:"snapshot,omitempty"`
}

type Probe struct {
	// +optional
	FailureThreshold *int32 `json:"failureThreshold,omitempty"`
	// +optional
	InitialDelaySeconds *int32 `json:"initialDelaySeconds,omitempty"`
	// +optional
	PeriodSeconds *int32 `json:"periodSeconds,omitempty"`
	// +optional
	SuccessThreshold *int32 `json:"successThreshold,omitempty"`
	// +optional
	TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`
}

type Pod struct {
	// +optional
	BootstrapperImage *string `json:"bootstrapperImage,omitempty"`

	// +optional
	SidecarImage *string `json:"sidecarImage,omitempty"`

	// +optional
	Image     *string                     `json:"image,omitempty"`
	Resources coreV1.ResourceRequirements `json:"resources"`
	// +optional
	LivenessProbe *Probe `json:"livenessProbe,omitempty"`
	// +optional
	ReadinessProbe *Probe `json:"readinessProbe,omitempty"`
}

// CassandraStatus is the status for the Cassandra resource
type CassandraStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CassandraList is a list of Cassandra resources
type CassandraList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Cassandra `json:"items"`
}

// Rack defines the properties of a rack in the cluster
type Rack struct {
	Name     string    `json:"name"`
	Zone     string    `json:"zone"`
	Replicas int32     `json:"replicas"`
	Storage  []Storage `json:"storage"`
}

// Storage defines the storage properties shared by pods in the same rack.
// Only one type of volume may be specified.
type Storage struct {
	// The full path to the volume in the pod
	Path          *string `json:"path,omitempty"`
	StorageSource `json:",inline"`
}

// StorageSource represents the volume source to use as storage
// Only one of its members may be specified.
// Supported volume sources are expected to be a subset of the volume types defined in `coreV1.VolumeSource`
type StorageSource struct {
	// The volume as persistent volume
	// +optional
	PersistentVolumeClaim *coreV1.PersistentVolumeClaimSpec `json:"persistentVolumeClaim,omitempty"`
	// The volume as an empty directory
	// +optional
	EmptyDir *coreV1.EmptyDirVolumeSource `json:"emptyDir,omitempty"`
}

// Snapshot defines the snapshot creation and deletion configuration
type Snapshot struct {
	// +optional
	Image *string `json:"image,omitempty"`
	// Schedule follows the cron format, see https://en.wikipedia.org/wiki/Cron
	Schedule string `json:"schedule"`
	// +optional
	Keyspaces []string `json:"keyspaces,omitempty"`
	// +optional
	TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`
	// +optional
	RetentionPolicy *RetentionPolicy `json:"retentionPolicy,omitempty"`
}

// RetentionPolicy defines how long the snapshots should be kept for and how often the cleanup task should run
type RetentionPolicy struct {
	// +optional
	RetentionPeriodDays *int32 `json:"retentionPeriodDays,omitempty"`
	// CleanupSchedule follows the cron format, see https://en.wikipedia.org/wiki/Cron
	CleanupSchedule string `json:"cleanupSchedule"`
	// +optional
	CleanupTimeoutSeconds *int32 `json:"cleanupTimeoutSeconds,omitempty"`
}

// QualifiedName is the cluster fully qualified name which follows the format <namespace>.<name>
func (c *Cassandra) QualifiedName() string {
	return fmt.Sprintf("%s.%s", c.Namespace, c.Name)
}

// SnapshotJobName is the name of the snapshot job for the cluster
func (c *Cassandra) SnapshotJobName() string {
	return fmt.Sprintf("%s-snapshot", c.Name)
}

// SnapshotCleanupJobName is the name of the snapshot cleanup job for the cluster
func (c *Cassandra) SnapshotCleanupJobName() string {
	return fmt.Sprintf("%s-snapshot-cleanup", c.Name)
}

// ServiceName is the cluster service name
func (c *Cassandra) ServiceName() string {
	return fmt.Sprintf("%s.%s", c.Name, c.Namespace)
}

// StorageVolumeName is the name of the volume used for storing Cassandra data
func (c *Cassandra) StorageVolumeName() string {
	return fmt.Sprintf("cassandra-storage-%s", c.Name)
}

// RackName is the fully qualifier name of the supplied rack within the cluster
func (c *Cassandra) RackName(rack *Rack) string {
	return fmt.Sprintf("%s-%s", c.Name, rack.Name)
}

// CustomConfigMapName returns the expected config map name for this cluster. This will return a value even if the config map does not exist.
func (c *Cassandra) CustomConfigMapName() string {
	return fmt.Sprintf("%s-config", c.Name)
}

// Equal checks equality of two CassandraSpecs. This is useful for checking equality with cmp.Equal
func (cs CassandraSpec) Equal(other CassandraSpec) bool {
	return reflect.DeepEqual(cs.Datacenter, other.Datacenter) &&
		cmp.Equal(sortedRacks(cs.Racks), sortedRacks(other.Racks)) &&
		cmp.Equal(cs.Pod, other.Pod) &&
		cmp.Equal(cs.Snapshot, other.Snapshot)
}

// Equal checks equality of two Snapshots. This is useful for testing with cmp.Equal
func (s Snapshot) Equal(other Snapshot) bool {
	return reflect.DeepEqual(s.Image, other.Image) &&
		reflect.DeepEqual(s.TimeoutSeconds, other.TimeoutSeconds) &&
		reflect.DeepEqual(sortedStrings(s.Keyspaces), sortedStrings(other.Keyspaces)) &&
		s.Schedule == other.Schedule &&
		cmp.Equal(s.RetentionPolicy, other.RetentionPolicy)
}

// Equal checks equality of two RetentionPolicy. This is useful for checking equality with cmp.Equal
func (rp RetentionPolicy) Equal(other RetentionPolicy) bool {
	return reflect.DeepEqual(rp.CleanupTimeoutSeconds, other.CleanupTimeoutSeconds) &&
		reflect.DeepEqual(rp.CleanupSchedule, other.CleanupSchedule) &&
		reflect.DeepEqual(rp.RetentionPeriodDays, other.RetentionPeriodDays)
}

// Equal checks equality of two Racks. This is useful for testing checking equality cmp.Equal
func (r Rack) Equal(other Rack) bool {
	return r.Name == other.Name &&
		r.Zone == other.Zone &&
		r.Replicas == other.Replicas &&
		cmp.Equal(r.Storage, other.Storage)
}

// Equal checks equality of two Storage. This is useful for testing checking equality cmp.Equal
func (ss Storage) Equal(other Storage) bool {
	return reflect.DeepEqual(ss.Path, other.Path) &&
		reflect.DeepEqual(ss.StorageSource, other.StorageSource)
}

// Equal checks equality of two Pods. This is useful for checking equality with cmp.Equal
func (p Pod) Equal(other Pod) bool {
	return reflect.DeepEqual(p.BootstrapperImage, other.BootstrapperImage) &&
		reflect.DeepEqual(p.SidecarImage, other.SidecarImage) &&
		reflect.DeepEqual(p.Image, other.Image) &&
		reflect.DeepEqual(p.LivenessProbe, other.LivenessProbe) &&
		reflect.DeepEqual(p.ReadinessProbe, other.ReadinessProbe) &&
		resourcesAreEqual(p.Resources.Requests, other.Resources.Requests, coreV1.ResourceMemory) &&
		resourcesAreEqual(p.Resources.Limits, other.Resources.Limits, coreV1.ResourceMemory) &&
		resourcesAreEqual(p.Resources.Requests, other.Resources.Requests, coreV1.ResourceCPU) &&
		resourcesAreEqual(p.Resources.Limits, other.Resources.Limits, coreV1.ResourceCPU)
}

func resourcesAreEqual(resources coreV1.ResourceList, otherResources coreV1.ResourceList, resourceName coreV1.ResourceName) bool {
	val, ok := resources[resourceName]
	otherVal, otherOk := otherResources[resourceName]

	if ok && otherOk {
		return val.Cmp(otherVal) == 0
	}
	return ok == otherOk
}

func sortedRacks(racks []Rack) []Rack {
	sort.SliceStable(racks, func(i, j int) bool {
		return racks[i].Name > racks[j].Name
	})
	return racks
}

func sortedStrings(strings []string) []string {
	sort.Strings(strings)
	return strings
}
