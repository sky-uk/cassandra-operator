package helpers

import (
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/ptr"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestHelpers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Helpers Suite", test.CreateParallelReporters("helpers"))
}

var _ = Describe("Cassandra Helpers", func() {

	Context("Snapshot Retention", func() {
		var snapshot *v1alpha1.Snapshot
		BeforeEach(func() {
			snapshot = &v1alpha1.Snapshot{
				Schedule: "01 23 * * *",
			}
		})

		It("should be found disabled when no retention policy is defined", func() {
			Expect(HasRetentionPolicyEnabled(snapshot)).To(BeFalse())
		})

		It("should be found disabled when retention policy is not enabled", func() {
			snapshot.RetentionPolicy = &v1alpha1.RetentionPolicy{
				Enabled:         false,
				CleanupSchedule: "11 11 * * *",
			}
			Expect(HasRetentionPolicyEnabled(snapshot)).To(BeFalse())
		})

		It("should be found enabled when retention policy is enabled", func() {
			snapshot.RetentionPolicy = &v1alpha1.RetentionPolicy{
				Enabled:         true,
				CleanupSchedule: "11 11 * * *",
			}
			Expect(HasRetentionPolicyEnabled(snapshot)).To(BeTrue())
		})
	})

	Context("Snapshot Properties", func() {
		var (
			snapshotTimeout = int32(10)
			snapshot1       *v1alpha1.Snapshot
			snapshot2       *v1alpha1.Snapshot
		)

		BeforeEach(func() {
			snapshot1 = &v1alpha1.Snapshot{
				Schedule:       "01 23 * * *",
				TimeoutSeconds: &snapshotTimeout,
				Keyspaces:      []string{"keyspace1", "keyspace2"},
			}
			snapshot2 = &v1alpha1.Snapshot{
				Schedule:       "01 23 * * *",
				TimeoutSeconds: &snapshotTimeout,
				Keyspaces:      []string{"keyspace1", "keyspace2"},
			}
		})

		It("should be found equal when snapshots have the same properties values", func() {
			Expect(SnapshotPropertiesUpdated(snapshot1, snapshot2)).To(BeFalse())
			Expect(SnapshotPropertiesUpdated(snapshot2, snapshot1)).To(BeFalse())
		})

		It("should be found equal when only retention policy is different", func() {
			snapshot1.RetentionPolicy = &v1alpha1.RetentionPolicy{CleanupSchedule: "01 10 * * *"}
			Expect(SnapshotPropertiesUpdated(snapshot1, snapshot2)).To(BeFalse())
			Expect(SnapshotPropertiesUpdated(snapshot2, snapshot1)).To(BeFalse())
		})

		It("should be found different when schedule is different", func() {
			snapshot1.Schedule = "01 10 * * *"
			Expect(SnapshotPropertiesUpdated(snapshot1, snapshot2)).To(BeTrue())
			Expect(SnapshotPropertiesUpdated(snapshot2, snapshot1)).To(BeTrue())
		})

		It("should be found different when one has no timeout", func() {
			snapshot1.TimeoutSeconds = nil
			Expect(SnapshotPropertiesUpdated(snapshot1, snapshot2)).To(BeTrue())
			Expect(SnapshotPropertiesUpdated(snapshot2, snapshot1)).To(BeTrue())
		})

		It("should be found equal when both have no timeout", func() {
			snapshot1.TimeoutSeconds = nil
			snapshot2.TimeoutSeconds = nil
			Expect(SnapshotPropertiesUpdated(snapshot1, snapshot2)).To(BeFalse())
			Expect(SnapshotPropertiesUpdated(snapshot2, snapshot1)).To(BeFalse())
		})

		It("should be found different when keyspaces list are different", func() {
			snapshot1.Keyspaces = []string{"keyspace2"}
			Expect(SnapshotPropertiesUpdated(snapshot1, snapshot2)).To(BeTrue())
			Expect(SnapshotPropertiesUpdated(snapshot2, snapshot1)).To(BeTrue())
		})

		It("should be found different when a snapshot has no keyspace", func() {
			snapshot1.Keyspaces = nil
			Expect(SnapshotPropertiesUpdated(snapshot1, snapshot2)).To(BeTrue())
			Expect(SnapshotPropertiesUpdated(snapshot2, snapshot1)).To(BeTrue())
		})

		It("should be found equal when both have no keyspace", func() {
			snapshot1.Keyspaces = nil
			snapshot2.Keyspaces = nil
			Expect(SnapshotPropertiesUpdated(snapshot1, snapshot2)).To(BeFalse())
			Expect(SnapshotPropertiesUpdated(snapshot2, snapshot1)).To(BeFalse())
		})
	})

	Context("Snapshot Cleanup Properties", func() {
		var (
			cleanupTimeout = int32(10)
			snapshot1      *v1alpha1.Snapshot
			snapshot2      *v1alpha1.Snapshot
		)

		BeforeEach(func() {
			snapshot1 = &v1alpha1.Snapshot{
				Schedule:       "01 23 * * *",
				TimeoutSeconds: &cleanupTimeout,
				Keyspaces:      []string{"keyspace1", "keyspace2"},
				RetentionPolicy: &v1alpha1.RetentionPolicy{
					Enabled:               true,
					CleanupSchedule:       "11 11 * * *",
					CleanupTimeoutSeconds: ptr.Int32(10),
					RetentionPeriodDays:   ptr.Int32(7),
				},
			}
			snapshot2 = &v1alpha1.Snapshot{
				Schedule:       "01 23 * * *",
				TimeoutSeconds: &cleanupTimeout,
				Keyspaces:      []string{"keyspace1", "keyspace2"},
				RetentionPolicy: &v1alpha1.RetentionPolicy{
					Enabled:               true,
					CleanupSchedule:       "11 11 * * *",
					CleanupTimeoutSeconds: ptr.Int32(10),
					RetentionPeriodDays:   ptr.Int32(7),
				},
			}
		})

		It("should be found equal when snapshots have the same properties values", func() {
			Expect(SnapshotCleanupPropertiesUpdated(snapshot1, snapshot2)).To(BeFalse())
			Expect(SnapshotCleanupPropertiesUpdated(snapshot2, snapshot1)).To(BeFalse())
		})

		It("should be found equal when properties other than retention policy are different", func() {
			snapshot1.Schedule = "01 10 * * *"
			snapshot1.TimeoutSeconds = nil
			snapshot1.Keyspaces = nil
			Expect(SnapshotCleanupPropertiesUpdated(snapshot1, snapshot2)).To(BeFalse())
			Expect(SnapshotCleanupPropertiesUpdated(snapshot2, snapshot1)).To(BeFalse())
		})

		It("should be found equal even when one is not enabled", func() {
			snapshot1.RetentionPolicy.Enabled = false
			Expect(SnapshotCleanupPropertiesUpdated(snapshot1, snapshot2)).To(BeFalse())
			Expect(SnapshotCleanupPropertiesUpdated(snapshot2, snapshot1)).To(BeFalse())
		})

		It("should be found different when the cleanup schedule is different", func() {
			snapshot1.RetentionPolicy.CleanupSchedule = "01 10 * * *"
			Expect(SnapshotCleanupPropertiesUpdated(snapshot1, snapshot2)).To(BeTrue())
			Expect(SnapshotCleanupPropertiesUpdated(snapshot2, snapshot1)).To(BeTrue())
		})

		It("should be found different when one has no retention period", func() {
			snapshot1.RetentionPolicy.RetentionPeriodDays = nil
			Expect(SnapshotCleanupPropertiesUpdated(snapshot1, snapshot2)).To(BeTrue())
			Expect(SnapshotCleanupPropertiesUpdated(snapshot2, snapshot1)).To(BeTrue())
		})

		It("should be found different when retention period have different values", func() {
			snapshot1.RetentionPolicy.RetentionPeriodDays = ptr.Int32(1)
			Expect(SnapshotCleanupPropertiesUpdated(snapshot1, snapshot2)).To(BeTrue())
			Expect(SnapshotCleanupPropertiesUpdated(snapshot2, snapshot1)).To(BeTrue())
		})

		It("should be found different when one has no cleanup timeout", func() {
			snapshot1.RetentionPolicy.CleanupTimeoutSeconds = nil
			Expect(SnapshotCleanupPropertiesUpdated(snapshot1, snapshot2)).To(BeTrue())
			Expect(SnapshotCleanupPropertiesUpdated(snapshot2, snapshot1)).To(BeTrue())
		})

		It("should be found different when cleanup timeout have different values", func() {
			snapshot1.RetentionPolicy.CleanupTimeoutSeconds = ptr.Int32(30)
			Expect(SnapshotCleanupPropertiesUpdated(snapshot1, snapshot2)).To(BeTrue())
			Expect(SnapshotCleanupPropertiesUpdated(snapshot2, snapshot1)).To(BeTrue())
		})

	})
})