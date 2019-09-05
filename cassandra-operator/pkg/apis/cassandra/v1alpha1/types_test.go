package v1alpha1

import (
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/ptr"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test"
)

func TestTypes(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Types Suite", test.CreateParallelReporters("types"))
}

var _ = Describe("Cassandra Types", func() {
	Context("Pod", func() {

		DescribeTable("equality",
			podEqualityCheck,
			Entry("if all fields are equal", func(pod, _ *Pod) {}),
			Entry("when cpu request value is the same but using a different amount", func(pod, _ *Pod) { pod.Resources.Requests[coreV1.ResourceCPU] = resource.MustParse("1") }),
			Entry("when cpu limit value is the same but using a different amount", func(pod, _ *Pod) { pod.Resources.Limits[coreV1.ResourceCPU] = resource.MustParse("1") }),
			Entry("when memory request value is the same but using a different amount", func(pod, _ *Pod) { pod.Resources.Requests[coreV1.ResourceMemory] = resource.MustParse("2048Mi") }),
			Entry("when memory limit value is the same but using a different amount", func(pod, _ *Pod) { pod.Resources.Limits[coreV1.ResourceMemory] = resource.MustParse("2048Mi") }),
			Entry("when storage size value is the same but using a different amount", func(pod, _ *Pod) { pod.StorageSize = resource.MustParse("1024Mi") }),
		)

		DescribeTable("inequality",
			podInequalityCheck,
			Entry("when one pod has a nil bootstrap image", func(pod, _ *Pod) { pod.BootstrapperImage = nil }),
			Entry("when pods have different bootstrap images", func(pod, _ *Pod) { pod.BootstrapperImage = ptr.String("another image") }),
			Entry("when one pod has a nil sidecar image", func(pod, _ *Pod) { pod.SidecarImage = nil }),
			Entry("when pods have different sidecar images", func(pod, _ *Pod) { pod.SidecarImage = ptr.String("another image") }),
			Entry("when one pod has a nil cassandra image", func(pod, _ *Pod) { pod.Image = nil }),
			Entry("when pods have different cassandra images", func(pod, _ *Pod) { pod.Image = ptr.String("another image") }),
			Entry("when one pod has no storage size", func(pod, _ *Pod) { pod.StorageSize = resource.Quantity{} }),
			Entry("when pods have different storage sizes", func(pod, _ *Pod) { pod.StorageSize = resource.MustParse("10Gi") }),
			Entry("when one pod has no cpu request", func(pod, _ *Pod) { pod.Resources.Requests[coreV1.ResourceCPU] = resource.Quantity{} }),
			Entry("when one pod has no cpu limit", func(pod, _ *Pod) { pod.Resources.Requests[coreV1.ResourceCPU] = resource.Quantity{} }),
			Entry("when pods have different number of cpu request", func(pod, _ *Pod) { pod.Resources.Requests[coreV1.ResourceCPU] = resource.MustParse("10") }),
			Entry("when pods have different number of cpu limit", func(pod, _ *Pod) { pod.Resources.Limits[coreV1.ResourceCPU] = resource.MustParse("120") }),
			Entry("when one pod has no cpu request and the other has 0 requests", func(pod, otherPod *Pod) {
				delete(pod.Resources.Requests, coreV1.ResourceCPU)
				otherPod.Resources.Requests[coreV1.ResourceCPU] = resource.Quantity{}
			}),
			Entry("when one pod has no cpu limit and the other has 0 limits", func(pod, otherPod *Pod) {
				delete(pod.Resources.Limits, coreV1.ResourceCPU)
				otherPod.Resources.Limits[coreV1.ResourceCPU] = resource.Quantity{}
			}),
			Entry("when one pod has no memory request", func(pod, _ *Pod) { pod.Resources.Requests[coreV1.ResourceMemory] = resource.Quantity{} }),
			Entry("when one pod has no memory limit", func(pod, _ *Pod) { pod.Resources.Limits[coreV1.ResourceMemory] = resource.Quantity{} }),
			Entry("when pods have different memory request sizes", func(pod, _ *Pod) { pod.Resources.Requests[coreV1.ResourceMemory] = resource.MustParse("1000Mi") }),
			Entry("when pods have different memory limit sizes", func(pod, _ *Pod) { pod.Resources.Limits[coreV1.ResourceMemory] = resource.MustParse("1Gi") }),
			Entry("when one pod has no memory request and the other has 0 requests", func(pod, otherPod *Pod) {
				delete(pod.Resources.Requests, coreV1.ResourceMemory)
				otherPod.Resources.Requests[coreV1.ResourceMemory] = resource.Quantity{}
			}),
			Entry("when one pod has no memory limit and the other has 0 limits", func(pod, otherPod *Pod) {
				delete(pod.Resources.Limits, coreV1.ResourceMemory)
				otherPod.Resources.Limits[coreV1.ResourceMemory] = resource.Quantity{}
			}),
			Entry("when liveness probes have different success threshold values", func(pod, _ *Pod) { pod.LivenessProbe.SuccessThreshold = ptr.Int32(20) }),
			Entry("when liveness probes have different timeout values", func(pod, _ *Pod) { pod.LivenessProbe.TimeoutSeconds = ptr.Int32(20) }),
			Entry("when liveness probes have different failure threshold values", func(pod, _ *Pod) { pod.LivenessProbe.FailureThreshold = ptr.Int32(20) }),
			Entry("when liveness probes have different initial delay values", func(pod, _ *Pod) { pod.LivenessProbe.InitialDelaySeconds = ptr.Int32(20) }),
			Entry("when liveness probes have different period seconds values", func(pod, _ *Pod) { pod.LivenessProbe.PeriodSeconds = ptr.Int32(20) }),
			Entry("when one liveness probe has a nil success threshold", func(pod, _ *Pod) { pod.LivenessProbe.SuccessThreshold = nil }),
			Entry("when one liveness probe has a nil timeout", func(pod, _ *Pod) { pod.LivenessProbe.TimeoutSeconds = nil }),
			Entry("when one liveness probe has a nil failure threshold", func(pod, _ *Pod) { pod.LivenessProbe.FailureThreshold = nil }),
			Entry("when one liveness probe has a nil delay", func(pod, _ *Pod) { pod.LivenessProbe.InitialDelaySeconds = nil }),
			Entry("when one liveness probe has a nil period", func(pod, _ *Pod) { pod.LivenessProbe.PeriodSeconds = nil }),
			Entry("when readiness probes have different timeout values", func(pod, _ *Pod) { pod.ReadinessProbe.TimeoutSeconds = ptr.Int32(20) }),
			Entry("when readiness probes have different failure threshold values", func(pod, _ *Pod) { pod.ReadinessProbe.FailureThreshold = ptr.Int32(20) }),
			Entry("when readiness probes have different initial delay values", func(pod, _ *Pod) { pod.ReadinessProbe.InitialDelaySeconds = ptr.Int32(20) }),
			Entry("when readiness probes have different period seconds values", func(pod, _ *Pod) { pod.ReadinessProbe.PeriodSeconds = ptr.Int32(20) }),
			Entry("when one readiness probe has a nil success threshold", func(pod, _ *Pod) { pod.ReadinessProbe.SuccessThreshold = nil }),
			Entry("when one readiness probe has a nil timeout", func(pod, _ *Pod) { pod.ReadinessProbe.TimeoutSeconds = nil }),
			Entry("when one readiness probe has a nil failure threshold", func(pod, _ *Pod) { pod.ReadinessProbe.FailureThreshold = nil }),
			Entry("when one readiness probe has a nil delay", func(pod, _ *Pod) { pod.ReadinessProbe.InitialDelaySeconds = nil }),
			Entry("when one readiness probe has a nil period", func(pod, _ *Pod) { pod.ReadinessProbe.PeriodSeconds = nil }),
		)
	})

	Context("Rack", func() {
		DescribeTable("equality",
			rackEqualityCheck,
			Entry("if all fields are equal", func(rack *Rack) {}),
		)
		DescribeTable("inequality",
			rackInequalityCheck,
			Entry("when one rack has a different name", func(rack *Rack) { rack.Name = "otherName" }),
			Entry("when one rack has a different zone", func(rack *Rack) { rack.Zone = "otherZone" }),
			Entry("when one rack has a different storage class", func(rack *Rack) { rack.StorageClass = "other Storage class" }),
			Entry("when one rack has a different number of replicas", func(rack *Rack) { rack.Replicas = 10 }),
		)
	})

	Context("Snapshot", func() {
		DescribeTable("equality",
			snapshotEqualityCheck,
			Entry("if all fields are equal", func(snapshot *Snapshot) {}),
			Entry("when the keyspaces are in different order", func(snapshot *Snapshot) { snapshot.Keyspaces = []string{"k2", "k1"} }),
		)
		DescribeTable("inequality",
			snapshotInequalityCheck,
			Entry("when one snapshot has a different image", func(snapshot *Snapshot) { snapshot.Image = ptr.String("otherImage") }),
			Entry("when one snapshot has a different schedule", func(snapshot *Snapshot) { snapshot.Schedule = "other Schedule" }),
			Entry("when one snapshot has different keyspaces", func(snapshot *Snapshot) { snapshot.Keyspaces = []string{"k1"} }),
			Entry("when one snapshot has no keyspaces", func(snapshot *Snapshot) { snapshot.Keyspaces = nil }),
			Entry("when one snapshot has an empty keyspaces list", func(snapshot *Snapshot) { snapshot.Keyspaces = []string{} }),
			Entry("when one snapshot has a different timeout", func(snapshot *Snapshot) { snapshot.TimeoutSeconds = ptr.Int32(1) }),
			Entry("when one snapshot has no timeout", func(snapshot *Snapshot) { snapshot.TimeoutSeconds = nil }),
			Entry("when one snapshot has retention policy", func(snapshot *Snapshot) { snapshot.RetentionPolicy = nil }),
			Entry("when one snapshot has retention policy disabled", func(snapshot *Snapshot) { snapshot.RetentionPolicy.Enabled = ptr.Bool(false) }),
			Entry("when one snapshot has a different cleanup schedule", func(snapshot *Snapshot) { snapshot.RetentionPolicy.CleanupSchedule = "other schedule" }),
			Entry("when one snapshot has no cleanup timeout", func(snapshot *Snapshot) { snapshot.RetentionPolicy.CleanupTimeoutSeconds = ptr.Int32(1) }),
			Entry("when one snapshot has a different cleanup timeout", func(snapshot *Snapshot) { snapshot.RetentionPolicy.CleanupTimeoutSeconds = nil }),
			Entry("when one snapshot has no retention period", func(snapshot *Snapshot) { snapshot.RetentionPolicy.RetentionPeriodDays = ptr.Int32(1) }),
			Entry("when one snapshot has a different retention period", func(snapshot *Snapshot) { snapshot.RetentionPolicy.RetentionPeriodDays = nil }),
		)
	})

	Context("CassandraSpec", func() {
		DescribeTable("equality",
			cassandraEqualityCheck,
			Entry("if all fields are equal", func(cass *CassandraSpec) {}),
			Entry("when the racks are in a different order", func(cass *CassandraSpec) { cass.Racks = []Rack{*rackSpec("b"), *rackSpec("a")} }),
		)
		DescribeTable("inequality",
			cassandraInequalityCheck,
			Entry("when one cassandra has a different datacenter", func(cass *CassandraSpec) { cass.Datacenter = ptr.String("otherDc") }),
			Entry("when one cassandra has empty dir enabled", func(cass *CassandraSpec) { cass.UseEmptyDir = ptr.Bool(true) }),
			Entry("when one cassandra has no racks", func(cass *CassandraSpec) { cass.Racks = nil }),
			Entry("when one cassandra has an empty racks list", func(cass *CassandraSpec) { cass.Racks = []Rack{} }),
			Entry("when one cassandra has a different number of racks", func(cass *CassandraSpec) { cass.Racks = []Rack{*rackSpec("a")} }),
			Entry("when one cassandra has a no snapshot", func(cass *CassandraSpec) { cass.Snapshot = nil }),
			Entry("when one cassandra has a different schedule", func(cass *CassandraSpec) { cass.Snapshot.Schedule = "1 2 3 4" }),
			Entry("when one cassandra has a different pod spec", func(cass *CassandraSpec) { cass.Pod.Resources.Requests[coreV1.ResourceCPU] = resource.MustParse("30") }),
		)
	})
})

