package cluster

import (
	"fmt"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/hash"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/imageversion"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/ptr"
	"regexp"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1beta2"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/api/batch/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	v1alpha1helpers "github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1/helpers"
)

const (
	// UserID is the ID of the operating system user which the various containers provisioned by the operator should
	// be run as.
	UserID = int64(999)

	// GroupID is the primary group of the user which runs the containers.
	GroupID = UserID

	// ApplicationNameLabel is a recommended Kubernetes label applied to all resources created by the operator.
	ApplicationNameLabel = "app.kubernetes.io/name"

	// ApplicationInstanceLabel is a recommended Kubernetes label applied to all resources created by the operator.
	ApplicationInstanceLabel = "app.kubernetes.io/instance"

	// ApplicationVersionLabel is a recommended Kubernetes label applied to all resources created by the operator.
	ApplicationVersionLabel = "app.kubernetes.io/version"

	// ApplicationComponentLabel is a recommended Kubernetes label applied to all resources created by the operator.
	ApplicationComponentLabel = "app.kubernetes.io/component"

	// ManagedByLabel is a recommended Kubernetes label applied to all resources created by the operator.
	ManagedByLabel = "app.kubernetes.io/managed-by"

	// ManagedByCassandraOperator is the fixed value for the app.kubernetes.io/managed-by label.
	ManagedByCassandraOperator = "cassandra-operator"

	// ConfigHashAnnotation gives the name of the annotation that the operator attaches to pods when they have
	// an associated custom config map.
	ConfigHashAnnotation = "clusterConfigHash"

	// SnapshotCronJob is the value of app.kubernetes.io/component for snapshot cronjobs
	SnapshotCronJob = "snapshot"

	// SnapshotCleanupCronJob is the value of app.kubernetes.io/component for snapshot cleanup cronjobs
	SnapshotCleanupCronJob = "snapshot-cleanup"

	// RackLabel is a label used to identify the rack name in a cluster
	RackLabel       = "cassandra-operator/rack"
	customConfigDir = "/custom-config"

	cassandraContainerName             = "cassandra"
	cassandraBootstrapperContainerName = "cassandra-bootstrapper"
	cassandraSidecarContainerName      = "cassandra-sidecar"

	configurationVolumeName = "configuration"
	extraLibVolumeName      = "extra-lib"

	healthServerPort = 8080
)

var (
	maxSidecarMemoryRequest    resource.Quantity
	sidecarMemoryLimit         resource.Quantity
	maxSidecarCPURequest       resource.Quantity
	initContainerMemoryRequest resource.Quantity
	operatorManagedVolumes     map[string]bool
	alphanumericChars          *regexp.Regexp
	securityContext            *v1.PodSecurityContext
)

func init() {
	initContainerMemoryRequest = resource.MustParse("100Mi")
	maxSidecarMemoryRequest = resource.MustParse("50Mi")
	sidecarMemoryLimit = resource.MustParse("50Mi")
	maxSidecarCPURequest = resource.MustParse("100m")
	operatorManagedVolumes = make(map[string]bool)
	operatorManagedVolumes[configurationVolumeName] = true
	operatorManagedVolumes[extraLibVolumeName] = true
	alphanumericChars = regexp.MustCompile("[^a-zA-Z0-9]+")
	securityContext = &v1.PodSecurityContext{
		RunAsUser:  ptr.Int64(UserID),
		RunAsGroup: ptr.Int64(GroupID),
		FSGroup:    ptr.Int64(GroupID),
	}
}

// Cluster defines the properties of a Cassandra cluster which the operator should manage.
type Cluster struct {
	definition *v1alpha1.Cassandra
	Online     bool
}

// New creates a new cluster definition from the supplied Cassandra definition
func New(clusterDefinition *v1alpha1.Cassandra) *Cluster {
	cluster := &Cluster{}
	CopyInto(cluster, clusterDefinition)
	return cluster
}

// CopyInto copies a Cassandra cluster definition into the internal cluster data structure supplied.
func CopyInto(cluster *Cluster, clusterDefinition *v1alpha1.Cassandra) {
	cluster.definition = clusterDefinition.DeepCopy()
}

