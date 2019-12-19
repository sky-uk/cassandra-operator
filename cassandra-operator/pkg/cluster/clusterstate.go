package cluster

import (
	"context"
	"fmt"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/ptr"
	"k8s.io/api/apps/v1beta2"
	"k8s.io/apimachinery/pkg/api/errors"
	"sort"
	"strings"

	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type clusterStateFinder interface {
	findClusterStateFor(desiredCassandra *v1alpha1.Cassandra) (*v1alpha1.Cassandra, error)
	findStatefulSetsFor(desiredCassandra *v1alpha1.Cassandra) (*v1beta2.StatefulSetList, error)
}

// implements clusterStateFinder
type currentClusterStateFinder struct {
	client        client.Client
	objectFactory objectReferenceFactory
}

func (cass *currentClusterStateFinder) findClusterStateFor(desiredCassandra *v1alpha1.Cassandra) (*v1alpha1.Cassandra, error) {
	statefulSets, err := cass.findStatefulSetsFor(desiredCassandra)
	if err != nil {
		return nil, err
	}

	var racks []v1alpha1.Rack
	for _, statefulSet := range statefulSets.Items {
		rack, err := cass.buildRackSpecFrom(&statefulSet, desiredCassandra)
		if err != nil {
			return nil, err
		}
		racks = append(racks, rack)
	}

	// pick any rack as this information is common
	anyRack := &statefulSets.Items[0]
	pod, err := cass.buildPodSpecFrom(anyRack, desiredCassandra)
	if err != nil {
		return nil, err
	}
	datacenter, err := cass.datacenterFrom(anyRack)
	if err != nil {
		return nil, err
	}

	cassandra := &v1alpha1.Cassandra{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: desiredCassandra.Namespace,
			Name:      desiredCassandra.Name,
		},
		Spec: v1alpha1.CassandraSpec{
			Datacenter: ptr.String(datacenter),
			Racks:      racks,
			Pod:        *pod,
		},
	}
	return cassandra, nil
}

func (cass *currentClusterStateFinder) findStatefulSetsFor(desiredCassandra *v1alpha1.Cassandra) (*v1beta2.StatefulSetList, error) {
	statefulSets := cass.objectFactory.newStatefulSetList()
	options := []client.ListOption{
		client.InNamespace(desiredCassandra.Namespace),
		client.MatchingLabels(map[string]string{ApplicationNameLabel: desiredCassandra.Name, ManagedByLabel: ManagedByCassandraOperator}),
	}
	err := cass.client.List(context.TODO(), statefulSets, options...)
	if err != nil {
		return nil, err
	} else if len(statefulSets.Items) == 0 {
		return nil, errors.NewNotFound(v1beta2.Resource("statefulset"), desiredCassandra.Name)
	}
	sort.SliceStable(statefulSets.Items, func(i, j int) bool {
		return statefulSets.Items[i].Name < statefulSets.Items[j].Name
	})
	return statefulSets, nil
}

func (cass *currentClusterStateFinder) buildPodSpecFrom(statefulSet *v1beta2.StatefulSet, desiredCassandra *v1alpha1.Cassandra) (*v1alpha1.Pod, error) {
	cassandraContainer, err := cass.containerWithName(cassandraContainerName, statefulSet)
	if err != nil {
		return nil, err
	}

	cassandraSideCarContainer, err := cass.containerWithName(cassandraSidecarContainerName, statefulSet)
	if err != nil {
		return nil, err
	}

	bootstrapperContainer, err := cass.initContainerWithName(cassandraBootstrapperContainerName, statefulSet)
	if err != nil {
		return nil, err
	}

	return &v1alpha1.Pod{
		Image:             ptr.String(cassandraContainer.Image),
		SidecarImage:      ptr.String(cassandraSideCarContainer.Image),
		BootstrapperImage: ptr.String(bootstrapperContainer.Image),
		Resources:         cassandraContainer.Resources,
		ReadinessProbe:    cass.buildProbeFrom(cassandraContainer.ReadinessProbe),
		LivenessProbe:     cass.buildProbeFrom(cassandraContainer.LivenessProbe),
	}, nil
}

func (cass *currentClusterStateFinder) buildRackSpecFrom(statefulSet *v1beta2.StatefulSet, desiredCassandra *v1alpha1.Cassandra) (v1alpha1.Rack, error) {
	rackName := strings.TrimPrefix(statefulSet.Name, fmt.Sprintf("%s-", desiredCassandra.Name))
	rackStorages, err := cass.buildCassandraStorage(statefulSet)
	if err != nil {
		return v1alpha1.Rack{}, err
	}

	return v1alpha1.Rack{
		Name:     rackName,
		Replicas: *statefulSet.Spec.Replicas,
		Zone:     cass.zoneAffinity(statefulSet),
		Storage:  rackStorages,
	}, nil
}

