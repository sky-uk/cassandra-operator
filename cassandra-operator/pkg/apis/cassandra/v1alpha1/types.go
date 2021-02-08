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
// +kubebuilder:resource:scope=Namespaced
type Cassandra struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CassandraSpec   `json:"spec,omitempty"`
	Status CassandraStatus `json:"status,omitempty"`
}

// CassandraSpec is the specification for the Cassandra resource
type CassandraSpec struct {
	// The assigned datacenter name for the Cassandra cluster.
	// +optional
	Datacenter *string `json:"datacenter,omitempty"`
	Racks      []Rack  `json:"racks"`
	Pod        Pod     `json:"pod"`
	// +optional
	Snapshot *Snapshot `json:"snapshot,omitempty"`
}

// Probe represents a scheme for monitoring cassandra nodes status.
type Probe struct {
	// Minimum consecutive failures for the probe to be considered failed after having succeeded. Minimum value is 1.
	// +optional
	FailureThreshold *int32 `json:"failureThreshold,omitempty"`
	// Number of seconds after the container has started before probes are initiated.
	// +optional
	InitialDelaySeconds *int32 `json:"initialDelaySeconds,omitempty"`
	// How often (in seconds) to perform the probe. Minimum value is 1.
	// +optional
	PeriodSeconds *int32 `json:"periodSeconds,omitempty"`
	// Minimum consecutive successes for the probe to be considered successful after having failed.
	// +optional
	SuccessThreshold *int32 `json:"successThreshold,omitempty"`
	// Timeout for the probe.
	// +optional
	TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`
}

// Pod corresponds to a Cassandra node.
type Pod struct {
	// The name of the Docker image used to prepare the configuration for the Cassandra node before it can be started.
	// +optional
	BootstrapperImage *string `json:"bootstrapperImage,omitempty"`

	// The Docker image to run on each Cassandra node in the cluster. It is recommended to use one of the official Cassandra images, version 3
	// +optional
	Image     *string                     `json:"image,omitempty"`
	Resources coreV1.ResourceRequirements `json:"resources"`
	// Liveness probe for the cassandra container
	// +optional
	LivenessProbe *Probe `json:"livenessProbe,omitempty"`
	// Readiness probe for the cassandra container
	// +optional
	ReadinessProbe *Probe `json:"readinessProbe,omitempty"`
	// Env variables for the cassandra container
	// +optional
	Env *[]CassEnvVar `json:"env,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,7,rep,name=env"`
	// Sidecar container specification
	Sidecar *Sidecar `json:"sidecar"`
}

// EnvVar represents an environment variable the operator will add to the Cassandra Container
// This is almost identical to coreV1.EnvVar, but replaces the type of ValueFrom.
type CassEnvVar struct {
	// Name of the environment variable. Must be a C_IDENTIFIER.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`

	// Optional: no more than one of the following may be specified.

	// Variable references $(VAR_NAME) are expanded
	// using the previous defined environment variables in the container and
	// any service environment variables. If a variable cannot be resolved,
	// the reference in the input string will be unchanged. The $(VAR_NAME)
	// syntax can be escaped with a double $$, ie: $$(VAR_NAME). Escaped
	// references will never be expanded, regardless of whether the variable
	// exists or not.
	// Defaults to "".
	// +optional
	Value string `json:"value,omitempty" protobuf:"bytes,2,opt,name=value"`
	// Source for the environment variable's value. Cannot be used if value is not empty.
	// +optional
	ValueFrom *CassEnvVarSource `json:"valueFrom,omitempty" protobuf:"bytes,3,opt,name=valueFrom"`
}

// CassEnvVarSource represents a source for the value of an CassEnvVar.
// This is almost identical to coreV1.EnvVarSource, but is restricted to a required SecretKeyRef
type CassEnvVarSource struct {
	// Selects a key of a secret in the pod's namespace
	SecretKeyRef coreV1.SecretKeySelector `json:"secretKeyRef" protobuf:"bytes,4,name=secretKeyRef"`
}