// Definition returns a copy of the definition of the cluster. Any modifications made to this will be ignored.
func (c *Cluster) Definition() *v1alpha1.Cassandra {
	return c.definition.DeepCopy()
}

// Name is the unqualified name of the cluster
func (c *Cluster) Name() string {
	return c.definition.Name
}

// Namespace is the namespace the cluster resides in
func (c *Cluster) Namespace() string {
	return c.definition.Namespace
}

// QualifiedName is the namespace-qualified name of the cluster
func (c *Cluster) QualifiedName() string {
	return c.definition.QualifiedName()
}

// ApplicationVersion returns the version part of the main Cassandra image name
func (c *Cluster) ApplicationVersion() string {
	return imageversion.Version(c.definition.Spec.Pod.Image)
}

// Racks returns the set of racks defined for the cluster
func (c *Cluster) Racks() []v1alpha1.Rack {
	return c.definition.Spec.Racks
}

// CreateStatefulSetForRack creates a StatefulSet based on the rack details
func (c *Cluster) CreateStatefulSetForRack(rack *v1alpha1.Rack, customConfigMap *v1.ConfigMap) *appsv1.StatefulSet {
	sts := &appsv1.StatefulSet{
		ObjectMeta: c.objectMetadataWithOwner(c.definition.RackName(rack), RackLabel, rack.Name),
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					ApplicationInstanceLabel: c.QualifiedName(),
					ManagedByLabel:           ManagedByCassandraOperator,
					RackLabel:                rack.Name,
				},
			},
			Replicas:    &rack.Replicas,
			ServiceName: c.definition.Name,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						ApplicationNameLabel:     c.Name(),
						ApplicationInstanceLabel: c.QualifiedName(),
						ApplicationVersionLabel:  imageversion.Version(c.definition.Spec.Pod.Image),
						ManagedByLabel:           ManagedByCassandraOperator,
						RackLabel:                rack.Name,
					},
				},
				Spec: v1.PodSpec{
					ServiceAccountName: v1alpha1.NodeServiceAccountName,
					InitContainers: []v1.Container{
						c.createInitConfigContainer(),
						c.createCassandraBootstrapperContainer(rack),
					},
					SecurityContext: securityContext,
					Containers: []v1.Container{
						c.createCassandraContainer(rack),
						c.createCassandraSidecarContainer(rack),
					},
					Volumes:  c.createPodVolumes(rack),
					Affinity: c.createAffinityRules(rack),
				},
			},

			VolumeClaimTemplates: c.createCassandraDataPersistentVolumeClaimForRack(rack),
		},
	}

	if customConfigMap != nil {
		c.AddCustomConfigVolumeToStatefulSet(sts, rack, customConfigMap)
	}

	return sts
}

// UpdateStatefulSetToDesiredState updates the current statefulSet to the desired state based on the desired rack definition
func (c *Cluster) UpdateStatefulSetToDesiredState(currentStatefulSet *appsv1.StatefulSet, targetRack *v1alpha1.Rack, customConfigMap *v1.ConfigMap) {
	desiredStatefulSet := c.CreateStatefulSetForRack(targetRack, customConfigMap)
	currentStatefulSet.Spec = desiredStatefulSet.Spec
}

func (c *Cluster) createAffinityRules(rack *v1alpha1.Rack) *v1.Affinity {
	affinity := v1.Affinity{
		PodAntiAffinity: &v1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							ApplicationInstanceLabel: c.QualifiedName(),
							ManagedByLabel:           ManagedByCassandraOperator,
						},
					},
					TopologyKey: "kubernetes.io/hostname",
				},
			},
		},
		NodeAffinity: &v1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
				NodeSelectorTerms: []v1.NodeSelectorTerm{
					{
						MatchExpressions: []v1.NodeSelectorRequirement{
							{
								Key:      "failure-domain.beta.kubernetes.io/zone",
								Operator: v1.NodeSelectorOpIn,
								Values:   []string{rack.Zone},
							},
						},
					},
				},
			},
		},
	}
	return &affinity
}

