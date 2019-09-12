package validation_test

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1/validation"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/ptr"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test"
)

func TestValidation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Validation Suite", test.CreateParallelReporters("validation"))
}

var _ = Describe("ValidateCassandra", func() {
	DescribeTable(
		"success cases",
		func(mutate func(*v1alpha1.Cassandra)) {
			c := aValidCassandra()
			mutate(c)
			err := validation.ValidateCassandra(c).ToAggregate()
			Expect(err).ToNot(HaveOccurred())
		},
		Entry(
			"A fully populated Cassandra cluster",
			func(c *v1alpha1.Cassandra) {},
		),
		Entry(
			"Snapshot is not required",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Snapshot = nil
			},
		),
		Entry(
			"Snapshot.RetentionPolicy is not required",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Snapshot.RetentionPolicy = nil
			},
		),
		Entry(
			"Pod cpu request can be provided without cpu limit",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.Resources.Requests[coreV1.ResourceCPU] = resource.MustParse("151m")
				delete(c.Spec.Pod.Resources.Limits, coreV1.ResourceCPU)
			},
		),
		Entry(
			"Pod cpu limit can be provided without cpu request",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.Resources.Limits[coreV1.ResourceCPU] = resource.MustParse("151m")
				delete(c.Spec.Pod.Resources.Requests, coreV1.ResourceCPU)
			},
		),
		Entry(
			"Both pod cpu request and limit can be omitted",
			func(c *v1alpha1.Cassandra) {
				delete(c.Spec.Pod.Resources.Limits, coreV1.ResourceCPU)
				delete(c.Spec.Pod.Resources.Requests, coreV1.ResourceCPU)
			},
		),
		Entry(
			"A rack storage class may be nil",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Racks[0].Storage[0].PersistentVolumeClaim.StorageClassName = nil
			},
		),
		Entry(
			"A rack storage class may be empty",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Racks[0].Storage[0].PersistentVolumeClaim.StorageClassName = ptr.String("")
			},
		),
	)

	DescribeTable(
		"failure cases",
		func(expectedMessage string, mutate func(*v1alpha1.Cassandra)) {
			c := aValidCassandra()
			mutate(c)
			err := validation.ValidateCassandra(c).ToAggregate()
			fmt.Println(err)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(expectedMessage))
		},
		Entry(
			"Spec.Racks is required",
			"spec.Racks: Required value",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Racks = nil
			},
		),
		Entry(
			"Rack.Replicas must be positive",
			"spec.Racks.rack1.Replicas: Invalid value: -1: must be >= 1",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Racks[0].Replicas = -1
			},
		),
		Entry(
			"Rack.Replicas must be >= 1",
			"spec.Racks.rack1.Replicas: Invalid value: 0: must be >= 1",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Racks[0].Replicas = 0
			},
		),
		Entry(
			"Rack storage is required",
			"spec.Racks.rack1.Storage: Required value: at least one storage is required",
			func(c *v1alpha1.Cassandra) {
				c.Spec.UseEmptyDir = ptr.Bool(false)
				c.Spec.Racks[0].Storage = []v1alpha1.Storage{}
			},
		),
		Entry(
			"Rack storage must contain one storage source",
			"spec.Racks.rack1.Storage[0]: Required value: one storage source is required",
			func(c *v1alpha1.Cassandra) {
				c.Spec.UseEmptyDir = ptr.Bool(false)
				c.Spec.Racks[0].Storage[0].StorageSource = v1alpha1.StorageSource{}
			},
		),
		Entry(
			"Rack storage  may contain at most one volume type",
			"spec.Racks.rack1.Storage[0]: Forbidden: only one storage source per storage is allowed",
			func(c *v1alpha1.Cassandra) {
				c.Spec.UseEmptyDir = ptr.Bool(false)
				c.Spec.Racks[0].Storage[0].StorageSource = v1alpha1.StorageSource{
					PersistentVolumeClaim: &coreV1.PersistentVolumeClaimSpec{
						Resources:coreV1.ResourceRequirements{
							Requests: coreV1.ResourceList{
								coreV1.ResourceStorage: resource.MustParse("100Gi"),
							},
						},
					},
					EmptyDir:              &coreV1.EmptyDirVolumeSource{},
				}
			},
		),
		Entry(
			"Rack storage size is required",
			"spec.Racks.rack1.Storage[0].persistentVolumeClaim.resources[storage]: Required value: a storage size is required",
			func(c *v1alpha1.Cassandra) {
				c.Spec.UseEmptyDir = ptr.Bool(false)
				delete(c.Spec.Racks[0].Storage[0].PersistentVolumeClaim.Resources.Requests, coreV1.ResourceStorage)
			},
		),
		Entry(
			"Rack storage path is required",
			"spec.Racks.rack1.Storage[0].Path: Required value: a volume path is required",
			func(c *v1alpha1.Cassandra) {
				c.Spec.UseEmptyDir = ptr.Bool(false)
				c.Spec.Racks[0].Storage[0].Path = nil
			},
		),
		// TODO support for multiple volumes would be allowed soon
		Entry(
			"Rack storage may not contain more than one storage (for now)",
			"spec.Racks.rack1.Storage: Forbidden: no more than one storage per rack is allowed",
			func(c *v1alpha1.Cassandra) {
				c.Spec.UseEmptyDir = ptr.Bool(false)
				otherStorage := v1alpha1.Storage{
					Path: ptr.String("some path"),
					StorageSource: v1alpha1.StorageSource{
						EmptyDir: &coreV1.EmptyDirVolumeSource{},
					},
				}
				c.Spec.Racks[0].Storage = append(c.Spec.Racks[0].Storage, otherStorage)
			},
		),
		// TODO I don't think this is a valid requirement and one that we won't be able to enforce when supporting multiple volumes
		Entry(
			"Rack.Zone must not be empty when UseEmptyDir=false",
			"spec.Racks.rack1.Zone: Required value: because spec.useEmptyDir is false",
			func(c *v1alpha1.Cassandra) {
				c.Spec.UseEmptyDir = ptr.Bool(false)
				c.Spec.Racks[0].Zone = ""
			},
		),
		Entry(
			"Pod.Resources.Requests.Memory must be > 0",
			"spec.Pod.Resources.Requests.Memory: Invalid value: \"0\": must be > 0",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.Resources.Requests[coreV1.ResourceMemory] = resource.Quantity{}
			},
		),
		Entry(
			"Pod cpu request > cpu limit",
			"spec.Pod.Resources.Requests.Cpu: Invalid value: \"151m\": must not be greater than spec.Pod.Resources.Limits.Cpu (150m)",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.Resources.Requests[coreV1.ResourceCPU] = resource.MustParse("151m")
				c.Spec.Pod.Resources.Limits[coreV1.ResourceCPU] = resource.MustParse("150m")
			},
		),
		Entry(
			"Pod memory request > cpu limit",
			"spec.Pod.Resources.Requests.Memory: Invalid value: \"151Mi\": must not be greater than spec.Pod.Resources.Limits.Memory (150Mi)",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.Resources.Requests[coreV1.ResourceMemory] = resource.MustParse("151Mi")
				c.Spec.Pod.Resources.Limits[coreV1.ResourceMemory] = resource.MustParse("150Mi")
			},
		),
		Entry(
			"LivenessProbe.FailureThreshold must be positive",
			"spec.Pod.LivenessProbe.FailureThreshold: Invalid value: -1: must be >= 1",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.LivenessProbe.FailureThreshold = ptr.Int32(-1)
			},
		),
		Entry(
			"LivenessProbe.FailureThreshold must be >= 1",
			"spec.Pod.LivenessProbe.FailureThreshold: Invalid value: 0: must be >= 1",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.LivenessProbe.FailureThreshold = ptr.Int32(0)
			},
		),
		Entry(
			"LivenessProbe.InitialDelaySeconds must be positive",
			"spec.Pod.LivenessProbe.InitialDelaySeconds: Invalid value: -1: must be >= 0",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.LivenessProbe.InitialDelaySeconds = ptr.Int32(-1)
			},
		),
		Entry(
			"LivenessProbe.PeriodSeconds must be positive",
			"spec.Pod.LivenessProbe.PeriodSeconds: Invalid value: -1: must be >= 1",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.LivenessProbe.PeriodSeconds = ptr.Int32(-1)
			},
		),
		Entry(
			"LivenessProbe.PeriodSeconds must be >= 1",
			"spec.Pod.LivenessProbe.PeriodSeconds: Invalid value: 0: must be >= 1",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.LivenessProbe.PeriodSeconds = ptr.Int32(0)
			},
		),
		Entry(
			"LivenessProbe.SuccessThreshold must be positive",
			"spec.Pod.LivenessProbe.SuccessThreshold: Invalid value: -1: must be 1",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.LivenessProbe.SuccessThreshold = ptr.Int32(-1)
			},
		),
		Entry(
			"LivenessProbe.SuccessThreshold must be 1 (<1)",
			"spec.Pod.LivenessProbe.SuccessThreshold: Invalid value: 0: must be 1",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.LivenessProbe.SuccessThreshold = ptr.Int32(0)
			},
		),
		Entry(
			"LivenessProbe.SuccessThreshold must be 1 (>1)",
			"spec.Pod.LivenessProbe.SuccessThreshold: Invalid value: 2: must be 1",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.LivenessProbe.SuccessThreshold = ptr.Int32(2)
			},
		),
		Entry(
			"LivenessProbe.TimeoutSeconds must be positive",
			"spec.Pod.LivenessProbe.TimeoutSeconds: Invalid value: -1: must be >= 1",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.LivenessProbe.TimeoutSeconds = ptr.Int32(-1)
			},
		),
		Entry(
			"LivenessProbe.TimeoutSeconds must be >= 1",
			"spec.Pod.LivenessProbe.TimeoutSeconds: Invalid value: 0: must be >= 1",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.LivenessProbe.TimeoutSeconds = ptr.Int32(0)
			},
		),

		Entry(
			"ReadinessProbe.FailureThreshold must be positive",
			"spec.Pod.ReadinessProbe.FailureThreshold: Invalid value: -1: must be >= 1",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.ReadinessProbe.FailureThreshold = ptr.Int32(-1)
			},
		),
		Entry(
			"ReadinessProbe.FailureThreshold must be >= 1",
			"spec.Pod.ReadinessProbe.FailureThreshold: Invalid value: 0: must be >= 1",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.ReadinessProbe.FailureThreshold = ptr.Int32(0)
			},
		),
		Entry(
			"ReadinessProbe.InitialDelaySeconds must be positive",
			"spec.Pod.ReadinessProbe.InitialDelaySeconds: Invalid value: -1: must be >= 0",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.ReadinessProbe.InitialDelaySeconds = ptr.Int32(-1)
			},
		),
		Entry(
			"ReadinessProbe.PeriodSeconds must be positive",
			"spec.Pod.ReadinessProbe.PeriodSeconds: Invalid value: -1: must be >= 1",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.ReadinessProbe.PeriodSeconds = ptr.Int32(-1)
			},
		),
		Entry(
			"ReadinessProbe.PeriodSeconds must be >= 1",
			"spec.Pod.ReadinessProbe.PeriodSeconds: Invalid value: 0: must be >= 1",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.ReadinessProbe.PeriodSeconds = ptr.Int32(0)
			},
		),
		Entry(
			"ReadinessProbe.SuccessThreshold must be positive",
			"spec.Pod.ReadinessProbe.SuccessThreshold: Invalid value: -1: must be >= 1",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.ReadinessProbe.SuccessThreshold = ptr.Int32(-1)
			},
		),
		Entry(
			"ReadinessProbe.SuccessThreshold must be >= 1",
			"spec.Pod.ReadinessProbe.SuccessThreshold: Invalid value: 0: must be >= 1",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.ReadinessProbe.SuccessThreshold = ptr.Int32(0)
			},
		),
		Entry(
			"ReadinessProbe.TimeoutSeconds must be positive",
			"spec.Pod.ReadinessProbe.TimeoutSeconds: Invalid value: -1: must be >= 1",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.ReadinessProbe.TimeoutSeconds = ptr.Int32(-1)
			},
		),
		Entry(
			"ReadinessProbe.TimeoutSeconds must be >= 1",
			"spec.Pod.ReadinessProbe.TimeoutSeconds: Invalid value: 0: must be >= 1",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.ReadinessProbe.TimeoutSeconds = ptr.Int32(0)
			},
		),

		Entry(
			"Snapshot.Schedule must not be empty",
			"spec.Snapshot.Schedule: Invalid value: \"\": is not a valid cron expression (Empty spec string)",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Snapshot.Schedule = ""
			},
		),
		Entry(
			"Snapshot.Schedule must be valid cron syntax",
			"spec.Snapshot.Schedule: Invalid value: \"x y z\": is not a valid cron expression (Expected 5 to 6 fields, found 3: x y z)",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Snapshot.Schedule = "x y z"
			},
		),
		Entry(
			"Snapshot.TimeoutSeconds must be positive",
			"spec.Snapshot.TimeoutSeconds: Invalid value: -1: must be >= 1",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Snapshot.TimeoutSeconds = ptr.Int32(-1)
			},
		),
		Entry(
			"Snapshot.RetentionPolicy.RetentionPeriodDays must be positive",
			"spec.Snapshot.RetentionPolicy.RetentionPeriodDays: Invalid value: -1: must be >= 0",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Snapshot.RetentionPolicy.RetentionPeriodDays = ptr.Int32(-1)
			},
		),
		Entry(
			"Snapshot.RetentionPolicy.CleanupTimeoutSeconds must be positive",
			"spec.Snapshot.RetentionPolicy.CleanupTimeoutSeconds: Invalid value: -1: must be >= 0",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Snapshot.RetentionPolicy.CleanupTimeoutSeconds = ptr.Int32(-1)
			},
		),
		Entry(
			"Snapshot.RetentionPolicy.CleanupSchedule must not be empty",
			"spec.Snapshot.RetentionPolicy.CleanupSchedule: Invalid value: \"\": is not a valid cron expression (Empty spec string)",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Snapshot.RetentionPolicy.CleanupSchedule = ""
			},
		),
		Entry(
			"Snapshot.RetentionPolicy.CleanupSchedule must be valid cron syntax",
			"spec.Snapshot.RetentionPolicy.CleanupSchedule: Invalid value: \"x y z\": is not a valid cron expression (Expected 5 to 6 fields, found 3: x y z)",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Snapshot.RetentionPolicy.CleanupSchedule = "x y z"
			},
		),
	)
	DescribeTable(
		"failure cases when usingEmpty",
		func(expectedMessage string, mutate func(*v1alpha1.Cassandra)) {
			c := aValidCassandraUsingEmptyDir()
			mutate(c)
			err := validation.ValidateCassandra(c).ToAggregate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(expectedMessage))
		},
		Entry(
			"Rack.Zone must not be empty when UseEmptyDir=false",
			"spec.Racks.rack1.Zone: Invalid value: \"eu-west1-a\": must be set to \"\" when spec.useEmptyDir is true",
			func(c *v1alpha1.Cassandra) {
				c.Spec.UseEmptyDir = ptr.Bool(true)
				c.Spec.Racks[0].Zone = "eu-west1-a"
			},
		),
	)
})

