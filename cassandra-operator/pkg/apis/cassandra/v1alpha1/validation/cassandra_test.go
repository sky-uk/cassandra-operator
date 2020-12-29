package validation_test

import (
	"fmt"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test/apis"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1/validation"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/ptr"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
		Entry(
			"Snapshot cpu request can be provided without cpu limit",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Snapshot.Resources.Requests[coreV1.ResourceCPU] = resource.MustParse("151m")
				delete(c.Spec.Snapshot.Resources.Limits, coreV1.ResourceCPU)
			},
		),
		Entry(
			"Snapshot cpu limit can be provided without cpu request",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Snapshot.Resources.Limits[coreV1.ResourceCPU] = resource.MustParse("151m")
				delete(c.Spec.Snapshot.Resources.Requests, coreV1.ResourceCPU)
			},
		),
		Entry(
			"Both Snapshot cpu request and limit can be omitted",
			func(c *v1alpha1.Cassandra) {
				delete(c.Spec.Snapshot.Resources.Limits, coreV1.ResourceCPU)
				delete(c.Spec.Snapshot.Resources.Requests, coreV1.ResourceCPU)
			},
		),
		Entry(
			"Snapshot.RetentionPolicy cpu request can be provided without cpu limit",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Snapshot.RetentionPolicy.Resources.Requests[coreV1.ResourceCPU] = resource.MustParse("151m")
				delete(c.Spec.Snapshot.RetentionPolicy.Resources.Limits, coreV1.ResourceCPU)
			},
		),
		Entry(
			"Snapshot.RetentionPolicy cpu limit can be provided without cpu request",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Snapshot.RetentionPolicy.Resources.Limits[coreV1.ResourceCPU] = resource.MustParse("151m")
				delete(c.Spec.Snapshot.RetentionPolicy.Resources.Requests, coreV1.ResourceCPU)
			},
		),
		Entry(
			"Both Snapshot.RetentionPolicy cpu request and limit can be omitted",
			func(c *v1alpha1.Cassandra) {
				delete(c.Spec.Snapshot.RetentionPolicy.Resources.Limits, coreV1.ResourceCPU)
				delete(c.Spec.Snapshot.RetentionPolicy.Resources.Requests, coreV1.ResourceCPU)
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
			"spec.Racks.a.Replicas: Invalid value: -1: must be >= 1",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Racks[0].Replicas = -1
			},
		),
		Entry(
			"Rack.Replicas must be >= 1",
			"spec.Racks.a.Replicas: Invalid value: 0: must be >= 1",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Racks[0].Replicas = 0
			},
		),
		Entry(
			"Rack storage is required",
			"spec.Racks.a.Storage: Required value: at least one storage is required",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Racks[0].Storage = []v1alpha1.Storage{}
			},
		),
		Entry(
			"Rack storage must contain one storage source",
			"spec.Racks.a.Storage[0]: Required value: one storage source is required",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Racks[0].Storage[0].StorageSource = v1alpha1.StorageSource{}
			},
		),
		Entry(
			"Rack storage  may contain at most one volume type",
			"spec.Racks.a.Storage[0]: Forbidden: only one storage source per storage is allowed",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Racks[0].Storage[0].StorageSource = v1alpha1.StorageSource{
					PersistentVolumeClaim: &coreV1.PersistentVolumeClaimSpec{
						Resources: coreV1.ResourceRequirements{
							Requests: coreV1.ResourceList{
								coreV1.ResourceStorage: resource.MustParse("100Gi"),
							},
						},
					},
					EmptyDir: &coreV1.EmptyDirVolumeSource{},
				}
			},
		),
		Entry(
			"Rack storage size is required",
			"spec.Racks.a.Storage[0].persistentVolumeClaim.resources[storage]: Required value: a storage size is required",
			func(c *v1alpha1.Cassandra) {
				delete(c.Spec.Racks[0].Storage[0].PersistentVolumeClaim.Resources.Requests, coreV1.ResourceStorage)
			},
		),
		Entry(
			"Rack storage path is required",
			"spec.Racks.a.Storage[0].Path: Required value: a volume path is required",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Racks[0].Storage[0].Path = nil
			},
		),
		Entry(
			"Rack storage may not contain multiple storage for the same path",
			"spec.Racks.a.Storage: Forbidden: multiple storages have the same path '/some-path'",
			func(c *v1alpha1.Cassandra) {
				aStorage := apis.APersistentVolume().OfSize("1Mi").AtPath("/some-path").Build()
				otherStorage := apis.AnEmptyDir().AtPath("/some-path").Build()
				rackStorage := []v1alpha1.Storage{aStorage, otherStorage}
				c.Spec.Racks[0].Storage = rackStorage
			},
		),
		Entry(
			"Rack storage /etc/cassandra is a reserved path",
			"spec.Racks.a.Storage[0]: Forbidden: Storage path '/etc/cassandra' is reserved for the operator",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Racks[0].Storage[0].Path = ptr.String("/etc/cassandra")
			},
		),
		Entry(
			"Rack storage /extra-lib is a reserved path",
			"spec.Racks.a.Storage[0]: Forbidden: Storage path '/extra-lib' is reserved for the operator",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Racks[0].Storage[0].Path = ptr.String("/extra-lib")
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
			"Pod memory request > memory limit",
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
		Entry(
			"Snapshot.Resources must not be nil",
			"spec.Snapshot.Resources.Requests.Memory: Invalid value: \"0\": must be > 0",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Snapshot.Resources.Requests = coreV1.ResourceList{coreV1.ResourceMemory: resource.Quantity{}}
			},
		),
		Entry(
			"Snapshot.Resources.Requests.Memory must be > 0",
			"spec.Snapshot.Resources.Requests.Memory: Invalid value: \"0\": must be > 0",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Snapshot.Resources.Requests = coreV1.ResourceList{coreV1.ResourceMemory: resource.Quantity{}}
			},
		),
		Entry(
			"Snapshot.Resources cpu request > cpu limit",
			"spec.Snapshot.Resources.Requests.Cpu: Invalid value: \"71m\": must not be greater than spec.Snapshot.Resources.Limits.Cpu (70m)",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Snapshot.Resources.Requests = coreV1.ResourceList{
					coreV1.ResourceCPU:    resource.MustParse("71m"),
					coreV1.ResourceMemory: resource.MustParse("55Mi"),
				}
				c.Spec.Snapshot.Resources.Limits = coreV1.ResourceList{
					coreV1.ResourceCPU:    resource.MustParse("70m"),
					coreV1.ResourceMemory: resource.MustParse("55Mi"),
				}
			},
		),
		Entry(
			"Snapshot.Resources.Requests memory request > memory limit",
			"spec.Snapshot.Resources.Requests.Memory: Invalid value: \"71Mi\": must not be greater than spec.Snapshot.Resources.Limits.Memory (70Mi)",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Snapshot.Resources.Requests = coreV1.ResourceList{coreV1.ResourceMemory: resource.MustParse("71Mi")}
				c.Spec.Snapshot.Resources.Limits = coreV1.ResourceList{coreV1.ResourceMemory: resource.MustParse("70Mi")}
			},
		),
		Entry(
			"Snapshot.RetentionPolicy.Resources.Requests.Memory must be > 0",
			"spec.Snapshot.RetentionPolicy.Resources.Requests.Memory: Invalid value: \"0\": must be > 0",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Snapshot.RetentionPolicy.Resources.Requests = coreV1.ResourceList{coreV1.ResourceMemory: resource.Quantity{}}
			},
		),
		Entry(
			"Snapshot.RetentionPolicy.Resources cpu request > cpu limit",
			"spec.Snapshot.RetentionPolicy.Resources.Requests.Cpu: Invalid value: \"61m\": must not be greater than spec.Snapshot.RetentionPolicy.Resources.Limits.Cpu (60m)",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Snapshot.RetentionPolicy.Resources.Requests = coreV1.ResourceList{
					coreV1.ResourceCPU:    resource.MustParse("61m"),
					coreV1.ResourceMemory: resource.MustParse("54Mi"),
				}
				c.Spec.Snapshot.RetentionPolicy.Resources.Limits = coreV1.ResourceList{
					coreV1.ResourceCPU:    resource.MustParse("60m"),
					coreV1.ResourceMemory: resource.MustParse("54Mi"),
				}
			},
		),
		Entry(
			"Snapshot.RetentionPolicy.Resources memory request > memory limit",
			"spec.Snapshot.RetentionPolicy.Resources.Requests.Memory: Invalid value: \"61Mi\": must not be greater than spec.Snapshot.RetentionPolicy.Resources.Limits.Memory (60Mi)",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Snapshot.RetentionPolicy.Resources.Requests = coreV1.ResourceList{coreV1.ResourceMemory: resource.MustParse("61Mi")}
				c.Spec.Snapshot.RetentionPolicy.Resources.Limits = coreV1.ResourceList{coreV1.ResourceMemory: resource.MustParse("60Mi")}
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
		Entry(
			"Additional rack Storage may be added",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Racks[0].Storage = append(c.Spec.Racks[0].Storage, apis.AnEmptyDir().AtPath("another path").Build())
				c.Spec.Racks[0].Storage = append(c.Spec.Racks[0].Storage, apis.APersistentVolume().OfSize("1Gi").AtPath("yes another path").Build())
			},
		),
		Entry(
			"EmptyDir rack Storage may be removed",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Racks[0].Storage = c.Spec.Racks[0].Storage[:1]
			},
		),
		Entry(
			"Storage path may be changed for an EmptyDir rack Storage",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Racks[0].Storage[1].Path = ptr.String("/var/another-emptydir")
			},
		),
		Entry(
			"Snapshot Requests Memory changed",
			func(c *v1alpha1.Cassandra) {
				quantity := c.Spec.Snapshot.Resources.Requests[coreV1.ResourceMemory]
				quantity.Sub(resource.MustParse("1Mi"))
				c.Spec.Snapshot.Resources.Requests[coreV1.ResourceMemory] = quantity
			},
		),
		Entry(
			"Snapshot Limit Memory changed",
			func(c *v1alpha1.Cassandra) {
				quantity := c.Spec.Snapshot.Resources.Limits[coreV1.ResourceMemory]
				quantity.Add(resource.MustParse("1Mi"))
				c.Spec.Snapshot.Resources.Limits[coreV1.ResourceMemory] = quantity
			},
		),
		Entry(
			"Snapshot Request CPU changed",
			func(c *v1alpha1.Cassandra) {
				quantity := c.Spec.Snapshot.Resources.Requests[coreV1.ResourceCPU]
				quantity.Sub(resource.MustParse("2000"))
				c.Spec.Snapshot.Resources.Requests[coreV1.ResourceCPU] = quantity
			},
		),
		Entry(
			"Snapshot Limit Cpu changed",
			func(c *v1alpha1.Cassandra) {
				quantity := c.Spec.Snapshot.Resources.Limits[coreV1.ResourceCPU]
				quantity.Add(resource.MustParse("150"))
				c.Spec.Snapshot.Resources.Limits[coreV1.ResourceCPU] = quantity
			},
		),
		Entry(
			"Snapshot Request Cpu removed",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Snapshot.Resources.Requests[coreV1.ResourceCPU] = resource.Quantity{}
			},
		),
		Entry(
			"Snapshot Limit Cpu removed",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Snapshot.Resources.Limits[coreV1.ResourceCPU] = resource.Quantity{}
			},
		),
		Entry(
			"Snapshot.RetentionPolicy Requests Memory changed",
			func(c *v1alpha1.Cassandra) {
				quantity := c.Spec.Snapshot.RetentionPolicy.Resources.Requests[coreV1.ResourceMemory]
				quantity.Sub(resource.MustParse("1Mi"))
				c.Spec.Snapshot.RetentionPolicy.Resources.Requests[coreV1.ResourceMemory] = quantity
			},
		),
		Entry(
			"Snapshot.RetentionPolicy Limit Memory changed",
			func(c *v1alpha1.Cassandra) {
				quantity := c.Spec.Snapshot.RetentionPolicy.Resources.Limits[coreV1.ResourceMemory]
				quantity.Add(resource.MustParse("1Mi"))
				c.Spec.Snapshot.RetentionPolicy.Resources.Limits[coreV1.ResourceMemory] = quantity
			},
		),
		Entry(
			"Snapshot.RetentionPolicy Request CPU changed",
			func(c *v1alpha1.Cassandra) {
				quantity := c.Spec.Snapshot.RetentionPolicy.Resources.Requests[coreV1.ResourceCPU]
				quantity.Sub(resource.MustParse("2000"))
				c.Spec.Snapshot.RetentionPolicy.Resources.Requests[coreV1.ResourceCPU] = quantity
			},
		),
		Entry(
			"Snapshot.RetentionPolicy Limit Cpu changed",
			func(c *v1alpha1.Cassandra) {
				quantity := c.Spec.Snapshot.RetentionPolicy.Resources.Limits[coreV1.ResourceCPU]
				quantity.Add(resource.MustParse("150"))
				c.Spec.Snapshot.RetentionPolicy.Resources.Limits[coreV1.ResourceCPU] = quantity
			},
		),
		Entry(
			"Snapshot.RetentionPolicy Request Cpu removed",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Snapshot.RetentionPolicy.Resources.Requests[coreV1.ResourceCPU] = resource.Quantity{}
			},
		),
		Entry(
			"Snapshot.RetentionPolicy Limit Cpu removed",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Snapshot.RetentionPolicy.Resources.Limits[coreV1.ResourceCPU] = resource.Quantity{}
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
			"spec.Datacenter: Forbidden: This field can not be changed: current: \"my-dc\", new: \"a-different-datacenter\"",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Datacenter = ptr.String("a-different-datacenter")
			},
		),
		Entry(
			"Pod.Image",
			"spec.Pod.Image: Forbidden: This field can not be changed: current: \"cassandra:3.11\", new: \"a-different/image:1.2.3\"",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.Image = ptr.String("a-different/image:1.2.3")
			},
		),
		Entry(
			"Rack Storage",
			"spec.Racks.a.Storage: Forbidden: Persistent volume claims can not be changed",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Racks[0].Storage[0].PersistentVolumeClaim.StorageClassName = ptr.String("some other class")
			},
		),
		Entry(
			"Rack Storage pvc deletion",
			"spec.Racks.a.Storage: Forbidden: Persistent volume claims cannot be deleted. Missing Storage at path(s): pv-path1",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Racks[0].Storage = c.Spec.Racks[0].Storage[1:]
			},
		),
		Entry(
			"Rack deletion",
			"spec.Racks: Forbidden: Rack deletion is not supported. Missing Racks: b",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Racks = c.Spec.Racks[:1]
			},
		),
		Entry(
			"Rack.Zone",
			"spec.Racks.a.Zone: Forbidden: This field can not be changed: current: eu-west-1a, new: new-zone",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Racks[0].Zone = "new-zone"
			},
		),
		Entry(
			"Rack.Replicas decrease (scale-in)",
			"spec.Racks.a.Replicas: Forbidden: This field can not be decremented (scale-in is not yet supported): current: 2, new: 1",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Racks[0].Replicas -= 1
			},
		),
	)
})

func aValidCassandra() *v1alpha1.Cassandra {
	return apis.ACassandra().
		WithDefaults().
		WithSpec(apis.ACassandraSpec().
			WithDefaults().
			WithSnapshot(apis.ASnapshot().WithDefaults().WithRetentionPolicy(apis.ARetentionPolicy().WithDefaults())).
			WithRacks(
				apis.ARack("a", 2).WithDefaults().WithStorages(
					apis.APersistentVolume().AtPath("pv-path1").OfSize("1Gi").WithStorageClass("storage-a"),
					apis.AnEmptyDir().AtPath("path1"),
				),
				apis.ARack("b", 2).WithDefaults().WithStorages(
					apis.APersistentVolume().AtPath("pv-path1").OfSize("1Gi").WithStorageClass("storage-b"),
					apis.AnEmptyDir().AtPath("path1"),
				),
			)).
		Build()
}

func rackSpec(name string) v1alpha1.Rack {
	return apis.ARack(name, 2).WithDefaults().Build()
}
