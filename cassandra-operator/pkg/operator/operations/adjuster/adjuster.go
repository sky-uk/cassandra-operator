package adjuster

import (
	"bytes"
	"fmt"
	"text/template"
	"time"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/core/v1"

	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	v1alpha1helpers "github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1/helpers"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/operator/hash"
)

const statefulSetPatchTemplate = `{
  "spec": {
	"replicas": {{ .Replicas }},
	"template": {
	  "spec": {
		"initContainers": [{
		   "name": "cassandra-bootstrapper",
		   "image": "{{ .PodBootstrapperImage }}"
		}],
		"containers": [{
		   "name": "cassandra",
		   "livenessProbe": {
			 "failureThreshold": {{ .PodLivenessProbe.FailureThreshold }},
			 "initialDelaySeconds": {{ .PodLivenessProbe.InitialDelaySeconds }},
			 "periodSeconds": {{ .PodLivenessProbe.PeriodSeconds }},
			 "successThreshold": {{ .PodLivenessProbe.SuccessThreshold }},
			 "timeoutSeconds": {{ .PodLivenessProbe.TimeoutSeconds }}
		   },
		   "readinessProbe": {
			 "failureThreshold": {{ .PodReadinessProbe.FailureThreshold }},
			 "initialDelaySeconds": {{ .PodReadinessProbe.InitialDelaySeconds }},
			 "periodSeconds": {{ .PodReadinessProbe.PeriodSeconds }},
			 "successThreshold": {{ .PodReadinessProbe.SuccessThreshold }},
			 "timeoutSeconds": {{ .PodReadinessProbe.TimeoutSeconds }}
		   },
		   "resources": {
			 "requests": {
               {{ if .PodCPURequest }}
			   "cpu": "{{ .PodCPURequest }}",
    	 	   {{ end }}	
			   "memory": "{{ .PodMemoryRequest }}"
			 },
			 "limits": {
               {{ if .PodCPULimit}}
			   "cpu": "{{ .PodCPULimit }}",
               {{ end }}
			   "memory": "{{ .PodMemoryLimit }}"
			 }
		   }
		},
		{
		   "name": "cassandra-sidecar",
		   "image": "{{ .PodCassandraSidecarImage }}"
		}]
	  }
	}
  }
}`

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
	patchTemplate *template.Template
}

type patchProperties struct {
	Replicas                 int32
	PodBootstrapperImage     string
	PodCassandraSidecarImage string
	PodCPURequest            string
	PodMemoryRequest         string
	PodCPULimit              string
	PodMemoryLimit           string
	PodLivenessProbe         *v1alpha1.Probe
	PodReadinessProbe        *v1alpha1.Probe
}

// New creates a new Adjuster.
func New() (*Adjuster, error) {
	tmpl := template.New("cassandra-spec-patch")
	tmpl, err := tmpl.Parse(statefulSetPatchTemplate)
	if err != nil {
		return nil, fmt.Errorf("unable to parse cassandra patch template %s: %v", statefulSetPatchTemplate, err)
	}
	return &Adjuster{tmpl}, nil
}

// ChangesForCluster compares oldCluster with newCluster, and produces an ordered list of ClusterChanges which need to
// be applied in order for the running cluster to be in the state matching newCluster.
func (r *Adjuster) ChangesForCluster(oldCluster *v1alpha1.Cassandra, newCluster *v1alpha1.Cassandra) ([]ClusterChange, error) {
	addedRacks, matchedRacks, _ := v1alpha1helpers.MatchRacks(&oldCluster.Spec, &newCluster.Spec)
	changeTime := time.Now()
	var clusterChanges []ClusterChange

	for _, addedRack := range addedRacks {
		clusterChanges = append(clusterChanges, ClusterChange{Rack: addedRack, ChangeType: AddRack})
	}

	if r.podSpecHasChanged(oldCluster, newCluster) {
		for _, matchedRack := range matchedRacks {
			patch, err := r.patchForRack(&matchedRack.New, newCluster, changeTime)
			if err != nil {
				return nil, err
			}
			clusterChanges = append(clusterChanges, ClusterChange{Rack: matchedRack.New, ChangeType: UpdateRack, Patch: patch})
		}
	} else {
		for _, matchedRack := range r.scaledUpRacks(matchedRacks) {
			patch, err := r.patchForRack(&matchedRack, newCluster, changeTime)
			if err != nil {
				return nil, err
			}
			clusterChanges = append(clusterChanges, ClusterChange{Rack: matchedRack, ChangeType: UpdateRack, Patch: patch})
		}
	}

	return clusterChanges, nil
}

// CreateConfigMapHashPatchForRack produces a ClusterChange which need to be applied for the given rack
func (r *Adjuster) CreateConfigMapHashPatchForRack(rack *v1alpha1.Rack, configMap *v1.ConfigMap) *ClusterChange {
	configMapHash := hash.ConfigMapHash(configMap)
	patch := fmt.Sprintf(updateAnnotationPatchFormat, cluster.ConfigHashAnnotation, configMapHash)
	return &ClusterChange{Rack: *rack, ChangeType: UpdateRack, Patch: patch}
}

func (r *Adjuster) patchForRack(rack *v1alpha1.Rack, newCluster *v1alpha1.Cassandra, changeTime time.Time) (string, error) {
	props := patchProperties{
		Replicas:                 rack.Replicas,
		PodBootstrapperImage:     *newCluster.Spec.Pod.BootstrapperImage,
		PodCassandraSidecarImage: *newCluster.Spec.Pod.SidecarImage,
		PodCPURequest:            quantityOrEmpty(newCluster.Spec.Pod.Resources.Requests, v1.ResourceCPU),
		PodMemoryRequest:         newCluster.Spec.Pod.Resources.Requests.Memory().String(),
		PodCPULimit:              quantityOrEmpty(newCluster.Spec.Pod.Resources.Limits, v1.ResourceCPU),
		PodMemoryLimit:           newCluster.Spec.Pod.Resources.Limits.Memory().String(),
		PodLivenessProbe:         newCluster.Spec.Pod.LivenessProbe,
		PodReadinessProbe:        newCluster.Spec.Pod.ReadinessProbe,
	}
	var patch bytes.Buffer
	if err := r.patchTemplate.Execute(&patch, props); err != nil {
		return "", fmt.Errorf("unable to create patch from template with properties: %v, error: %v", props, err)
	}

	patchString := patch.String()
	return patchString, nil
}

func quantityOrEmpty(resources v1.ResourceList, resourceName v1.ResourceName) string {
	if val, ok := resources[resourceName]; ok {
		return val.String()
	}
	return ""
}

func (r *Adjuster) podSpecHasChanged(oldCluster, newCluster *v1alpha1.Cassandra) bool {
	return !cmp.Equal(oldCluster.Spec.Pod, newCluster.Spec.Pod)
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