func cassandraEqualityCheck(mutate func(cassandra *CassandraSpec)) {
	cassandrasAreEqual := func(cassandra, otherCass *CassandraSpec) bool {
		return cassandra.Equal(*otherCass) && otherCass.Equal(*cassandra)
	}
	cassandraComparisonCheck(mutate, cassandrasAreEqual)
}

func cassandraInequalityCheck(mutate func(cassandra *CassandraSpec)) {
	cassandrasNotEqual := func(cassandra, otherCass *CassandraSpec) bool {
		return !cassandra.Equal(*otherCass) && !otherCass.Equal(*cassandra)
	}
	cassandraComparisonCheck(mutate, cassandrasNotEqual)
}

func cassandraComparisonCheck(mutate func(cassandra *CassandraSpec), expectCheck func(cassandra, otherCassandraSpec *CassandraSpec) bool) {
	cassandra1 := &CassandraSpec{
		Datacenter:  ptr.String("dc"),
		UseEmptyDir: ptr.Bool(false),
		Pod:         *podSpec(),
		Racks:       []Rack{*rackSpec("a"), *rackSpec("b")},
		Snapshot:    snapshotSpec(),
	}
	cassandra2 := cassandra1.DeepCopy()

	mutate(cassandra1)

	Expect(expectCheck(cassandra1, cassandra2)).To(BeTrue())
}