// Sidecar is the specification of a sidecar attached to a Cassandra node
type Sidecar struct {
	// The Docker image for the sidecar container running on each Cassandra node to expose node status.
	// +optional
	Image *string `json:"image,omitempty"`
	// Resource requirements for the sidecar container
	Resources coreV1.ResourceRequirements `json:"resources"`
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

// Rack defines the rack topology in the cluster, where a rack has a number of replicas located in the same physical grouping (e.g. zone).
// At least one rack must be supplied.
type Rack struct {
	// Name of the rack.
	Name string `json:"name"`
	// Zone in which the rack resides.
	// This is set against the `failure-domain.beta.kubernetes.io/zone` value in the node affinity rule of the corresponding `StatefulSet`.
	Zone string `json:"zone"`
	// The desired number of replicas in the rack.
	Replicas int32 `json:"replicas"`
	// The rack storage options.
	Storage []Storage `json:"storage"`
}

// Storage defines the storage properties shared by pods in the same rack.
// Multiple volume types are available such as `emptyDir` and `persistentVolumeClaim`.
// The volume types available are expected to be a subset of the volume types defined in `k8s.io/api/core/v1/VolumeSource`.
// Only one type of volume may be specified.
type Storage struct {
	// The full path to the volume in the pod
	Path          *string `json:"path,omitempty"`
	StorageSource `json:",inline"`
}

// StorageSource represents the volume type to use as storage
// The volume types available are expected to be a subset of the volume types defined in 1k8s.io/api/core/v1/VolumeSource1.
// Only one of its members may be specified.
type StorageSource struct {
	// The volume as a persistent volume of type `k8s.io/api/core/v1/PersistentVolumeClaimSpec`
	// +optional
	PersistentVolumeClaim *coreV1.PersistentVolumeClaimSpec `json:"persistentVolumeClaim,omitempty"`
	// The volume as an empty directory of type `k8s.io/api/core/v1/EmptyDirVolumeSource`
	// +optional
	EmptyDir *coreV1.EmptyDirVolumeSource `json:"emptyDir,omitempty"`
}

// Snapshot defines the snapshot creation and deletion configuration
type Snapshot struct {
	// The name of the Docker image used to create and cleanup snapshots.
	// +optional
	Image *string `json:"image,omitempty"`
	// Crontab expression specifying when snapshots will be taken.
	// It follows the cron format, see https://en.wikipedia.org/wiki/Cron
	Schedule string `json:"schedule"`
	// List of keyspaces to snapshot. Leave empty to snapshot all keyspaces.
	// +optional
	Keyspaces []string `json:"keyspaces,omitempty"`
	// Time limit for the snapshot command to run.
	// +optional
	TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`
	// +optional
	RetentionPolicy *RetentionPolicy `json:"retentionPolicy,omitempty"`
	// Resource requirements for the Snapshot job
	Resources coreV1.ResourceRequirements `json:"resources"`
}

// RetentionPolicy defines how long the snapshots should be kept for and how often the cleanup task should run
type RetentionPolicy struct {
	// The period of time for which snapshots will be retained. Snapshots older than this period will be deleted.
	// +optional
	RetentionPeriodDays *int32 `json:"retentionPeriodDays,omitempty"`
	// Crontab expression specifying when snapshot cleanup will run.
	// It follows the cron format, see https://en.wikipedia.org/wiki/Cron
	CleanupSchedule string `json:"cleanupSchedule"`
	// Time limit for the cleanup command to run.
	// +optional
	CleanupTimeoutSeconds *int32 `json:"cleanupTimeoutSeconds,omitempty"`
	// Resource requirements for the Snapshot Cleanup Job
	Resources coreV1.ResourceRequirements `json:"resources"`
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

// Equal checks the equality of two CassEnvVar.
func (cev CassEnvVar) Equal(other CassEnvVar) bool {
	return cmp.Equal(cev.Name, other.Name) &&
		cmp.Equal(cev.Value, other.Value) &&
		reflect.DeepEqual(cev.ValueFrom, other.ValueFrom)
}

// Equal checks the equality of two CassEnvVarSource.
func (cevs CassEnvVarSource) Equal(other CassEnvVarSource) bool {
	return reflect.DeepEqual(cevs.SecretKeyRef, other.SecretKeyRef)
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
		cmp.Equal(s.RetentionPolicy, other.RetentionPolicy) &&
		allResourcesAreEqual(s.Resources, other.Resources)
}

// Equal checks equality of two RetentionPolicy. This is useful for checking equality with cmp.Equal
func (rp RetentionPolicy) Equal(other RetentionPolicy) bool {
	return reflect.DeepEqual(rp.CleanupTimeoutSeconds, other.CleanupTimeoutSeconds) &&
		reflect.DeepEqual(rp.CleanupSchedule, other.CleanupSchedule) &&
		reflect.DeepEqual(rp.RetentionPeriodDays, other.RetentionPeriodDays) &&
		allResourcesAreEqual(rp.Resources, other.Resources)
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
		reflect.DeepEqual(p.Sidecar, other.Sidecar) &&
		reflect.DeepEqual(p.Image, other.Image) &&
		reflect.DeepEqual(p.LivenessProbe, other.LivenessProbe) &&
		reflect.DeepEqual(p.ReadinessProbe, other.ReadinessProbe) &&
		allResourcesAreEqual(p.Resources, other.Resources) &&
		allEnvVarsEqual(p.Env, other.Env)
}

// Equal checks equality of two Sidecars. This is useful for checking equality with cmp.Equal
func (s Sidecar) Equal(other Sidecar) bool {
	return reflect.DeepEqual(s.Image, other.Image) &&
		allResourcesAreEqual(s.Resources, other.Resources)
}

func allResourcesAreEqual(aResource coreV1.ResourceRequirements, anotherResource coreV1.ResourceRequirements) bool {
	return resourcesAreEqual(aResource.Requests, anotherResource.Requests, coreV1.ResourceMemory) &&
		resourcesAreEqual(aResource.Limits, anotherResource.Limits, coreV1.ResourceMemory) &&
		resourcesAreEqual(aResource.Requests, anotherResource.Requests, coreV1.ResourceCPU) &&
		resourcesAreEqual(aResource.Limits, anotherResource.Limits, coreV1.ResourceCPU)
}

func isNilOrEmpty(envs *[]CassEnvVar) bool {
	if envs == nil {
		return true
	}
	if len(*envs) == 0 {
		return true
	}
	return false
}

func allEnvVarsEqual(envs *[]CassEnvVar, otherEnvs *[]CassEnvVar) bool {
	if isNilOrEmpty(envs) && isNilOrEmpty(otherEnvs) {
		return true
	}
	if isNilOrEmpty(envs) && !isNilOrEmpty(otherEnvs) {
		return false
	}
	if !isNilOrEmpty(envs) && isNilOrEmpty(otherEnvs) {
		return false
	}
	return reflect.DeepEqual(sortedEnvs(*envs), sortedEnvs(*otherEnvs))
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

func sortedEnvs(envs []CassEnvVar) []CassEnvVar {
	sort.SliceStable(envs, func(i, j int) bool {
		return envs[i].Name > envs[j].Name
	})
	return envs
}
