package adjuster

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/PaesslerAG/jsonpath"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	v1alpha1helpers "github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1/helpers"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/ptr"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test"
)

const (
	containerCPU                      = "$.spec.template.spec.containers[0].resources.requests.cpu"
	containerMemoryRequest            = "$.spec.template.spec.containers[0].resources.requests.memory"
	containerMemoryLimit              = "$.spec.template.spec.containers[0].resources.limits.memory"
	livenessProbeTimeout              = "$.spec.template.spec.containers[0].livenessProbe.timeoutSeconds"
	livenessProbeFailureThreshold     = "$.spec.template.spec.containers[0].livenessProbe.failureThreshold"
	livenessProbeInitialDelaySeconds  = "$.spec.template.spec.containers[0].livenessProbe.initialDelaySeconds"
	livenessProbePeriodSeconds        = "$.spec.template.spec.containers[0].livenessProbe.periodSeconds"
	readinessProbeTimeout             = "$.spec.template.spec.containers[0].readinessProbe.timeoutSeconds"
	readinessProbeFailureThreshold    = "$.spec.template.spec.containers[0].readinessProbe.failureThreshold"
	readinessProbeInitialDelaySeconds = "$.spec.template.spec.containers[0].readinessProbe.initialDelaySeconds"
	readinessProbeSuccessThreshold    = "$.spec.template.spec.containers[0].readinessProbe.successThreshold"
	readinessProbePeriodSeconds       = "$.spec.template.spec.containers[0].readinessProbe.periodSeconds"
	rackReplicas                      = "$.spec.replicas"
	clusterConfigHash                 = "$.spec.template.metadata.annotations.clusterConfigHash"
	bootstrapperImage                 = "$.spec.template.spec.initContainers[0].image"
	sidecarImage                      = "$.spec.template.spec.containers[1].image"
)

func TestCluster(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Operator Suite", test.CreateParallelReporters("operator"))
}