func (cass *currentClusterStateFinder) datacenterFrom(statefulSet *v1beta2.StatefulSet) (string, error) {
	bootstrapperContainer, err := cass.initContainerWithName(cassandraBootstrapperContainerName, statefulSet)
	if err != nil {
		return "", err
	}

	for _, env := range bootstrapperContainer.Env {
		if env.Name == "CLUSTER_DATA_CENTER" {
			return env.Value, nil
		}
	}
	return "", fmt.Errorf("no CLUSTER_DATA_CENTER env variable found in container %s", bootstrapperContainer.Name)
}

func (cass *currentClusterStateFinder) zoneAffinity(statefulSet *v1beta2.StatefulSet) string {
	affinity := statefulSet.Spec.Template.Spec.Affinity
	if affinity != nil && affinity.NodeAffinity != nil && affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		nodeSelectors := affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
		for _, selector := range nodeSelectors {
			for _, expr := range selector.MatchExpressions {
				if expr.Key == "failure-domain.beta.kubernetes.io/zone" {
					return strings.Join(expr.Values, "")
				}
			}
		}
	}
	return ""
}

func (cass *currentClusterStateFinder) containerWithName(name string, statefulSet *v1beta2.StatefulSet) (*corev1.Container, error) {
	for _, container := range statefulSet.Spec.Template.Spec.Containers {
		if container.Name == name {
			return &container, nil
		}
	}
	return nil, fmt.Errorf("no container with name %s found in statefulset %s", name, statefulSet.Name)
}

func (cass *currentClusterStateFinder) initContainerWithName(name string, statefulSet *v1beta2.StatefulSet) (*corev1.Container, error) {
	for _, container := range statefulSet.Spec.Template.Spec.InitContainers {
		if container.Name == name {
			return &container, nil
		}
	}
	return nil, fmt.Errorf("no init container with name %s found in statefulset %s", name, statefulSet.Name)
}

func (cass *currentClusterStateFinder) buildProbeFrom(probe *corev1.Probe) *v1alpha1.Probe {
	if probe == nil {
		return &v1alpha1.Probe{}
	}
	return &v1alpha1.Probe{
		InitialDelaySeconds: ptr.Int32(probe.InitialDelaySeconds),
		PeriodSeconds:       ptr.Int32(probe.PeriodSeconds),
		TimeoutSeconds:      ptr.Int32(probe.TimeoutSeconds),
		FailureThreshold:    ptr.Int32(probe.FailureThreshold),
		SuccessThreshold:    ptr.Int32(probe.SuccessThreshold),
	}
}

func (cass *currentClusterStateFinder) buildCassandraStorage(statefulSet *v1beta2.StatefulSet) ([]v1alpha1.Storage, error) {
	var cassandraStorages []v1alpha1.Storage
	cassandraContainer, err := cass.containerWithName(cassandraContainerName, statefulSet)
	if err != nil {
		return nil, err
	}

	volumeMountsInSpec := filterOutOperatorManagedVolumes(cassandraContainer.VolumeMounts)
	for _, volumeMount := range volumeMountsInSpec {
		storageSource := cass.buildStorageSource(volumeMount.Name, statefulSet)
		if storageSource == nil {
			return nil, fmt.Errorf("no supported volume source found for volume mount: %v", volumeMount.Name)
		}

		cassandraStorages = append(cassandraStorages, v1alpha1.Storage{
			Path:          ptr.String(volumeMount.MountPath),
			StorageSource: *storageSource,
		})
	}
	return cassandraStorages, nil
}

func filterOutOperatorManagedVolumes(mounts []corev1.VolumeMount) []corev1.VolumeMount {
	var volumesToKeep []corev1.VolumeMount
	for _, volumeMount := range mounts {
		if _, ok := operatorManagedVolumes[volumeMount.Name]; !ok {
			volumesToKeep = append(volumesToKeep, volumeMount)
		}
	}
	return volumesToKeep
}

func (cass *currentClusterStateFinder) buildStorageSource(volumeName string, statefulSet *v1beta2.StatefulSet) *v1alpha1.StorageSource {
	if volumeClaim := cass.volumeClaimWithName(volumeName, statefulSet); volumeClaim != nil {
		return &v1alpha1.StorageSource{
			PersistentVolumeClaim: &volumeClaim.Spec,
		}
	}
	if emptyDirVolume := cass.emptyDirVolumeWithName(volumeName, statefulSet); emptyDirVolume != nil {
		return &v1alpha1.StorageSource{
			EmptyDir: emptyDirVolume.EmptyDir,
		}
	}
	return nil
}

func (cass *currentClusterStateFinder) volumeClaimWithName(name string, statefulSet *v1beta2.StatefulSet) *corev1.PersistentVolumeClaim {
	for _, volumeClaim := range statefulSet.Spec.VolumeClaimTemplates {
		if volumeClaim.Name == name {
			return &volumeClaim
		}
	}
	return nil
}

func (cass *currentClusterStateFinder) emptyDirVolumeWithName(name string, statefulSet *v1beta2.StatefulSet) *corev1.Volume {
	for _, volume := range statefulSet.Spec.Template.Spec.Volumes {
		if volume.Name == name && volume.EmptyDir != nil {
			return &volume
		}
	}
	return nil
}