// CreateService creates a headless service for the supplied cluster definition.
func (c *Cluster) CreateService() *v1.Service {
	return &v1.Service{
		ObjectMeta: c.objectMetadataWithOwner(c.definition.Name),
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				ApplicationInstanceLabel: c.QualifiedName(),
				ManagedByLabel:           ManagedByCassandraOperator,
			},
			ClusterIP: v1.ClusterIPNone,
			Ports: []v1.ServicePort{
				{
					Name:       "cassandra",
					Port:       9042,
					TargetPort: intstr.FromInt(9042),
				},
				{
					Name:       "jolokia",
					Port:       7777,
					TargetPort: intstr.FromInt(7777),
				},
			},
		},
	}
}

// SetDefaults uses the helper to set defaults on the cluster
func (c *Cluster) SetDefaults() {
	v1alpha1helpers.SetDefaultsForCassandra(c.definition, nil)
}

// CreateSnapshotJob creates a cronjob to trigger the creation of a snapshot
func (c *Cluster) CreateSnapshotJob() *v1beta1.CronJob {
	if c.definition.Spec.Snapshot == nil {
		return nil
	}

	return c.createCronJob(
		c.definition.SnapshotJobName(),
		"snapshot",
		v1alpha1.SnapshotServiceAccountName,
		c.definition.Spec.Snapshot.Schedule,
		c.CreateSnapshotContainer(),
	)
}

// CreateSnapshotContainer creates the container used to trigger the snapshot creation
func (c *Cluster) CreateSnapshotContainer() *v1.Container {
	return &v1.Container{
		Name:      c.definition.SnapshotJobName(),
		Image:     *c.definition.Spec.Snapshot.Image,
		Command:   c.snapshotCommand(),
		Resources: c.definition.Spec.Snapshot.Resources,
	}
}

func (c *Cluster) snapshotCommand() []string {
	if c.definition.Spec.Snapshot == nil {
		return []string{}
	}
	backupCommand := []string{"/cassandra-snapshot", "create",
		"-n", c.definition.Namespace,
		"-l", c.CassandraPodSelector(),
	}
	timeoutDuration := durationSeconds(c.definition.Spec.Snapshot.TimeoutSeconds)
	backupCommand = append(backupCommand, "-t", timeoutDuration.String())
	if len(c.definition.Spec.Snapshot.Keyspaces) > 0 {
		backupCommand = append(backupCommand, "-k")
		backupCommand = append(backupCommand, strings.Join(c.definition.Spec.Snapshot.Keyspaces, ","))
	}
	return backupCommand
}

// CassandraPodSelector generates a label selector expression which will include all Cassandra pods belonging to a
// cluster, while excluding snapshot or snapshot-cleanup pods.
func (c *Cluster) CassandraPodSelector() string {
	return fmt.Sprintf("%s=%s,%s=%s,!%s",
		ApplicationInstanceLabel, c.QualifiedName(),
		ManagedByLabel, ManagedByCassandraOperator,
		ApplicationComponentLabel,
	)
}

// CreateSnapshotCleanupJob creates a cronjob to trigger the snapshot cleanup
func (c *Cluster) CreateSnapshotCleanupJob() *v1beta1.CronJob {
	if c.definition.Spec.Snapshot == nil ||
		c.definition.Spec.Snapshot.RetentionPolicy == nil {
		return nil
	}

	return c.createCronJob(
		c.definition.SnapshotCleanupJobName(),
		SnapshotCleanupCronJob,
		v1alpha1.SnapshotServiceAccountName,
		c.definition.Spec.Snapshot.RetentionPolicy.CleanupSchedule,
		c.CreateSnapshotCleanupContainer(),
	)
}

// CreateSnapshotCleanupContainer creates the container that will execute the snapshot cleanup command
func (c *Cluster) CreateSnapshotCleanupContainer() *v1.Container {
	return &v1.Container{
		Name:      c.definition.SnapshotCleanupJobName(),
		Image:     *c.definition.Spec.Snapshot.Image,
		Command:   c.snapshotCleanupCommand(),
		Resources: c.definition.Spec.Snapshot.RetentionPolicy.Resources,
	}
}