func snapshotEqualityCheck(mutate func(snapshot *Snapshot)) {
	snapshotsEqual := func(snapshot, otherRack *Snapshot) bool {
		return snapshot.Equal(*otherRack) && otherRack.Equal(*snapshot)
	}
	snapshotComparisonCheck(mutate, snapshotsEqual)
}

func snapshotInequalityCheck(mutate func(snapshot *Snapshot)) {
	snapshotsNotEqual := func(snapshot, otherRack *Snapshot) bool {
		return !snapshot.Equal(*otherRack) && !otherRack.Equal(*snapshot)
	}
	snapshotComparisonCheck(mutate, snapshotsNotEqual)
}

func snapshotComparisonCheck(mutate func(snapshot *Snapshot), expectCheck func(snapshot, otherSnapshot *Snapshot) bool) {
	snapshot1 := snapshotSpec()
	snapshot2 := snapshot1.DeepCopy()

	mutate(snapshot1)

	Expect(expectCheck(snapshot1, snapshot2)).To(BeTrue())
}

func rackEqualityCheck(mutate func(rack *Rack)) {
	racksEqual := func(rack, otherRack *Rack) bool {
		return rack.Equal(*otherRack) && otherRack.Equal(*rack)
	}
	rackComparisonCheck(mutate, racksEqual)
}