var _ = Describe("cluster events", func() {
	var oldCluster, newCluster *v1alpha1.Cassandra
	var adjuster *Adjuster

	BeforeEach(func() {
		livenessProbe := &v1alpha1.Probe{
			FailureThreshold:    ptr.Int32(3),
			InitialDelaySeconds: ptr.Int32(30),
			PeriodSeconds:       ptr.Int32(30),
			SuccessThreshold:    ptr.Int32(1),
			TimeoutSeconds:      ptr.Int32(5),
		}
		readinessProbe := &v1alpha1.Probe{
			FailureThreshold:    ptr.Int32(3),
			InitialDelaySeconds: ptr.Int32(30),
			PeriodSeconds:       ptr.Int32(15),
			SuccessThreshold:    ptr.Int32(1),
			TimeoutSeconds:      ptr.Int32(5),
		}
		oldCluster = &v1alpha1.Cassandra{
			Spec: v1alpha1.CassandraSpec{
				Datacenter: ptr.String("ADatacenter"),
				Racks:      []v1alpha1.Rack{{Name: "a", Replicas: 1, StorageClass: "some-storage", Zone: "some-zone"}},
				Pod: v1alpha1.Pod{
					Image:          ptr.String("anImage"),
					Memory:         resource.MustParse("2Gi"),
					CPU:            resource.MustParse("100m"),
					StorageSize:    resource.MustParse("1Gi"),
					LivenessProbe:  livenessProbe.DeepCopy(),
					ReadinessProbe: readinessProbe.DeepCopy(),
				},
			},
		}
		v1alpha1helpers.SetDefaultsForCassandra(oldCluster)
		newCluster = &v1alpha1.Cassandra{
			Spec: v1alpha1.CassandraSpec{
				Datacenter: ptr.String("ADatacenter"),
				Racks:      []v1alpha1.Rack{{Name: "a", Replicas: 1, StorageClass: "some-storage", Zone: "some-zone"}},
				Pod: v1alpha1.Pod{
					Image:          ptr.String("anImage"),
					Memory:         resource.MustParse("2Gi"),
					CPU:            resource.MustParse("100m"),
					StorageSize:    resource.MustParse("1Gi"),
					LivenessProbe:  livenessProbe.DeepCopy(),
					ReadinessProbe: readinessProbe.DeepCopy(),
				},
			},
		}
		v1alpha1helpers.SetDefaultsForCassandra(newCluster)
		var err error
		adjuster, err = New()
		Expect(err).To(Not(HaveOccurred()))
	})

	Context("a config map hash annotation patch is requested for rack", func() {
		It("should produce an UpdateRack change with the new config map hash", func() {
			rack := v1alpha1.Rack{Name: "a", Replicas: 1, StorageClass: "some-storage", Zone: "some-zone"}
			configMap := v1.ConfigMap{
				Data: map[string]string{
					"test": "value",
				},
			}
			change := adjuster.CreateConfigMapHashPatchForRack(&rack, &configMap)

			Expect(change.Rack).To(Equal(rack))
			Expect(change.ChangeType).To(Equal(UpdateRack))
			Expect(evaluateJSONPath(clusterConfigHash, change.Patch)).To(Equal("29ab74e6c0e7eb7d55f4d76d92a3f4bab949e0539600ab8f37fdd882fa44cdf4"))
		})
	})

	Context("pod spec change is detected", func() {
		It("should produce a change with updated cpu when cpu specification has changed", func() {
			newCluster.Spec.Pod.CPU = resource.MustParse("110m")
			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)

			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(1))
			Expect(changes).To(HaveClusterChange(newCluster.Spec.Racks[0], UpdateRack, map[string]interface{}{containerCPU: "110m"}))
		})

		It("should produce a change with updated memory when memory specification has changed", func() {
			newCluster.Spec.Pod.Memory = resource.MustParse("1Gi")
			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)

			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(1))
			Expect(changes).To(HaveClusterChange(newCluster.Spec.Racks[0], UpdateRack, map[string]interface{}{containerMemoryRequest: "1Gi", containerMemoryLimit: "1Gi"}))
		})

		It("should produce a change with updated timeout when liveness probe specification has changed", func() {
			newCluster.Spec.Pod.LivenessProbe.FailureThreshold = ptr.Int32(5)
			newCluster.Spec.Pod.LivenessProbe.InitialDelaySeconds = ptr.Int32(99)
			newCluster.Spec.Pod.LivenessProbe.PeriodSeconds = ptr.Int32(20)
			newCluster.Spec.Pod.LivenessProbe.TimeoutSeconds = ptr.Int32(10)
			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)

			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(1))
			Expect(changes).To(HaveClusterChange(newCluster.Spec.Racks[0],
				UpdateRack,
				map[string]interface{}{
					livenessProbeFailureThreshold:    float64(5),
					livenessProbeInitialDelaySeconds: float64(99),
					livenessProbePeriodSeconds:       float64(20),
					livenessProbeTimeout:             float64(10),
				},
			))
		})

		It("should produce a change with updated timeout when readiness probe specification has changed", func() {
			newCluster.Spec.Pod.ReadinessProbe.FailureThreshold = ptr.Int32(27)
			newCluster.Spec.Pod.ReadinessProbe.InitialDelaySeconds = ptr.Int32(55)
			newCluster.Spec.Pod.ReadinessProbe.PeriodSeconds = ptr.Int32(77)
			newCluster.Spec.Pod.ReadinessProbe.SuccessThreshold = ptr.Int32(80)
			newCluster.Spec.Pod.ReadinessProbe.TimeoutSeconds = ptr.Int32(4)
			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)

			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(1))
			Expect(changes).To(HaveClusterChange(newCluster.Spec.Racks[0],
				UpdateRack, map[string]interface{}{
					readinessProbeFailureThreshold:    float64(27),
					readinessProbeInitialDelaySeconds: float64(55),
					readinessProbePeriodSeconds:       float64(77),
					readinessProbeSuccessThreshold:    float64(80),
					readinessProbeTimeout:             float64(4),
				}))
		})

		It("should produce a patch containing the updated image when the bootstrapper image has been updated", func() {
			newCluster.Spec.Pod.BootstrapperImage = ptr.String("someotherimage")
			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)
			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(1))
			Expect(changes).To(HaveClusterChange(newCluster.Spec.Racks[0], UpdateRack, map[string]interface{}{bootstrapperImage: "someotherimage"}))
		})

		It("should produce a patch containing the updated image when the sidecar image has been updated", func() {
			newCluster.Spec.Pod.SidecarImage = ptr.String("anotherImage")
			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)
			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(1))
			Expect(changes).To(HaveClusterChange(newCluster.Spec.Racks[0], UpdateRack, map[string]interface{}{sidecarImage: "anotherImage"}))
		})
	})

	Context("scale-up change is detected", func() {
		Context("single-rack cluster", func() {
			It("should produce a change with the updated number of replicas", func() {
				newCluster.Spec.Racks[0].Replicas = 2
				changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)

				Expect(err).ToNot(HaveOccurred())
				Expect(changes).To(HaveLen(1))
				Expect(changes).To(HaveClusterChange(newCluster.Spec.Racks[0], UpdateRack, map[string]interface{}{rackReplicas: float64(2)}))
			})
		})

		Context("multiple-rack cluster", func() {
			It("should produce a change for each changed rack with the updated number of replicas", func() {
				oldCluster.Spec.Racks = []v1alpha1.Rack{{Name: "a", Replicas: 1, StorageClass: "some-storage", Zone: "some-zone"}, {Name: "b", Replicas: 1, StorageClass: "another-storage", Zone: "another-zone"}, {Name: "c", Replicas: 1, StorageClass: "yet-another-storage", Zone: "yet-another-zone"}}
				newCluster.Spec.Racks = []v1alpha1.Rack{{Name: "a", Replicas: 2, StorageClass: "some-storage", Zone: "some-zone"}, {Name: "b", Replicas: 1, StorageClass: "another-storage", Zone: "another-zone"}, {Name: "c", Replicas: 3, StorageClass: "yet-another-storage", Zone: "yet-another-zone"}}
				changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)

				Expect(err).ToNot(HaveOccurred())
				Expect(changes).To(HaveLen(2))
				Expect(changes).To(HaveClusterChange(newCluster.Spec.Racks[0], UpdateRack, map[string]interface{}{rackReplicas: float64(2)}))
				Expect(changes).To(HaveClusterChange(newCluster.Spec.Racks[2], UpdateRack, map[string]interface{}{rackReplicas: float64(3)}))
			})
		})
	})

	Context("both pod resource and scale-up changes are detected", func() {
		It("should produce a change for all racks with the updated pod resource and replication", func() {
			oldCluster.Spec.Racks = []v1alpha1.Rack{{Name: "a", Replicas: 1, StorageClass: "some-storage", Zone: "some-zone"}, {Name: "b", Replicas: 1, StorageClass: "another-storage", Zone: "another-zone"}}
			newCluster.Spec.Racks = []v1alpha1.Rack{{Name: "a", Replicas: 1, StorageClass: "some-storage", Zone: "some-zone"}, {Name: "b", Replicas: 3, StorageClass: "another-storage", Zone: "another-zone"}}
			newCluster.Spec.Pod.CPU = resource.MustParse("1")
			newCluster.Spec.Pod.Memory = resource.MustParse("999Mi")

			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)
			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(2))
			Expect(changes).To(HaveClusterChange(newCluster.Spec.Racks[0], UpdateRack, map[string]interface{}{rackReplicas: float64(1), containerCPU: "1", containerMemoryRequest: "999Mi", containerMemoryLimit: "999Mi"}))
			Expect(changes).To(HaveClusterChange(newCluster.Spec.Racks[1], UpdateRack, map[string]interface{}{rackReplicas: float64(3), containerCPU: "1", containerMemoryRequest: "999Mi", containerMemoryLimit: "999Mi"}))
		})
	})

	Context("nothing relevant has changed in the definition", func() {
		It("should not produce any changes when nothing has changed", func() {
			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)
			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(0))
		})

		It("should not produce a change when a snapshot configuration is added", func() {
			newCluster.Spec.Snapshot = &v1alpha1.Snapshot{
				Schedule: "2 * * * *",
			}
			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)
			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(0))
		})

		It("should not produce a change when a snapshot configuration is modified", func() {
			oldCluster.Spec.Snapshot = &v1alpha1.Snapshot{
				Schedule: "2 * * * *",
			}
			newCluster.Spec.Snapshot = &v1alpha1.Snapshot{
				Schedule: "3 * * * *",
			}
			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)
			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(0))
		})

		It("should not produce a change when a snapshot configuration is removed", func() {
			oldCluster.Spec.Snapshot = &v1alpha1.Snapshot{
				Schedule: "2 * * * *",
			}
			newCluster.Spec.Snapshot = nil
			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)
			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(0))
		})

		It("should not produce a change when cpu values are the same but expressed with different units", func() {
			oldCluster.Spec.Pod.CPU = resource.MustParse("1000m")
			newCluster.Spec.Pod.CPU = resource.MustParse("1")
			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)
			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(0))
		})

		It("should not produce a change when memory values are the same but expressed with different units", func() {
			oldCluster.Spec.Pod.Memory = resource.MustParse("1Gi")
			newCluster.Spec.Pod.Memory = resource.MustParse("1024Mi")
			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)
			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(0))
		})
	})

	Context("a new rack definition is added", func() {
		It("should produce a change describing the new rack", func() {
			newRack := v1alpha1.Rack{Name: "b", Replicas: 2, Zone: "zone-b", StorageClass: "storage-class-b"}
			newCluster.Spec.Racks = append(newCluster.Spec.Racks, newRack)

			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)
			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(1))
			Expect(changes).To(HaveClusterChange(newRack, AddRack, nil))
		})
	})

	Context("correct ordering of changes", func() {
		It("should perform add operations before update operations", func() {
			oldCluster.Spec.Racks = []v1alpha1.Rack{{Name: "a", Replicas: 1, Zone: "zone-a", StorageClass: "storage-class-a"}, {Name: "b", Replicas: 1, Zone: "zone-b", StorageClass: "storage-class-b"}}
			newCluster.Spec.Racks = []v1alpha1.Rack{
				{Name: "a", Replicas: 1, Zone: "zone-a", StorageClass: "storage-class-a"},
				{Name: "b", Replicas: 1, Zone: "zone-b", StorageClass: "storage-class-b"},
				{Name: "c", Replicas: 1, Zone: "zone-c", StorageClass: "storage-class-c"},
			}
			newCluster.Spec.Pod.Memory = resource.MustParse("3Gi")

			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)

			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(3))
			Expect(changes[0].ChangeType).To(Equal(AddRack))
			Expect(changes[0].Rack.Name).To(Equal("c"))

			Expect(changes[1].ChangeType).To(Equal(UpdateRack))
			Expect(changes[1].Rack.Name).To(Or(Equal("a"), Equal("b")))

			Expect(changes[2].ChangeType).To(Equal(UpdateRack))
			Expect(changes[2].Rack.Name).To(Or(Equal("a"), Equal("b")))
		})

		It("should treat a scale up and update as a single update operation", func() {
			oldCluster.Spec.Racks = []v1alpha1.Rack{{Name: "a", Replicas: 1, Zone: "zone-a", StorageClass: "storage-class-a"}}
			newCluster.Spec.Racks = []v1alpha1.Rack{{Name: "a", Replicas: 2, Zone: "zone-a", StorageClass: "storage-class-a"}}
			newCluster.Spec.Pod.Memory = resource.MustParse("3Gi")

			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)

			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(1))
			Expect(changes[0].ChangeType).To(Equal(UpdateRack))
			Expect(changes[0].Rack.Name).To(Equal("a"))
		})

		It("should order racks changes in the order in which the racks were defined in the old cluster state", func() {
			oldCluster.Spec.Racks = []v1alpha1.Rack{{Name: "a", Replicas: 1, Zone: "zone-a", StorageClass: "storage-class-a"}, {Name: "b", Replicas: 1, Zone: "zone-b", StorageClass: "storage-class-b"}, {Name: "c", Replicas: 1, Zone: "zone-c", StorageClass: "storage-class-c"}}
			newCluster.Spec.Racks = []v1alpha1.Rack{{Name: "c", Replicas: 1, Zone: "zone-c", StorageClass: "storage-class-c"}, {Name: "a", Replicas: 1, Zone: "zone-a", StorageClass: "storage-class-a"}, {Name: "b", Replicas: 1, Zone: "zone-b", StorageClass: "storage-class-b"}}
			newCluster.Spec.Pod.CPU = resource.MustParse("101m")

			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)

			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(3))
			Expect(changes[0].Rack.Name).To(Equal("a"))
			Expect(changes[1].Rack.Name).To(Equal("b"))
			Expect(changes[2].Rack.Name).To(Equal("c"))
		})
	})
})