var _ = Describe("ValidateCassandraUpdate", func() {
	DescribeTable(
		"allowed changes",
		func(mutate func(*v1alpha1.Cassandra)) {
			c1 := aValidCassandra()
			c2 := c1.DeepCopy()
			mutate(c2)
			err := validation.ValidateCassandraUpdate(c1, c2).ToAggregate()
			Expect(err).ToNot(HaveOccurred())
		},
		Entry(
			"No changes",
			func(c *v1alpha1.Cassandra) {},
		),
		Entry(
			"Rack Added",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Racks = append(
					c.Spec.Racks,
					rackSpec("c"),
				)
			},
		),
		Entry(
			"Rack scale out",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Racks[0].Replicas += 1
			},
		),
		Entry(
			"Pod BootstrapperImage changed",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.BootstrapperImage = ptr.String("foo/bar:baz")
			},
		),
		Entry(
			"Pod SidecarImage changed",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.SidecarImage = ptr.String("foo/bar:baz")
			},
		),
		Entry(
			"Pod Requests Memory changed",
			func(c *v1alpha1.Cassandra) {
				quantity := c.Spec.Pod.Resources.Requests[coreV1.ResourceMemory]
				quantity.Sub(resource.MustParse("1Mi"))
				c.Spec.Pod.Resources.Requests[coreV1.ResourceMemory] = quantity
			},
		),
		Entry(
			"Pod Limit Memory changed",
			func(c *v1alpha1.Cassandra) {
				quantity := c.Spec.Pod.Resources.Limits[coreV1.ResourceMemory]
				quantity.Add(resource.MustParse("1Mi"))
				c.Spec.Pod.Resources.Limits[coreV1.ResourceMemory] = quantity
			},
		),
		Entry(
			"Pod Request CPU changed",
			func(c *v1alpha1.Cassandra) {
				quantity := c.Spec.Pod.Resources.Requests[coreV1.ResourceCPU]
				quantity.Sub(resource.MustParse("2000"))
				c.Spec.Pod.Resources.Requests[coreV1.ResourceCPU] = quantity
			},
		),
		Entry(
			"Pod Limit Cpu changed",
			func(c *v1alpha1.Cassandra) {
				quantity := c.Spec.Pod.Resources.Limits[coreV1.ResourceCPU]
				quantity.Add(resource.MustParse("150"))
				c.Spec.Pod.Resources.Limits[coreV1.ResourceCPU] = quantity
			},
		),
		Entry(
			"Pod Request Cpu removed",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.Resources.Requests[coreV1.ResourceCPU] = resource.Quantity{}
			},
		),
		Entry(
			"Pod Limit Cpu removed",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.Resources.Limits[coreV1.ResourceCPU] = resource.Quantity{}
			},
		),
		Entry(
			"Pod LivenessProbe Changed",
			func(c *v1alpha1.Cassandra) {
				*c.Spec.Pod.LivenessProbe.PeriodSeconds += 1
			},
		),
		Entry(
			"Pod ReadinessProbe Changed",
			func(c *v1alpha1.Cassandra) {
				*c.Spec.Pod.ReadinessProbe.PeriodSeconds += 1
			},
		),
		Entry(
			"Snapshot Image Changed",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Snapshot.Image = ptr.String("foo/bar:baz")
			},
		),
		Entry(
			"Snapshot Schedule Changed",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Snapshot.Schedule = "* * * * *"
			},
		),
		Entry(
			"Snapshot Keyspace added",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Snapshot.Keyspaces = append(
					c.Spec.Snapshot.Keyspaces,
					"another-keyspace",
				)
			},
		),
		Entry(
			"Snapshot Keyspace removed",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Snapshot.Keyspaces = c.Spec.Snapshot.Keyspaces[:1]
			},
		),
		Entry(
			"Snapshot TimeoutSeconds changed",
			func(c *v1alpha1.Cassandra) {
				*c.Spec.Snapshot.TimeoutSeconds += 1
			},
		),
		Entry(
			"RetentionPolicy Enabled changed",
			func(c *v1alpha1.Cassandra) {
				*c.Spec.Snapshot.RetentionPolicy.Enabled = !*c.Spec.Snapshot.RetentionPolicy.Enabled
			},
		),
		Entry(
			"RetentionPolicy PeriodDays changed",
			func(c *v1alpha1.Cassandra) {
				*c.Spec.Snapshot.RetentionPolicy.RetentionPeriodDays += 1
			},
		),
		Entry(
			"RetentionPolicy CleanupSchedule changed",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Snapshot.RetentionPolicy.CleanupSchedule = "* * * * *"
			},
		),
		Entry(
			"RetentionPolicy CleanupTimeoutSeconds changed",
			func(c *v1alpha1.Cassandra) {
				*c.Spec.Snapshot.RetentionPolicy.CleanupTimeoutSeconds += 1
			},
		),
	)
	DescribeTable(
		"forbidden changes",
		func(expectedMessage string, mutate func(*v1alpha1.Cassandra)) {
			c1 := aValidCassandra()
			c2 := c1.DeepCopy()
			mutate(c2)
			errors := validation.ValidateCassandraUpdate(c1, c2)
			err := errors.ToAggregate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(expectedMessage))
		},
		Entry(
			"Invalid new cluster",
			"spec.Racks: Required value",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Racks = []v1alpha1.Rack{}
			},
		),
		Entry(
			"Datacenter",
			"spec.Datacenter: Forbidden: This field can not be changed: current: \"dc1\", new: \"a-different-datacenter\"",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Datacenter = ptr.String("a-different-datacenter")
			},
		),
		Entry(
			"Pod.Image",
			"spec.Pod.Image: Forbidden: This field can not be changed: current: \"cassandra:3.11.4\", new: \"a-different/image:1.2.3\"",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.Image = ptr.String("a-different/image:1.2.3")
			},
		),
		Entry(
			"Rack Storage",
			"spec.Racks.rack1.Storage: Forbidden: This field can not be changed",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Racks[0].Storage[0].Path = ptr.String("some other path")
			},
		),
		Entry(
			"UseEmptyDir",
			"spec.UseEmptyDir: Forbidden: This field can not be changed: current: false, new: true",
			func(c *v1alpha1.Cassandra) {
				c.Spec.UseEmptyDir = ptr.Bool(true)
				makeCompatibleWithUsingEmptyDir(c)
			},
		),
		Entry(
			"Rack deletion",
			"spec.Racks: Forbidden: Rack deletion is not supported. Missing Racks: rack2",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Racks = c.Spec.Racks[:1]
			},
		),
		Entry(
			"Rack.Zone",
			"spec.Racks.rack1.Zone: Forbidden: This field can not be changed: current: zone1, new: new-zone",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Racks[0].Zone = "new-zone"
			},
		),
		Entry(
			"Rack.Replicas decrease (scale-in)",
			"spec.Racks.rack1.Replicas: Forbidden: This field can not be decremented (scale-in is not yet supported): current: 2, new: 1",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Racks[0].Replicas -= 1
			},
		),
	)
})

