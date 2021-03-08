package adjuster

import (
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	v1alpha1helpers "github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1/helpers"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/hash"
	v1 "k8s.io/api/core/v1"
	"reflect"
)

const updateAnnotationPatchFormat = `{
  "spec": {
	"template": {
	  "metadata": {
		"annotations": {
			"%s": "%s"
		}
	  }
	}
  }
}`

// ClusterChangeType describes the type of change which needs to be made to a cluster.
type ClusterChangeType string

const (
	// AddRack means that a new rack should be added to a cluster.
	AddRack ClusterChangeType = "add rack"
	// UpdateRack means that an existing rack in the cluster needs to be updated.
	UpdateRack ClusterChangeType = "update rack"
)

// ClusterChange describes a single change which needs to be applied to Kubernetes in order for the running cluster to
// match the requirements described in the cluster definition.
type ClusterChange struct {
	// This is not a pointer on purpose to isolate the change from the actual state
	Rack       v1alpha1.Rack
	ChangeType ClusterChangeType
	Patch      string
}

// Adjuster calculates the set of changes which need to be applied to Kubernetes in order for the running
// cluster to match the requirements described in the cluster definition.
type Adjuster struct {
}

// New creates a new Adjuster.
func New() *Adjuster {
	return &Adjuster{}
}

// ChangesForCluster compares oldCluster with newCluster, and produces an ordered list of ClusterChanges which need to
// be applied in order for the running cluster to be in the state matching newCluster.
func (r *Adjuster) ChangesForCluster(oldCluster *v1alpha1.Cassandra, newCluster *v1alpha1.Cassandra) []ClusterChange {
	addedRacks, matchedRacks, _ := v1alpha1helpers.MatchRacks(&oldCluster.Spec, &newCluster.Spec)
	var clusterChanges []ClusterChange

	if r.podSpecHasChanged(oldCluster, newCluster) || r.rackStorageHasChanged(matchedRacks) {
		for _, matchedRack := range matchedRacks {
			clusterChanges = append(clusterChanges, ClusterChange{Rack: matchedRack.New, ChangeType: UpdateRack})
		}
	} else {
		for _, matchedRack := range r.scaledUpRacks(matchedRacks) {
			clusterChanges = append(clusterChanges, ClusterChange{Rack: matchedRack, ChangeType: UpdateRack})
		}
	}

	for _, addedRack := range addedRacks {
		clusterChanges = append(clusterChanges, ClusterChange{Rack: addedRack, ChangeType: AddRack})
	}
	return clusterChanges
}

// CreateConfigMapHashPatchForRack produces a ClusterChange which need to be applied for the given rack
func (r *Adjuster) CreateConfigMapHashPatchForRack(rack *v1alpha1.Rack, configMap *v1.ConfigMap) *ClusterChange {
	configMapHash := hash.ConfigMapHash(configMap)
	patch := fmt.Sprintf(updateAnnotationPatchFormat, cluster.ConfigHashAnnotation, configMapHash)
	return &ClusterChange{Rack: *rack, ChangeType: UpdateRack, Patch: patch}
}

func (r *Adjuster) podSpecHasChanged(oldCluster, newCluster *v1alpha1.Cassandra) bool {
	return !cmp.Equal(oldCluster.Spec.Pod, newCluster.Spec.Pod)
}

func (r *Adjuster) rackStorageHasChanged(matchedRacks []v1alpha1helpers.MatchedRack) bool {
	for _, matchedRack := range matchedRacks {
		if !reflect.DeepEqual(matchedRack.New.Storage, matchedRack.Old.Storage) {
			return true
		}
	}
	return false
}

func (r *Adjuster) scaledUpRacks(matchedRacks []v1alpha1helpers.MatchedRack) []v1alpha1.Rack {
	var scaledUpRacks []v1alpha1.Rack
	for _, matchedRack := range matchedRacks {
		if matchedRack.New.Replicas > matchedRack.Old.Replicas {
			scaledUpRacks = append(scaledUpRacks, matchedRack.New)
		}
	}
	return scaledUpRacks
}