//
// HaveClusterChange matcher
//
func HaveClusterChange(rack v1alpha1.Rack, changeType ClusterChangeType, patchExpectations map[string]interface{}) types.GomegaMatcher {
	return &haveClusterChange{rack, changeType, patchExpectations}
}

type haveClusterChange struct {
	rack              v1alpha1.Rack
	changeType        ClusterChangeType
	patchExpectations map[string]interface{}
}

func (matcher *haveClusterChange) Match(actual interface{}) (success bool, err error) {
	changes := actual.([]ClusterChange)
	if change := matcher.findClusterChange(&matcher.rack, changes); change != nil {
		if change.Rack != matcher.rack {
			return false, fmt.Errorf("expected rack %v to match, but found %v", matcher.rack, change.Rack)
		}

		if change.ChangeType != matcher.changeType {
			return false, fmt.Errorf("expected change type %s, but found %s", matcher.changeType, change.ChangeType)
		}

		for jsonPath, expectedValue := range matcher.patchExpectations {
			foundValue, err := evaluateJSONPath(jsonPath, change.Patch)
			if err != nil {
				return false, err
			}

			if foundValue != expectedValue {
				return false, fmt.Errorf("expected value %v at json path %s, but got %v. Change patch: %v", expectedValue, jsonPath, foundValue, change.Patch)
			}
		}
	} else {
		return false, fmt.Errorf("no matching change for rack %s in changes %v", matcher.rack.Name, changes)
	}
	return true, nil
}

func (matcher *haveClusterChange) findClusterChange(rackToFind *v1alpha1.Rack, changes []ClusterChange) *ClusterChange {
	for _, change := range changes {
		if change.Rack.Name == matcher.rack.Name {
			return &change
		}
	}
	return nil
}

func (matcher *haveClusterChange) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Actual clusterChange: %s", actual)
}

func (matcher *haveClusterChange) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Actual clusterChange: %s", actual)
}

func evaluateJSONPath(path, document string) (interface{}, error) {
	var v interface{}
	if err := json.Unmarshal([]byte(document), &v); err != nil {
		return nil, err
	}

	return jsonpath.Get(path, v)
}
