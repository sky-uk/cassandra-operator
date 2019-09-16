package cluster

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/ptr"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test/apis"
	"k8s.io/api/batch/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"testing"
)

func TestSnapshotState(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Snapshot State Suite", test.CreateParallelReporters("snapshotstate"))
}

var _ = Describe("current snapshot state", func() {

	Describe("the Cassandra snapshot job state reconstruction", func() {
		var (
			stateFinder currentSnapshotStateFinder
			fakes       *mocks
		)

		BeforeEach(func() {
			fakes = &mocks{
				client:        &mockClient{},
				objectFactory: &mockObjectFactory{},
			}
			stateFinder = currentSnapshotStateFinder{
				client:        fakes.client,
				objectFactory: fakes.objectFactory,
			}
		})

		AfterEach(func() {
			fakes.client.AssertExpectations(GinkgoT())
			fakes.objectFactory.AssertExpectations(GinkgoT())
		})

		Context("when a snapshot job does not exists", func() {
			It("should be nil when no snapshot cleanup job is found", func() {
				desiredCassandra := aClusterDefinition()
				fakes.snapshotJobIsNotFoundFor(desiredCassandra)

				snapshotCleanup, err := stateFinder.findSnapshotStateFor(desiredCassandra)
				Expect(err).To(HaveOccurred())
				Expect(errors.IsNotFound(err)).To(BeTrue())
				Expect(snapshotCleanup).To(BeNil())
			})
		})

		Context("when a snapshot job exists", func() {
			var (
				currentCassandra *v1alpha1.Cassandra
				desiredCassandra *v1alpha1.Cassandra
				expectedSnapshot *v1alpha1.Snapshot
			)

			BeforeEach(func() {
				currentCassandra = apis.ACassandra().WithDefaults().WithSpec(
					apis.ACassandraSpec().
						WithDefaults().
						WithSnapshot(
							apis.ASnapshot().
								WithDefaults().
								WithSchedule("* * * * 1").
								WithTimeoutSeconds(1).
								WithKeyspaces([]string{"keyspaces"}).
								WithImage("some image"))).
					Build()
				desiredCassandra = currentCassandra.DeepCopy()
				desiredCassandra.Spec.Snapshot = nil

			})

			AfterEach(func() {
				fakes.snapshotJobIsFoundFor(desiredCassandra, createSnapshotJobFor(currentCassandra))

				currentSnapshotState, err := stateFinder.findSnapshotStateFor(desiredCassandra)
				Expect(err).NotTo(HaveOccurred())
				Expect(currentSnapshotState).To(Equal(expectedSnapshot))
			})

			It("should contain the only properties that can be safely extracted from the cronjob", func() {
				expectedSnapshot = &v1alpha1.Snapshot{
					Image:    ptr.String("some image"),
					Schedule: "* * * * 1",
				}
			})

			It("should contain the snapshot properties derived from the desired cassandra when they match", func() {
				desiredCassandra.Spec.Snapshot = currentCassandra.Spec.Snapshot.DeepCopy()
				expectedSnapshot = desiredCassandra.Spec.Snapshot.DeepCopy()
			})

		})
	})

	Describe("the Cassandra snapshot cleanup job state reconstruction", func() {
		var (
			stateFinder currentSnapshotCleanupStateFinder
			fakes       *mocks
		)

		BeforeEach(func() {
			fakes = &mocks{
				client:        &mockClient{},
				objectFactory: &mockObjectFactory{},
			}
			stateFinder = currentSnapshotCleanupStateFinder{
				client:        fakes.client,
				objectFactory: fakes.objectFactory,
			}
		})

		AfterEach(func() {
			fakes.client.AssertExpectations(GinkgoT())
			fakes.objectFactory.AssertExpectations(GinkgoT())
		})

		Context("when a snapshot cleanup job does not exists", func() {
			It("should be nil when no snapshot cleanup job is found", func() {
				desiredCassandra := aClusterDefinition()
				fakes.snapshotJobCleanupIsNotFoundFor(desiredCassandra)

				snapshotCleanup, err := stateFinder.findSnapshotCleanupStateFor(desiredCassandra)
				Expect(err).To(HaveOccurred())
				Expect(errors.IsNotFound(err)).To(BeTrue())
				Expect(snapshotCleanup).To(BeNil())
			})
		})

		Context("when a snapshot cleanup job exists", func() {
			var (
				currentCassandra        *v1alpha1.Cassandra
				desiredCassandra        *v1alpha1.Cassandra
				expectedSnapshotCleanup *v1alpha1.RetentionPolicy
			)

			BeforeEach(func() {
				currentCassandra = apis.ACassandra().WithDefaults().WithSpec(
					apis.ACassandraSpec().
						WithDefaults().
						WithSnapshot(
							apis.ASnapshot().
								WithDefaults().
								WithRetentionPolicy(
									apis.ARetentionPolicy().
										WithDefaults().
										WithTimeoutSeconds(1).
										WithCleanupScheduled("* * * * 1").
										WithRetentionPeriodDays(60)))).
					Build()
				desiredCassandra = currentCassandra.DeepCopy()
				desiredCassandra.Spec.Snapshot.RetentionPolicy = nil
			})

			AfterEach(func() {
				fakes.snapshotCleanupJobIsFoundFor(desiredCassandra, createSnapshotCleanupJobFor(currentCassandra))

				currentSnapshotState, err := stateFinder.findSnapshotCleanupStateFor(desiredCassandra)
				Expect(err).NotTo(HaveOccurred())
				Expect(currentSnapshotState).To(Equal(expectedSnapshotCleanup))
			})

			It("should contain the only properties that can be safely extracted from the cronjob", func() {
				expectedSnapshotCleanup = &v1alpha1.RetentionPolicy{
					Enabled:         ptr.Bool(true),
					CleanupSchedule: "* * * * 1",
				}
			})

			It("should contain the snapshot properties derived from the desired cassandra when they match", func() {
				desiredCassandra.Spec.Snapshot.RetentionPolicy = currentCassandra.Spec.Snapshot.RetentionPolicy.DeepCopy()
				expectedSnapshotCleanup = desiredCassandra.Spec.Snapshot.RetentionPolicy.DeepCopy()
			})
		})
	})

})

func createSnapshotJobFor(clusterDef *v1alpha1.Cassandra) *v1beta1.CronJob {
	c := New(clusterDef)
	return c.CreateSnapshotJob()
}

func createSnapshotCleanupJobFor(clusterDef *v1alpha1.Cassandra) *v1beta1.CronJob {
	c := New(clusterDef)
	return c.CreateSnapshotCleanupJob()
}