func (c *Cluster) snapshotCleanupCommand() []string {
	if c.definition.Spec.Snapshot == nil || c.definition.Spec.Snapshot.RetentionPolicy == nil {
		return []string{}
	}
	cleanupCommand := []string{"/cassandra-snapshot", "cleanup",
		"-n", c.Namespace(),
		"-l", c.CassandraPodSelector(),
	}
	retentionPeriodDuration := durationDays(c.definition.Spec.Snapshot.RetentionPolicy.RetentionPeriodDays)
	cleanupCommand = append(cleanupCommand, "-r", retentionPeriodDuration.String())
	cleanupTimeoutDuration := durationSeconds(c.definition.Spec.Snapshot.RetentionPolicy.CleanupTimeoutSeconds)
	cleanupCommand = append(cleanupCommand, "-t", cleanupTimeoutDuration.String())
	return cleanupCommand
}

func (c *Cluster) createCronJob(objectName, componentName, serviceAccountName, schedule string, container *v1.Container) *v1beta1.CronJob {
	return &v1beta1.CronJob{
		ObjectMeta: c.objectMetadataWithOwner(objectName, ApplicationComponentLabel, componentName),
		Spec: v1beta1.CronJobSpec{
			Schedule:          schedule,
			ConcurrencyPolicy: v1beta1.ForbidConcurrent,
			JobTemplate: v1beta1.JobTemplateSpec{
				ObjectMeta: c.objectMetadataWithOwner(objectName, ApplicationComponentLabel, componentName),
				Spec: batchv1.JobSpec{
					Template: v1.PodTemplateSpec{
						ObjectMeta: c.objectMetadataWithOwner(objectName, ApplicationComponentLabel, componentName),
						Spec: v1.PodSpec{
							SecurityContext:    securityContext,
							RestartPolicy:      v1.RestartPolicyOnFailure,
							ServiceAccountName: serviceAccountName,
							Containers:         []v1.Container{*container},
						},
					},
				},
			},
		},
	}
}

func (c *Cluster) objectMetadata(name string, extraLabels ...string) metav1.ObjectMeta {
	labels := map[string]string{
		ApplicationNameLabel:     c.Name(),
		ApplicationInstanceLabel: c.QualifiedName(),
		ManagedByLabel:           ManagedByCassandraOperator,
	}

	imageName := c.definition.Spec.Pod.Image
	if imageName != nil {
		imageVersion := c.ApplicationVersion()
		labels[ApplicationVersionLabel] = imageVersion
	}

	for i := 0; i < len(extraLabels)-1; i += 2 {
		labels[extraLabels[i]] = extraLabels[i+1]
	}

	return metav1.ObjectMeta{
		Name:      name,
		Namespace: c.Namespace(),
		Labels:    labels,
	}
}

func (c *Cluster) objectMetadataWithOwner(name string, extraLabels ...string) metav1.ObjectMeta {
	meta := c.objectMetadata(name, extraLabels...)
	meta.OwnerReferences = []metav1.OwnerReference{v1alpha1helpers.NewControllerRef(c.definition)}
	return meta
}

func (c *Cluster) createCassandraContainer(rack *v1alpha1.Rack) v1.Container {
	return v1.Container{
		Name:  cassandraContainerName,
		Image: *c.definition.Spec.Pod.Image,
		Ports: []v1.ContainerPort{
			{
				Name:          "internode",
				Protocol:      v1.ProtocolTCP,
				ContainerPort: 7000,
			},
			{
				Name:          "jmx-exporter",
				Protocol:      v1.ProtocolTCP,
				ContainerPort: 7070,
			},
			{
				Name:          "cassandra-jmx",
				Protocol:      v1.ProtocolTCP,
				ContainerPort: 7199,
			},
			{
				Name:          "jolokia",
				Protocol:      v1.ProtocolTCP,
				ContainerPort: 7777,
			},
			{
				Name:          "client",
				Protocol:      v1.ProtocolTCP,
				ContainerPort: 9042,
			},
		},
		Resources:      c.definition.Spec.Pod.Resources,
		LivenessProbe:  createHTTPProbe(c.definition.Spec.Pod.LivenessProbe, "/live", healthServerPort),
		ReadinessProbe: createHTTPProbe(c.definition.Spec.Pod.ReadinessProbe, "/ready", healthServerPort),
		Lifecycle: &v1.Lifecycle{
			PreStop: &v1.Handler{
				Exec: &v1.ExecAction{
					Command: []string{"/bin/sh", "-c", "nodetool drain"},
				},
			},
		},
		Env: []v1.EnvVar{
			{Name: "EXTRA_CLASSPATH", Value: "/extra-lib/cassandra-seed-provider.jar"},
		},
		VolumeMounts: c.createVolumeMounts(rack),
	}
}