func rackInequalityCheck(mutate func(rack *Rack)) {
	racksNotEqual := func(rack, otherRack *Rack) bool {
		return !rack.Equal(*otherRack) && !otherRack.Equal(*rack)
	}
	rackComparisonCheck(mutate, racksNotEqual)
}

func rackComparisonCheck(mutate func(rack *Rack), expectCheck func(rack, otherRack *Rack) bool) {
	rack1 := rackSpec("a")
	rack2 := rack1.DeepCopy()

	mutate(rack1)

	Expect(expectCheck(rack1, rack2)).To(BeTrue())
}
func podEqualityCheck(mutate func(pod, otherPod *Pod)) {
	podsEqual := func(pod, otherPod *Pod) bool {
		return pod.Equal(*otherPod) && otherPod.Equal(*pod)
	}
	podComparisonCheck(mutate, podsEqual)
}

func podInequalityCheck(mutate func(pod, otherPod *Pod)) {
	podsNotEqual := func(pod, otherPod *Pod) bool {
		return !pod.Equal(*otherPod) && !otherPod.Equal(*pod)
	}
	podComparisonCheck(mutate, podsNotEqual)
}

func podComparisonCheck(mutate func(pod, otherPod *Pod), expectCheck func(pod, otherPod *Pod) bool) {
	pod1 := podSpec()
	pod2 := pod1.DeepCopy()

	mutate(pod1, pod2)

	Expect(expectCheck(pod1, pod2)).To(BeTrue())
}

func podSpec() *Pod {
	return &Pod{
		BootstrapperImage: ptr.String("BootstrapperImage"),
		SidecarImage:      ptr.String("SidecarImage"),
		Image:             ptr.String("Image"),
		Resources: coreV1.ResourceRequirements{
			Limits: coreV1.ResourceList{
				coreV1.ResourceMemory: resource.MustParse("2Gi"),
				coreV1.ResourceCPU:    resource.MustParse("1000m"),
			},
			Requests: coreV1.ResourceList{
				coreV1.ResourceMemory: resource.MustParse("2Gi"),
				coreV1.ResourceCPU:    resource.MustParse("1000m"),
			},
		},
		StorageSize: resource.MustParse("1Gi"),
		LivenessProbe: &Probe{
			SuccessThreshold:    ptr.Int32(1),
			PeriodSeconds:       ptr.Int32(2),
			InitialDelaySeconds: ptr.Int32(3),
			FailureThreshold:    ptr.Int32(4),
			TimeoutSeconds:      ptr.Int32(5),
		},
		ReadinessProbe: &Probe{
			SuccessThreshold:    ptr.Int32(1),
			PeriodSeconds:       ptr.Int32(2),
			InitialDelaySeconds: ptr.Int32(3),
			FailureThreshold:    ptr.Int32(4),
			TimeoutSeconds:      ptr.Int32(5),
		},
	}
}

func rackSpec(name string) *Rack {
	return &Rack{
		Name:         name,
		Zone:         "storage Zone",
		StorageClass: "storage Class",
		Replicas:     1,
	}
}

func snapshotSpec() *Snapshot {
	return &Snapshot{
		Image:          ptr.String("SnapshotImage"),
		Schedule:       "* * * * *",
		Keyspaces:      []string{"k1", "k2"},
		TimeoutSeconds: ptr.Int32(62),
		RetentionPolicy: &RetentionPolicy{
			Enabled:               ptr.Bool(true),
			RetentionPeriodDays:   ptr.Int32(30),
			CleanupTimeoutSeconds: ptr.Int32(30),
			CleanupSchedule:       "* * * * *",
		},
	}
}
