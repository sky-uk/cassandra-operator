package validation_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
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
	)

	DescribeTable(
		"failure cases",
		func(expectedMessage string, mutate func(*v1alpha1.Cassandra)) {
			c := aValidCassandra()
			mutate(c)
			err := validation.ValidateCassandra(c).ToAggregate()
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
			"Rack.StorageClass must not be empty when UseEmptyDir=false",
			"spec.Racks.rack1.StorageClass: Required value: because spec.useEmptyDir is false",
			func(c *v1alpha1.Cassandra) {
				c.Spec.UseEmptyDir = ptr.Bool(false)
				c.Spec.Racks[0].StorageClass = ""
			},
		),
		Entry(
			"Rack.Zone must not be empty when UseEmptyDir=false",
			"spec.Racks.rack1.Zone: Required value: because spec.useEmptyDir is false",
			func(c *v1alpha1.Cassandra) {
				c.Spec.UseEmptyDir = ptr.Bool(false)
				c.Spec.Racks[0].Zone = ""
			},
		),
		Entry(
			"Pod.Memory must be > 0",
			"spec.Pod.Memory: Invalid value: \"0\": must be > 0",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.Memory.Set(0)
			},
		),
		Entry(
			"Pod.StorageSize must be > 0 when UseEmptyDir=false",
			"spec.Pod.StorageSize: Invalid value: \"0\": must be > 0 when spec.useEmptyDir is false",
			func(c *v1alpha1.Cassandra) {
				c.Spec.UseEmptyDir = ptr.Bool(false)
				c.Spec.Pod.StorageSize.Set(0)
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
			"Rack.StorageClass must be empty when UseEmptyDir=true",
			"spec.Racks.rack1.StorageClass: Invalid value: \"standard-class\": must be set to \"\" when spec.useEmptyDir is true",
			func(c *v1alpha1.Cassandra) {
				c.Spec.UseEmptyDir = ptr.Bool(true)
				c.Spec.Racks[0].StorageClass = "standard-class"
			},
		),
		Entry(
			"Rack.Zone must not be empty when UseEmptyDir=false",
			"spec.Racks.rack1.Zone: Invalid value: \"eu-west1-a\": must be set to \"\" when spec.useEmptyDir is true",
			func(c *v1alpha1.Cassandra) {
				c.Spec.UseEmptyDir = ptr.Bool(true)
				c.Spec.Racks[0].Zone = "eu-west1-a"
			},
		),
		Entry(
			"Pod.StorageSize must be 0 when UseEmptyDir=true",
			"spec.Pod.StorageSize: Invalid value: \"1\": must be 0 when spec.useEmptyDir is true",
			func(c *v1alpha1.Cassandra) {
				c.Spec.UseEmptyDir = ptr.Bool(true)
				c.Spec.Pod.StorageSize.Set(1)
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
					v1alpha1.Rack{
						Zone:         "zone3",
						StorageClass: "fast",
						Replicas:     2,
					},
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
			"Pod Memory Increased",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.Memory.Add(resource.MustParse("1Mi"))
			},
		),
		Entry(
			"Pod Memory Decreased",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.Memory.Sub(resource.MustParse("1Mi"))
			},
		),
		Entry(
			"Pod CPU Increased",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.CPU.Add(resource.MustParse("1m"))
			},
		),
		Entry(
			"Pod CPU Decreased",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.CPU.Sub(resource.MustParse("1m"))
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
			"Pod.StorageSize",
			"spec.Pod.StorageSize: Forbidden: This field can not be changed: current: 100Gi, new: 101Gi",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Pod.StorageSize.Add(resource.MustParse("1Gi"))
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
			"Rack.StorageClass",
			"spec.Racks.rack1.StorageClass: Forbidden: This field can not be changed: current: fast, new: new-storageclass",
			func(c *v1alpha1.Cassandra) {
				c.Spec.Racks[0].StorageClass = "new-storageclass"
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
	cassandra.Spec.Pod.StorageSize = resource.Quantity{}
	for i := range cassandra.Spec.Racks {
		cassandra.Spec.Racks[i].StorageClass = ""
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
			Racks: []v1alpha1.Rack{
				{
					Name:         "rack1",
					Zone:         "zone1",
					StorageClass: "fast",
					Replicas:     2,
				},
				{
					Name:         "rack2",
					Zone:         "zone2",
					StorageClass: "fast",
					Replicas:     2,
				},
			},
			Pod: v1alpha1.Pod{
				Image:       ptr.String("cassandra:3.11.4"),
				CPU:         resource.MustParse("0"),
				Memory:      resource.MustParse("1Gi"),
				StorageSize: resource.MustParse("100Gi"),
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
