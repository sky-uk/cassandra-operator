package cluster

import (
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// StateFinder finds the actual state of resources in the Kubernetes cluster
type StateFinder interface {
	// FindCurrentStateFor returns the reconstructed Cassandra definition by looking
	// at the current state of all Kubernetes resources associated to it
	FindCurrentStateFor(desiredCassandra *v1alpha1.Cassandra) (*v1alpha1.Cassandra, error)

	// FindCurrentConfigHashFor returns the config hash associated to each statefulSet of the given Cassandra
	FindCurrentConfigHashFor(desiredCassandra *v1alpha1.Cassandra) (map[string]string, error)
}

// implements StateFinder
type currentStateFinder struct {
	clusterStateFinder         clusterStateFinder
	snapshotStateFinder        snapshotStateFinder
	snapshotCleanupStateFinder snapshotCleanupStateFinder
}

// NewStateFinder creates a new StateFinder
func NewStateFinder(client client.Client) StateFinder {
	factory := &defaultReferenceFactory{}
	return &currentStateFinder{
		clusterStateFinder:         &currentClusterStateFinder{client: client, objectFactory: factory},
		snapshotStateFinder:        &currentSnapshotStateFinder{client: client, objectFactory: factory},
		snapshotCleanupStateFinder: &currentSnapshotCleanupStateFinder{client: client, objectFactory: factory},
	}
}

// FindCurrentStateFor implements StateFinder
func (s *currentStateFinder) FindCurrentStateFor(desiredCassandra *v1alpha1.Cassandra) (*v1alpha1.Cassandra, error) {
	currentCassandra, err := s.clusterStateFinder.findClusterStateFor(desiredCassandra)
	if err != nil {
		return nil, err
	}

	currentSnapshot, err := s.snapshotStateFinder.findSnapshotStateFor(desiredCassandra)
	if errors.IsNotFound(err) {
		return currentCassandra, nil
	} else if err != nil {
		return nil, err
	}
	currentCassandra.Spec.Snapshot = currentSnapshot

	currentSnapshotCleanup, err := s.snapshotCleanupStateFinder.findSnapshotCleanupStateFor(desiredCassandra)
	if errors.IsNotFound(err) {
		return currentCassandra, nil
	} else if err != nil {
		return nil, err
	}
	currentCassandra.Spec.Snapshot.RetentionPolicy = currentSnapshotCleanup

	return currentCassandra, nil
}

// FindCurrentConfigHashFor implements StateFinder
func (s *currentStateFinder) FindCurrentConfigHashFor(desiredCassandra *v1alpha1.Cassandra) (map[string]string, error) {
	statefulSets, err := s.clusterStateFinder.findStatefulSetsFor(desiredCassandra)
	if err != nil {
		return nil, err
	}

	var configHash = make(map[string]string)
	for _, statefulSet := range statefulSets.Items {
		if statefulSet.Spec.Template.Annotations != nil {
			if _, ok := statefulSet.Spec.Template.Annotations[ConfigHashAnnotation]; ok {
				configHash[statefulSet.Name] = statefulSet.Spec.Template.Annotations[ConfigHashAnnotation]
			}
		}
	}
	return configHash, nil
}