func (c *Cluster) createCassandraSidecarContainer(rack *v1alpha1.Rack) v1.Container {
	return v1.Container{
		Name:  cassandraSidecarContainerName,
		Image: *c.definition.Spec.Pod.SidecarImage,
		Ports: []v1.ContainerPort{
			{
				Name:          "api",
				Protocol:      v1.ProtocolTCP,
				ContainerPort: healthServerPort,
			},
		},
		Env: c.createEnvironmentVariableDefinition(rack),
		Args: []string{
			"--log-level=info",
			fmt.Sprintf("--health-server-port=%d", healthServerPort),
		},
		Resources: v1.ResourceRequirements{
			Requests: v1.ResourceList{
				v1.ResourceCPU: minQuantity(
					*c.definition.Spec.Pod.Resources.Requests.Cpu(),
					maxSidecarCPURequest,
				),
				v1.ResourceMemory: minQuantity(
					*c.definition.Spec.Pod.Resources.Requests.Memory(),
					maxSidecarMemoryRequest,
				),
			},
			Limits: v1.ResourceList{
				v1.ResourceMemory: sidecarMemoryLimit,
			},
		},
	}
}

func (c *Cluster) createEnvironmentVariableDefinition(rack *v1alpha1.Rack) []v1.EnvVar {
	envVariables := []v1.EnvVar{
		{
			Name:  "CLUSTER_NAMESPACE",
			Value: c.definition.Namespace,
		},
		{
			Name:  "CLUSTER_NAME",
			Value: c.definition.Name,
		},
		{
			Name:  "CLUSTER_CURRENT_RACK",
			Value: rack.Name,
		},
		{
			Name:  "CLUSTER_DATA_CENTER",
			Value: *c.definition.Spec.Datacenter,
		},
		{
			Name: "NODE_LISTEN_ADDRESS",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "status.podIP",
				},
			},
		},
		{
			Name:  "POD_CPU_MILLICORES",
			Value: fmt.Sprintf("%d", c.definition.Spec.Pod.Resources.Requests.Cpu().MilliValue()),
		},
		{
			Name:  "POD_MEMORY_BYTES",
			Value: fmt.Sprintf("%d", c.definition.Spec.Pod.Resources.Requests.Memory().Value()),
		},
	}

	return envVariables
}

func (c *Cluster) createCassandraDataPersistentVolumeClaimForRack(rack *v1alpha1.Rack) []v1.PersistentVolumeClaim {
	var persistentVolumeClaim []v1.PersistentVolumeClaim

	for i := range rack.Storage {
		cassandraStorage := rack.Storage[i]
		if cassandraStorage.PersistentVolumeClaim != nil {
			persistentVolumeClaim = append(persistentVolumeClaim, v1.PersistentVolumeClaim{
				ObjectMeta: c.objectMetadata(resourceNameFromPath(*cassandraStorage.Path), RackLabel, rack.Name),
				Spec:       *cassandraStorage.StorageSource.PersistentVolumeClaim,
			})
		}
	}
	return persistentVolumeClaim
}

func (c *Cluster) createVolumeMounts(rack *v1alpha1.Rack) []v1.VolumeMount {
	mounts := []v1.VolumeMount{
		{Name: configurationVolumeName, MountPath: v1alpha1.ConfigurationVolumeMountPath},
		{Name: extraLibVolumeName, MountPath: v1alpha1.ExtraLibVolumeMountPath},
	}

	for i := range rack.Storage {
		mounts = append(mounts, v1.VolumeMount{
			Name:      resourceNameFromPath(*rack.Storage[i].Path),
			MountPath: *rack.Storage[i].Path,
		})
	}

	return mounts
}