func aValidCassandraUsingEmptyDir() *v1alpha1.Cassandra {
	cassandra := aValidCassandra().DeepCopy()
	makeCompatibleWithUsingEmptyDir(cassandra)
	return cassandra
}

func makeCompatibleWithUsingEmptyDir(cassandra *v1alpha1.Cassandra) {
	for i := range cassandra.Spec.Racks {
		cassandra.Spec.Racks[i].Zone = ""
	}
}

func aValidCassandra() *v1alpha1.Cassandra {
	return &v1alpha1.Cassandra{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster1",
			Namespace: "ns1",
		},
		Spec: v1alpha1.CassandraSpec{
			Datacenter:  ptr.String("dc1"),
			UseEmptyDir: ptr.Bool(false),
			Racks:       []v1alpha1.Rack{rackSpec("rack1"), rackSpec("rack2")},
			Pod: v1alpha1.Pod{
				Image: ptr.String("cassandra:3.11.4"),
				Resources: coreV1.ResourceRequirements{
					Requests: coreV1.ResourceList{
						coreV1.ResourceCPU:    resource.MustParse("0"),
						coreV1.ResourceMemory: resource.MustParse("1Gi"),
					},
					Limits: coreV1.ResourceList{
						coreV1.ResourceMemory: resource.MustParse("1Gi"),
					},
				},
				LivenessProbe: &v1alpha1.Probe{
					FailureThreshold:    ptr.Int32(1),
					InitialDelaySeconds: ptr.Int32(1),
					PeriodSeconds:       ptr.Int32(1),
					SuccessThreshold:    ptr.Int32(1),
					TimeoutSeconds:      ptr.Int32(1),
				},
				ReadinessProbe: &v1alpha1.Probe{
					FailureThreshold:    ptr.Int32(1),
					InitialDelaySeconds: ptr.Int32(1),
					PeriodSeconds:       ptr.Int32(1),
					SuccessThreshold:    ptr.Int32(1),
					TimeoutSeconds:      ptr.Int32(1),
				},
			},
			Snapshot: &v1alpha1.Snapshot{
				Schedule:       "1 23 * * *",
				Keyspaces:      []string{"keyspace1", "keyspace2"},
				TimeoutSeconds: ptr.Int32(1),
				RetentionPolicy: &v1alpha1.RetentionPolicy{
					Enabled:               ptr.Bool(true),
					RetentionPeriodDays:   ptr.Int32(1),
					CleanupTimeoutSeconds: ptr.Int32(0),
					CleanupSchedule:       "1 23 * * *",
				},
			},
		},
	}
}

func rackSpec(name string) v1alpha1.Rack {
	return v1alpha1.Rack{
		Name:     name,
		Zone:     "zone1",
		Replicas: 2,
		Storage: []v1alpha1.Storage{
			{
				Path: ptr.String("cassandra-home"),
				StorageSource: v1alpha1.StorageSource{
					PersistentVolumeClaim: &coreV1.PersistentVolumeClaimSpec{
						Resources: coreV1.ResourceRequirements{
							Requests: coreV1.ResourceList{
								coreV1.ResourceStorage: resource.MustParse("100Gi"),
							},
						},
					},
				},
			},
		},
	}
}
