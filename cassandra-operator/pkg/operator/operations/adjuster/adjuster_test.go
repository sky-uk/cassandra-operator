package adjuster

import (
	"encoding/json"
	"fmt"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test/apis"
	"reflect"
	"testing"

	"github.com/PaesslerAG/jsonpath"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/ptr"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test"
)

const (
	clusterConfigHash = "$.spec.template.metadata.annotations.clusterConfigHash"
)

func TestCluster(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Adjuster Suite", test.CreateParallelReporters("adjuster"))
}

var _ = Describe("cluster events", func() {
	var oldCluster, newCluster *v1alpha1.Cassandra
	var adjuster *Adjuster

	BeforeEach(func() {
		oldCluster = apis.ACassandra().WithDefaults().WithSpec(
			apis.ACassandraSpec().
				WithDefaults().
				WithDatacenter("ADatacenter").
				WithRacks(apis.ARack("a", 1).WithDefaults().WithZone("some-zone")).
				WithPod(apis.APod().
					WithDefaults().
					WithImageName(ptr.String("anImage")).
					WithResources(&v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("2Gi"),
							v1.ResourceCPU:    resource.MustParse("100m"),
						},
						Limits: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("2Gi"),
							v1.ResourceCPU:    resource.MustParse("100m"),
						},
					}).
					WithCassandraLivenessProbeFailureThreshold(3).
					WithCassandraInitialDelay(30).
					WithCassandraLivenessPeriod(30).
					WithCassandraLivenessProbeSuccessThreshold(1).
					WithCassandraLivenessTimeout(5).
					WithCassandraReadinessProbeFailureThreshold(3).
					WithCassandraReadinessPeriod(15).
					WithCassandraReadinessProbeSuccessThreshold(1).
					WithCassandraReadinessTimeout(5))).
			Build()

		newCluster = oldCluster.DeepCopy()
		adjuster = New()
	})

	Context("a config map hash annotation patch is requested for rack", func() {
		It("should produce an UpdateRack change with the new config map hash", func() {
			rack := v1alpha1.Rack{Name: "a", Replicas: 1, Zone: "some-zone"}
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
		It("should produce an UpdateRack change when cpu request specification has changed", func() {
			newCluster.Spec.Pod.Resources.Requests[v1.ResourceCPU] = resource.MustParse("110m")
			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)

			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(1))
			Expect(changes).To(HaveClusterChange(newCluster.Spec.Racks[0], UpdateRack))
		})

		It("should produce an UpdateRack change when cpu limit specification has changed", func() {
			newCluster.Spec.Pod.Resources.Limits[v1.ResourceCPU] = resource.MustParse("120m")
			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)

			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(1))
			Expect(changes).To(HaveClusterChange(newCluster.Spec.Racks[0], UpdateRack))
		})

		It("should produce an UpdateRack change when cpu request specification is zero", func() {
			newCluster.Spec.Pod.Resources.Requests[v1.ResourceCPU] = resource.MustParse("0")
			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)

			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(1))
			Expect(changes).To(HaveClusterChange(newCluster.Spec.Racks[0], UpdateRack))
		})

		It("should produce an UpdateRack change when removed from the specification", func() {
			delete(newCluster.Spec.Pod.Resources.Requests, v1.ResourceCPU)
			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)

			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(1))
			Expect(changes).To(HaveClusterChangeWithoutJSONKey(newCluster.Spec.Racks[0], UpdateRack))
		})

		It("should produce an UpdateRack change when cpu limit specification is zero", func() {
			newCluster.Spec.Pod.Resources.Limits[v1.ResourceCPU] = resource.MustParse("0")
			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)

			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(1))
			Expect(changes).To(HaveClusterChange(newCluster.Spec.Racks[0], UpdateRack))
		})

		It("should produce an UpdateRack change when cpu limit removed from the specification", func() {
			delete(newCluster.Spec.Pod.Resources.Limits, v1.ResourceCPU)
			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)

			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(1))
			Expect(changes).To(HaveClusterChangeWithoutJSONKey(newCluster.Spec.Racks[0], UpdateRack))
		})

		It("should produce an UpdateRack change when memory request specification has changed", func() {
			newCluster.Spec.Pod.Resources.Requests[v1.ResourceMemory] = resource.MustParse("1Gi")
			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)

			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(1))
			Expect(changes).To(HaveClusterChange(newCluster.Spec.Racks[0], UpdateRack))
		})

		It("should produce an UpdateRack change when memory limit specification has changed", func() {
			newCluster.Spec.Pod.Resources.Limits[v1.ResourceMemory] = resource.MustParse("3Gi")
			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)

			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(1))
			Expect(changes).To(HaveClusterChange(newCluster.Spec.Racks[0], UpdateRack))
		})

		It("should produce an UpdateRack change when liveness probe specification has changed", func() {
			newCluster.Spec.Pod.LivenessProbe.FailureThreshold = ptr.Int32(5)
			newCluster.Spec.Pod.LivenessProbe.InitialDelaySeconds = ptr.Int32(99)
			newCluster.Spec.Pod.LivenessProbe.PeriodSeconds = ptr.Int32(20)
			newCluster.Spec.Pod.LivenessProbe.TimeoutSeconds = ptr.Int32(10)
			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)

			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(1))
			Expect(changes).To(HaveClusterChange(newCluster.Spec.Racks[0], UpdateRack))
		})

		It("should produce an UpdateRack change when readiness probe specification has changed", func() {
			newCluster.Spec.Pod.ReadinessProbe.FailureThreshold = ptr.Int32(27)
			newCluster.Spec.Pod.ReadinessProbe.InitialDelaySeconds = ptr.Int32(55)
			newCluster.Spec.Pod.ReadinessProbe.PeriodSeconds = ptr.Int32(77)
			newCluster.Spec.Pod.ReadinessProbe.SuccessThreshold = ptr.Int32(80)
			newCluster.Spec.Pod.ReadinessProbe.TimeoutSeconds = ptr.Int32(4)
			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)

			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(1))
			Expect(changes).To(HaveClusterChange(newCluster.Spec.Racks[0], UpdateRack))
		})

		It("should produce an UpdateRack change when the bootstrapper image has been updated", func() {
			newCluster.Spec.Pod.BootstrapperImage = ptr.String("someotherimage")
			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)
			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(1))
			Expect(changes).To(HaveClusterChange(newCluster.Spec.Racks[0], UpdateRack))
		})

		It("should produce an UpdateRack change when the sidecar image has been updated", func() {
			newCluster.Spec.Pod.SidecarImage = ptr.String("anotherImage")
			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)
			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(1))
			Expect(changes).To(HaveClusterChange(newCluster.Spec.Racks[0], UpdateRack))
		})
	})

	Context("scale-up change is detected", func() {
		Context("single-rack cluster", func() {
			It("should produce an UpdateRack change", func() {
				newCluster.Spec.Racks[0].Replicas = 2
				changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)

				Expect(err).ToNot(HaveOccurred())
				Expect(changes).To(HaveLen(1))
				Expect(changes).To(HaveClusterChange(newCluster.Spec.Racks[0], UpdateRack))
			})
		})

		Context("multiple-rack cluster", func() {
			It("should produce an UpdateRack change for each changed rack", func() {
				oldCluster.Spec.Racks = []v1alpha1.Rack{apis.ARack("a", 1).Build(), apis.ARack("b", 1).Build(), apis.ARack("c", 1).Build()}
				newCluster.Spec.Racks = []v1alpha1.Rack{apis.ARack("a", 2).Build(), apis.ARack("b", 1).Build(), apis.ARack("c", 3).Build()}
				changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)

				Expect(err).ToNot(HaveOccurred())
				Expect(changes).To(HaveLen(2))
				Expect(changes).To(HaveClusterChange(newCluster.Spec.Racks[0], UpdateRack))
				Expect(changes).To(HaveClusterChange(newCluster.Spec.Racks[2], UpdateRack))
			})
		})
	})

	Context("both pod resource and scale-up changes are detected", func() {
		It("should produce an UpdateRack change for all racks", func() {
			oldCluster.Spec.Racks = []v1alpha1.Rack{apis.ARack("a", 1).Build(), apis.ARack("b", 1).Build()}
			newCluster.Spec.Racks = []v1alpha1.Rack{apis.ARack("a", 1).Build(), apis.ARack("b", 3).Build()}
			newCluster.Spec.Pod.Resources.Requests[v1.ResourceCPU] = resource.MustParse("1")
			newCluster.Spec.Pod.Resources.Requests[v1.ResourceMemory] = resource.MustParse("999Mi")

			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)
			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(2))
			Expect(changes).To(HaveClusterChange(newCluster.Spec.Racks[0], UpdateRack))
			Expect(changes).To(HaveClusterChange(newCluster.Spec.Racks[1], UpdateRack))
		})
	})

	Context("additional rack storage", func() {
		It("should produce an UpdateRack for all racks", func() {
			oldCluster.Spec = *apis.ACassandraSpec().WithDefaults().
				WithRacks(
					apis.ARack("a", 1).WithStorages(apis.APersistentVolume().WithDefaults().AtPath("path1")),
					apis.ARack("b", 1).WithStorages(apis.APersistentVolume().WithDefaults().AtPath("path1")),
				).
				Build()
			newCluster.Spec = *apis.ACassandraSpec().WithDefaults().
				WithRacks(
					apis.ARack("a", 1).WithStorages(
						apis.APersistentVolume().WithDefaults().AtPath("path1"),
						apis.AnEmptyDir().WithDefaults().AtPath("path1")),
					apis.ARack("b", 1).WithStorages(
						apis.APersistentVolume().WithDefaults().AtPath("path1"),
						apis.AnEmptyDir().WithDefaults().AtPath("path2")),
				).
				Build()
			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)
			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(2))
			Expect(changes).To(HaveClusterChange(newCluster.Spec.Racks[0], UpdateRack))
			Expect(changes).To(HaveClusterChange(newCluster.Spec.Racks[1], UpdateRack))
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
			oldCluster.Spec.Pod.Resources.Requests[v1.ResourceCPU] = resource.MustParse("1000m")
			newCluster.Spec.Pod.Resources.Requests[v1.ResourceCPU] = resource.MustParse("1")
			oldCluster.Spec.Pod.Resources.Limits[v1.ResourceCPU] = resource.MustParse("1000m")
			newCluster.Spec.Pod.Resources.Limits[v1.ResourceCPU] = resource.MustParse("1")
			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)
			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(0))
		})

		It("should not produce a change when memory values are the same but expressed with different units", func() {
			oldCluster.Spec.Pod.Resources.Requests[v1.ResourceMemory] = resource.MustParse("1Gi")
			newCluster.Spec.Pod.Resources.Requests[v1.ResourceMemory] = resource.MustParse("1024Mi")
			oldCluster.Spec.Pod.Resources.Limits[v1.ResourceMemory] = resource.MustParse("1Gi")
			newCluster.Spec.Pod.Resources.Limits[v1.ResourceMemory] = resource.MustParse("1048576Ki")
			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)
			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(0))
		})
	})

	Context("a new rack definition is added", func() {
		It("should produce an AddRack change for the new rack", func() {
			newRack := v1alpha1.Rack{Name: "b", Replicas: 2, Zone: "zone-b"}
			newCluster.Spec.Racks = append(newCluster.Spec.Racks, newRack)

			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)
			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(1))
			Expect(changes).To(HaveClusterChange(newRack, AddRack))
		})
	})

	Context("correct ordering of changes", func() {
		It("should perform add operations before update operations", func() {
			oldCluster.Spec.Racks = []v1alpha1.Rack{apis.ARack("a", 1).Build(), apis.ARack("b", 1).Build()}
			newCluster.Spec.Racks = []v1alpha1.Rack{
				apis.ARack("a", 1).Build(),
				apis.ARack("b", 1).Build(),
				apis.ARack("c", 1).Build(),
			}
			newCluster.Spec.Pod.Resources.Requests[v1.ResourceMemory] = resource.MustParse("3Gi")

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
			oldCluster.Spec.Racks = []v1alpha1.Rack{apis.ARack("a", 1).Build()}
			newCluster.Spec.Racks = []v1alpha1.Rack{apis.ARack("a", 2).Build()}
			newCluster.Spec.Pod.Resources.Requests[v1.ResourceMemory] = resource.MustParse("3Gi")

			changes, err := adjuster.ChangesForCluster(oldCluster, newCluster)

			Expect(err).ToNot(HaveOccurred())
			Expect(changes).To(HaveLen(1))
			Expect(changes[0].ChangeType).To(Equal(UpdateRack))
			Expect(changes[0].Rack.Name).To(Equal("a"))
		})

		It("should order racks changes in the order in which the racks were defined in the old cluster state", func() {
			oldCluster.Spec.Racks = []v1alpha1.Rack{apis.ARack("a", 1).Build(), apis.ARack("b", 1).Build(), apis.ARack("c", 1).Build()}
			newCluster.Spec.Racks = []v1alpha1.Rack{apis.ARack("c", 1).Build(), apis.ARack("a", 1).Build(), apis.ARack("b", 1).Build()}
			newCluster.Spec.Pod.Resources.Requests[v1.ResourceCPU] = resource.MustParse("101m")

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
func HaveClusterChange(rack v1alpha1.Rack, changeType ClusterChangeType) types.GomegaMatcher {
	return &haveClusterChange{rack, changeType}
}

func HaveClusterChangeWithoutJSONKey(rack v1alpha1.Rack, changeType ClusterChangeType) types.GomegaMatcher {
	return &haveClusterChange{rack, changeType}
}

type haveClusterChange struct {
	rack       v1alpha1.Rack
	changeType ClusterChangeType
}

func (matcher *haveClusterChange) Match(actual interface{}) (success bool, err error) {
	changes := actual.([]ClusterChange)
	if change := matcher.findClusterChange(&matcher.rack, changes); change != nil {
		if !reflect.DeepEqual(change.Rack, matcher.rack) {
			return false, fmt.Errorf("expected rack %v to match, but found %v", matcher.rack, change.Rack)
		}

		if change.ChangeType != matcher.changeType {
			return false, fmt.Errorf("expected change type %s, but found %s", matcher.changeType, change.ChangeType)
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