func (c *Cluster) createCustomConfigVolumeMount() v1.VolumeMount {
	return v1.VolumeMount{
		Name:      c.customConfigMapVolumeName(),
		MountPath: customConfigDir,
	}
}

func (c *Cluster) createPodVolumes(rack *v1alpha1.Rack) []v1.Volume {
	volumes := []v1.Volume{
		emptyDir("configuration"),
		emptyDir("extra-lib"),
	}

	for i := range rack.Storage {
		if rack.Storage[i].EmptyDir != nil {
			volumes = append(volumes, emptyDir(resourceNameFromPath(*rack.Storage[i].Path)))
		}
	}
	return volumes
}

func emptyDir(name string) v1.Volume {
	return v1.Volume{
		Name:         name,
		VolumeSource: v1.VolumeSource{EmptyDir: &v1.EmptyDirVolumeSource{}},
	}
}

func (c *Cluster) createConfigMapVolume(configMap *v1.ConfigMap) v1.Volume {
	return v1.Volume{
		Name: c.customConfigMapVolumeName(),
		VolumeSource: v1.VolumeSource{
			ConfigMap: &v1.ConfigMapVolumeSource{
				LocalObjectReference: v1.LocalObjectReference{
					Name: configMap.Name,
				},
			},
		},
	}
}

func createHTTPProbe(probe *v1alpha1.Probe, path string, port int) *v1.Probe {
	return &v1.Probe{
		Handler: v1.Handler{
			HTTPGet: &v1.HTTPGetAction{
				Port: intstr.FromInt(port),
				Path: path,
			},
		},
		InitialDelaySeconds: *probe.InitialDelaySeconds,
		PeriodSeconds:       *probe.PeriodSeconds,
		TimeoutSeconds:      *probe.TimeoutSeconds,
		FailureThreshold:    *probe.FailureThreshold,
		SuccessThreshold:    *probe.SuccessThreshold,
	}
}

// AddCustomConfigVolumeToStatefulSet updates the provided statefulset to mount the configmap as a volume
func (c *Cluster) AddCustomConfigVolumeToStatefulSet(statefulSet *appsv1.StatefulSet, _ *v1alpha1.Rack, customConfigMap *v1.ConfigMap) {
	if statefulSet.Spec.Template.Annotations == nil {
		statefulSet.Spec.Template.Annotations = map[string]string{}
	}
	statefulSet.Spec.Template.Annotations[ConfigHashAnnotation] = hash.ConfigMapHash(customConfigMap)

	if !c.hasConfigMapVolume(statefulSet) {
		statefulSet.Spec.Template.Spec.Volumes = append(statefulSet.Spec.Template.Spec.Volumes, c.createConfigMapVolume(customConfigMap))
	}

	for i := range statefulSet.Spec.Template.Spec.InitContainers {
		if statefulSet.Spec.Template.Spec.InitContainers[i].Name == cassandraBootstrapperContainerName &&
			!c.hasConfigMapVolumeMount(&statefulSet.Spec.Template.Spec.InitContainers[i]) {
			statefulSet.Spec.Template.Spec.InitContainers[i].VolumeMounts = append(statefulSet.Spec.Template.Spec.InitContainers[i].VolumeMounts, c.createCustomConfigVolumeMount())
		}
	}
}

func (c *Cluster) hasConfigMapVolumeMount(container *v1.Container) bool {
	for _, mount := range container.VolumeMounts {
		if mount.Name == c.customConfigMapVolumeName() {
			return true
		}
	}
	return false
}

func (c *Cluster) hasConfigMapVolume(statefulSet *appsv1.StatefulSet) bool {
	for _, volume := range statefulSet.Spec.Template.Spec.Volumes {
		if volume.Name == c.customConfigMapVolumeName() {
			return true
		}
	}
	return false
}

