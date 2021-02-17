package cluster

import (
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	appsv1 "k8s.io/api/apps/v1beta2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"strings"
	"time"
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

func getExtraClassPathVar() v1.EnvVar {
	return v1.EnvVar{Name: "EXTRA_CLASSPATH", Value: "/extra-lib/cassandra-seed-provider.jar"}
}

func addCassEnvVarToEnvVars(cassEnvVar v1alpha1.CassEnvVar, envVars []v1.EnvVar, i int) {
	envVar := v1.EnvVar{}
	if cassEnvVar.ValueFrom != nil {
		envVar = v1.EnvVar{
			Name:      cassEnvVar.Name,
			ValueFrom: &v1.EnvVarSource{SecretKeyRef: &cassEnvVar.ValueFrom.SecretKeyRef},
		}
	} else {
		envVar = v1.EnvVar{Name: cassEnvVar.Name, Value: cassEnvVar.Value}
	}
	envVars[i] = envVar

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

func createAffinityRules(rack *v1alpha1.Rack, qualifiedName string) *v1.Affinity {
	affinity := v1.Affinity{
		PodAntiAffinity: &v1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							ApplicationInstanceLabel: qualifiedName,
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

func createPodVolumes(rack *v1alpha1.Rack) []v1.Volume {
	volumes := []v1.Volume{
		emptyDir(configurationVolumeName),
		emptyDir(extraLibVolumeName),
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

func createVolumeMounts(rack *v1alpha1.Rack) []v1.VolumeMount {
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

func createCustomConfigVolumeMount(customConfigMapVolumeName string) v1.VolumeMount {
	return v1.VolumeMount{
		Name:      customConfigMapVolumeName,
		MountPath: customConfigDir,
	}
}

func createConfigMapVolume(configMap *v1.ConfigMap, customConfigMapVolumeName string) v1.Volume {
	return v1.Volume{
		Name: customConfigMapVolumeName,
		VolumeSource: v1.VolumeSource{
			ConfigMap: &v1.ConfigMapVolumeSource{
				LocalObjectReference: v1.LocalObjectReference{
					Name: configMap.Name,
				},
			},
		},
	}
}

func hasConfigMapVolumeMount(container *v1.Container, volumeName string) bool {
	for _, mount := range container.VolumeMounts {
		if mount.Name == volumeName {
			return true
		}
	}
	return false
}

func hasConfigMapVolume(statefulSet *appsv1.StatefulSet, volumeName string) bool {
	for _, volume := range statefulSet.Spec.Template.Spec.Volumes {
		if volume.Name == volumeName {
			return true
		}
	}
	return false
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