// RemoveCustomConfigVolumeFromStatefulSet updates the provided statefulset to unmount the configmap as a volume
func (c *Cluster) RemoveCustomConfigVolumeFromStatefulSet(statefulSet *appsv1.StatefulSet, _ *v1alpha1.Rack, _ *v1.ConfigMap) {
	var volumesAfterRemoval []v1.Volume
	for _, volume := range statefulSet.Spec.Template.Spec.Volumes {
		if volume.Name != c.customConfigMapVolumeName() {
			volumesAfterRemoval = append(volumesAfterRemoval, volume)
		}
	}
	statefulSet.Spec.Template.Spec.Volumes = volumesAfterRemoval

	delete(statefulSet.Spec.Template.Annotations, ConfigHashAnnotation)

	var volumesMountAfterRemoval []v1.VolumeMount
	for i := range statefulSet.Spec.Template.Spec.InitContainers {
		if statefulSet.Spec.Template.Spec.InitContainers[i].Name == cassandraBootstrapperContainerName {
			for _, volumeMount := range statefulSet.Spec.Template.Spec.InitContainers[i].VolumeMounts {
				if volumeMount.Name != c.customConfigMapVolumeName() {
					volumesMountAfterRemoval = append(volumesMountAfterRemoval, volumeMount)
				}
			}
			statefulSet.Spec.Template.Spec.InitContainers[i].VolumeMounts = volumesMountAfterRemoval
		}
	}
}

func (c *Cluster) customConfigMapVolumeName() string {
	return fmt.Sprintf("cassandra-custom-config-%s", c.definition.Name)
}
func (c *Cluster) createInitConfigContainer() v1.Container {
	memory := minQuantity(*c.definition.Spec.Pod.Resources.Requests.Memory(), initContainerMemoryRequest)

	return v1.Container{
		Name:    "init-config",
		Image:   *c.definition.Spec.Pod.Image,
		Command: []string{"sh", "-c", "cp -vr /etc/cassandra/* /configuration"},
		VolumeMounts: []v1.VolumeMount{
			{Name: "configuration", MountPath: "/configuration"},
		},
		Resources: v1.ResourceRequirements{
			Requests: v1.ResourceList{
				v1.ResourceMemory: memory,
			},
			Limits: v1.ResourceList{
				v1.ResourceMemory: memory,
			},
		},
	}
}

func (c *Cluster) createCassandraBootstrapperContainer(rack *v1alpha1.Rack) v1.Container {
	memory := minQuantity(*c.definition.Spec.Pod.Resources.Requests.Memory(), initContainerMemoryRequest)

	mounts := []v1.VolumeMount{
		{Name: "configuration", MountPath: "/configuration"},
		{Name: "extra-lib", MountPath: "/extra-lib"},
	}

	return v1.Container{
		Name:  cassandraBootstrapperContainerName,
		Env:   c.createEnvironmentVariableDefinition(rack),
		Image: *c.definition.Spec.Pod.BootstrapperImage,
		Resources: v1.ResourceRequirements{
			Requests: v1.ResourceList{
				v1.ResourceMemory: memory,
			},
			Limits: v1.ResourceList{
				v1.ResourceMemory: memory,
			},
		},
		VolumeMounts: mounts,
	}
}

// ConfigMapBelongsToAManagedCluster determines whether the supplied ConfigMap belongs to a managed cluster
func ConfigMapBelongsToAManagedCluster(managedClusters map[string]*Cluster, configMap *v1.ConfigMap) bool {
	for _, mc := range managedClusters {
		if configMap.Name == mc.definition.CustomConfigMapName() {
			return true
		}
	}
	return false
}

// LooksLikeACassandraConfigMap determines whether the supplied ConfigMap could belong to a managed cluster
func LooksLikeACassandraConfigMap(configMap *v1.ConfigMap) bool {
	return strings.HasSuffix(configMap.Name, "-config")
}

func durationDays(days *int32) time.Duration {
	return time.Duration(*days) * time.Hour * 24
}

func durationSeconds(seconds *int32) time.Duration {
	return time.Duration(*seconds) * time.Second
}

func minQuantity(r1, r2 resource.Quantity) resource.Quantity {
	d := r1.Cmp(r2)
	if d > 0 {
		return r2
	}
	return r1
}

func resourceNameFromPath(path string) string {
	return alphanumericChars.ReplaceAllString(strings.TrimPrefix(path, "/"), "-")
}
