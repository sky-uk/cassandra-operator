package helpers

import (
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test/apis"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/ptr"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test"
	coreV1 "k8s.io/api/core/v1"
)

func TestHelpers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Helpers Suite", test.CreateParallelReporters("helpers"))
}

var _ = Describe("Cassandra Helpers", func() {

	Describe("Cassandra.Spec", func() {
		var clusterDef *v1alpha1.Cassandra
		BeforeEach(func() {
			clusterDef = apis.ACassandra().WithDefaults().Build()
		})

		Describe("Defaulting Snapshot", func() {
			Context("TimeoutSeconds", func() {
				It("should default to 10", func() {
					clusterDef.Spec.Snapshot = &v1alpha1.Snapshot{}
					SetDefaultsForCassandra(clusterDef, nil)
					Expect(*clusterDef.Spec.Snapshot.TimeoutSeconds).To(Equal(int32(v1alpha1.DefaultSnapshotTimeoutSeconds)))
				})
				It("should not overwrite existing value", func() {
					clusterDef.Spec.Snapshot = &v1alpha1.Snapshot{
						TimeoutSeconds: ptr.Int32(456),
					}
					SetDefaultsForCassandra(clusterDef, nil)
					Expect(*clusterDef.Spec.Snapshot.TimeoutSeconds).To(Equal(int32(456)))
				})
			})

			It("should not change an undefined Snapshot", func() {
				clusterDef.Spec.Snapshot = nil
				SetDefaultsForCassandra(clusterDef, nil)
				Expect(clusterDef.Spec.Snapshot).To(BeNil())
			})

			Context("RetentionPolicy", func() {
				Context("RetentionPeriodDays", func() {
					It("should default to 7", func() {
						clusterDef.Spec.Snapshot = &v1alpha1.Snapshot{
							RetentionPolicy: &v1alpha1.RetentionPolicy{
								RetentionPeriodDays: nil,
							},
						}
						SetDefaultsForCassandra(clusterDef, nil)
						Expect(*clusterDef.Spec.Snapshot.RetentionPolicy.RetentionPeriodDays).To(Equal(int32(v1alpha1.DefaultRetentionPolicyRetentionPeriodDays)))
					})
					It("should not overwrite existing value", func() {
						clusterDef.Spec.Snapshot = &v1alpha1.Snapshot{
							RetentionPolicy: &v1alpha1.RetentionPolicy{
								RetentionPeriodDays: ptr.Int32(543),
							},
						}
						SetDefaultsForCassandra(clusterDef, nil)
						Expect(*clusterDef.Spec.Snapshot.RetentionPolicy.RetentionPeriodDays).To(Equal(int32(543)))
					})
				})
				Context("CleanupTimeoutSeconds", func() {
					It("should default to 10", func() {
						clusterDef.Spec.Snapshot = &v1alpha1.Snapshot{
							RetentionPolicy: &v1alpha1.RetentionPolicy{
								CleanupTimeoutSeconds: nil,
							},
						}
						SetDefaultsForCassandra(clusterDef, nil)
						Expect(*clusterDef.Spec.Snapshot.RetentionPolicy.CleanupTimeoutSeconds).To(Equal(int32(v1alpha1.DefaultRetentionPolicyCleanupTimeoutSeconds)))
					})
					It("should not overwrite existing value", func() {
						clusterDef.Spec.Snapshot = &v1alpha1.Snapshot{
							RetentionPolicy: &v1alpha1.RetentionPolicy{
								CleanupTimeoutSeconds: ptr.Int32(321),
							},
						}
						SetDefaultsForCassandra(clusterDef, nil)
						Expect(*clusterDef.Spec.Snapshot.RetentionPolicy.CleanupTimeoutSeconds).To(Equal(int32(321)))
					})
				})
				It("should not change an undefined Snapshot.RetentionPolicy", func() {
					clusterDef.Spec.Snapshot = &v1alpha1.Snapshot{}
					clusterDef.Spec.Snapshot.RetentionPolicy = nil
					SetDefaultsForCassandra(clusterDef, nil)
					Expect(clusterDef.Spec.Snapshot.RetentionPolicy).To(BeNil())
				})
			})
		})

		Describe("Defaulting datacenter", func() {
			It("should default Datacenter to dc1", func() {
				clusterDef.Spec.Datacenter = nil
				SetDefaultsForCassandra(clusterDef, nil)
				Expect(*clusterDef.Spec.Datacenter).To(Equal("dc1"))
			})
			It("should not overwrite Datacenter ", func() {
				clusterDef.Spec.Datacenter = ptr.String("carefully-chosen-datacenter-name")
				SetDefaultsForCassandra(clusterDef, nil)
				Expect(*clusterDef.Spec.Datacenter).To(Equal("carefully-chosen-datacenter-name"))
			})
		})

		Describe("Defaulting images", func() {
			Context("when image names are specified", func() {

				It("should use the specified version of the cassandra image if one is given", func() {
					clusterDef.Spec.Pod.Image = ptr.String("somerepo/someimage:v1.0")
					SetDefaultsForCassandra(clusterDef, nil)
					Expect(*clusterDef.Spec.Pod.Image).To(Equal("somerepo/someimage:v1.0"))
				})

				It("should use the specified version of the cassandra bootstrapper image if one is given", func() {
					clusterDef.Spec.Pod.BootstrapperImage = ptr.String("somerepo/some-bootstrapper-image:v1.0")
					SetDefaultsForCassandra(clusterDef, nil)
					Expect(*clusterDef.Spec.Pod.BootstrapperImage).To(Equal("somerepo/some-bootstrapper-image:v1.0"))
				})

				It("should use the specified version of the cassandra snapshot image if one is given", func() {
					clusterDef.Spec.Snapshot = &v1alpha1.Snapshot{
						Schedule: "1 23 * * *",
					}
					clusterDef.Spec.Snapshot.Image = ptr.String("somerepo/some-snapshot-image:v1.0")
					SetDefaultsForCassandra(clusterDef, nil)
					Expect(*clusterDef.Spec.Snapshot.Image).To(Equal("somerepo/some-snapshot-image:v1.0"))
				})

				It("should use the specified version of the cassandra snapshot image if one is given", func() {
					clusterDef.Spec.Pod.SidecarImage = ptr.String("somerepo/some-sidecar-image:v1.0")
					SetDefaultsForCassandra(clusterDef, nil)
					Expect(*clusterDef.Spec.Pod.SidecarImage).To(Equal("somerepo/some-sidecar-image:v1.0"))
				})

			})

			Context("when no image names are specified", func() {

				Context("and no image default scheme is provided", func() {

					It("should use the 3.11 version of the apache cassandra image", func() {
						clusterDef.Spec.Pod.Image = nil
						SetDefaultsForCassandra(clusterDef, nil)
						Expect(*clusterDef.Spec.Pod.Image).To(Equal("cassandra:3.11"))
					})

					It("should default the cassandra bootstrapper image to the latest skyuk image", func() {
						clusterDef.Spec.Pod.BootstrapperImage = nil
						SetDefaultsForCassandra(clusterDef, nil)
						Expect(*clusterDef.Spec.Pod.BootstrapperImage).To(Equal("skyuk/cassandra-bootstrapper:latest"))
					})

					It("should default the cassandra snapshot image to the latest skyuk image", func() {
						clusterDef.Spec.Snapshot = &v1alpha1.Snapshot{
							Schedule: "1 23 * * *",
						}
						clusterDef.Spec.Snapshot.Image = nil
						SetDefaultsForCassandra(clusterDef, nil)
						Expect(*clusterDef.Spec.Snapshot.Image).To(Equal("skyuk/cassandra-snapshot:latest"))
					})

					It("should default the cassandra sidecar image to the latest skyuk image", func() {
						clusterDef.Spec.Pod.SidecarImage = nil
						SetDefaultsForCassandra(clusterDef, nil)
						Expect(*clusterDef.Spec.Pod.SidecarImage).To(Equal("skyuk/cassandra-sidecar:latest"))
					})
				})

				Context("and an image default scheme is provided", func() {

					var imageDefaultsScheme *TemplatedImageScheme

					BeforeEach(func() {
						imageDefaultsScheme = &TemplatedImageScheme{
							ImageVersion:   "v123",
							RepositoryPath: "dockerRegistry/somepath",
						}
					})

					It("should ignore the image schema and use the 3.11 version of the apache cassandra image", func() {
						clusterDef.Spec.Pod.Image = nil
						SetDefaultsForCassandra(clusterDef, imageDefaultsScheme)
						Expect(*clusterDef.Spec.Pod.Image).To(Equal("cassandra:3.11"))
					})

					It("should default the cassandra bootstrapper image based on the scheme", func() {
						clusterDef.Spec.Pod.BootstrapperImage = nil
						SetDefaultsForCassandra(clusterDef, imageDefaultsScheme)
						Expect(*clusterDef.Spec.Pod.BootstrapperImage).To(Equal("dockerRegistry/somepath/cassandra-bootstrapper:v123"))
					})

					It("should default the cassandra snapshot image based on the scheme", func() {
						clusterDef.Spec.Snapshot = &v1alpha1.Snapshot{
							Schedule: "1 23 * * *",
						}
						clusterDef.Spec.Snapshot.Image = nil
						SetDefaultsForCassandra(clusterDef, imageDefaultsScheme)
						Expect(*clusterDef.Spec.Snapshot.Image).To(Equal("dockerRegistry/somepath/cassandra-snapshot:v123"))
					})

					It("should default the cassandra sidecar image based on the scheme", func() {
						clusterDef.Spec.Pod.SidecarImage = nil
						SetDefaultsForCassandra(clusterDef, imageDefaultsScheme)
						Expect(*clusterDef.Spec.Pod.SidecarImage).To(Equal("dockerRegistry/somepath/cassandra-sidecar:v123"))
					})

				})
			})

		})

		Describe("Defaulting liveness probe", func() {
			It("should set the default liveness probe values if it is not configured for the cluster", func() {
				clusterDef.Spec.Pod.LivenessProbe = nil
				SetDefaultsForCassandra(clusterDef, nil)
				Expect(clusterDef.Spec.Pod.LivenessProbe.FailureThreshold).To(Equal(ptr.Int32(3)))
				Expect(clusterDef.Spec.Pod.LivenessProbe.InitialDelaySeconds).To(Equal(ptr.Int32(30)))
				Expect(clusterDef.Spec.Pod.LivenessProbe.PeriodSeconds).To(Equal(ptr.Int32(30)))
				Expect(clusterDef.Spec.Pod.LivenessProbe.SuccessThreshold).To(Equal(ptr.Int32(1)))
				Expect(clusterDef.Spec.Pod.LivenessProbe.TimeoutSeconds).To(Equal(ptr.Int32(5)))
			})

			It("should set the default liveness probe values if the liveness probe is present but has unspecified values", func() {
				clusterDef.Spec.Pod.LivenessProbe = &v1alpha1.Probe{}
				SetDefaultsForCassandra(clusterDef, nil)
				Expect(clusterDef.Spec.Pod.LivenessProbe.FailureThreshold).To(Equal(ptr.Int32(3)))
				Expect(clusterDef.Spec.Pod.LivenessProbe.InitialDelaySeconds).To(Equal(ptr.Int32(30)))
				Expect(clusterDef.Spec.Pod.LivenessProbe.PeriodSeconds).To(Equal(ptr.Int32(30)))
				Expect(clusterDef.Spec.Pod.LivenessProbe.SuccessThreshold).To(Equal(ptr.Int32(1)))
				Expect(clusterDef.Spec.Pod.LivenessProbe.TimeoutSeconds).To(Equal(ptr.Int32(5)))
			})

			It("should use the specified liveness probe values if they are given", func() {
				clusterDef.Spec.Pod.LivenessProbe = &v1alpha1.Probe{
					SuccessThreshold:    ptr.Int32(1),
					PeriodSeconds:       ptr.Int32(2),
					InitialDelaySeconds: ptr.Int32(3),
					FailureThreshold:    ptr.Int32(4),
					TimeoutSeconds:      ptr.Int32(5),
				}
				SetDefaultsForCassandra(clusterDef, nil)
				Expect(clusterDef.Spec.Pod.LivenessProbe.SuccessThreshold).To(Equal(ptr.Int32(1)))
				Expect(clusterDef.Spec.Pod.LivenessProbe.PeriodSeconds).To(Equal(ptr.Int32(2)))
				Expect(clusterDef.Spec.Pod.LivenessProbe.InitialDelaySeconds).To(Equal(ptr.Int32(3)))
				Expect(clusterDef.Spec.Pod.LivenessProbe.FailureThreshold).To(Equal(ptr.Int32(4)))
				Expect(clusterDef.Spec.Pod.LivenessProbe.TimeoutSeconds).To(Equal(ptr.Int32(5)))
			})
		})

		Describe("Defaulting readiness probe", func() {

			It("should set the default readiness probe values if it is not configured for the cluster", func() {
				clusterDef.Spec.Pod.ReadinessProbe = nil
				SetDefaultsForCassandra(clusterDef, nil)
				Expect(clusterDef.Spec.Pod.ReadinessProbe.FailureThreshold).To(Equal(ptr.Int32(3)))
				Expect(clusterDef.Spec.Pod.ReadinessProbe.InitialDelaySeconds).To(Equal(ptr.Int32(30)))
				Expect(clusterDef.Spec.Pod.ReadinessProbe.PeriodSeconds).To(Equal(ptr.Int32(15)))
				Expect(clusterDef.Spec.Pod.ReadinessProbe.SuccessThreshold).To(Equal(ptr.Int32(1)))
				Expect(clusterDef.Spec.Pod.ReadinessProbe.TimeoutSeconds).To(Equal(ptr.Int32(5)))
			})

			It("should set the default readiness probe values if the readiness probe is present but has unspecified values", func() {
				clusterDef.Spec.Pod.ReadinessProbe = &v1alpha1.Probe{}
				SetDefaultsForCassandra(clusterDef, nil)
				Expect(clusterDef.Spec.Pod.ReadinessProbe.FailureThreshold).To(Equal(ptr.Int32(3)))
				Expect(clusterDef.Spec.Pod.ReadinessProbe.InitialDelaySeconds).To(Equal(ptr.Int32(30)))
				Expect(clusterDef.Spec.Pod.ReadinessProbe.PeriodSeconds).To(Equal(ptr.Int32(15)))
				Expect(clusterDef.Spec.Pod.ReadinessProbe.SuccessThreshold).To(Equal(ptr.Int32(1)))
				Expect(clusterDef.Spec.Pod.ReadinessProbe.TimeoutSeconds).To(Equal(ptr.Int32(5)))
			})

			It("should use the specified readiness probe values if they are given", func() {
				clusterDef.Spec.Pod.ReadinessProbe = &v1alpha1.Probe{
					SuccessThreshold:    ptr.Int32(1),
					PeriodSeconds:       ptr.Int32(2),
					InitialDelaySeconds: ptr.Int32(3),
					FailureThreshold:    ptr.Int32(4),
					TimeoutSeconds:      ptr.Int32(5),
				}
				SetDefaultsForCassandra(clusterDef, nil)
				Expect(clusterDef.Spec.Pod.ReadinessProbe.SuccessThreshold).To(Equal(ptr.Int32(1)))
				Expect(clusterDef.Spec.Pod.ReadinessProbe.PeriodSeconds).To(Equal(ptr.Int32(2)))
				Expect(clusterDef.Spec.Pod.ReadinessProbe.InitialDelaySeconds).To(Equal(ptr.Int32(3)))
				Expect(clusterDef.Spec.Pod.ReadinessProbe.FailureThreshold).To(Equal(ptr.Int32(4)))
				Expect(clusterDef.Spec.Pod.ReadinessProbe.TimeoutSeconds).To(Equal(ptr.Int32(5)))
			})
		})

		Describe("Defaulting storage", func() {
			It("should set access mode to read/write once when not set", func() {
				clusterDef.Spec.Racks[0].Storage[0].PersistentVolumeClaim.AccessModes = nil

				SetDefaultsForCassandra(clusterDef, nil)
				Expect(clusterDef.Spec.Racks[0].Storage[0].PersistentVolumeClaim.AccessModes).To(ConsistOf(coreV1.ReadWriteOnce))
			})

			It("should preserve access mode when already set", func() {
				clusterDef.Spec.Racks[0].Storage[0].PersistentVolumeClaim.AccessModes = []coreV1.PersistentVolumeAccessMode{coreV1.ReadWriteMany}

				SetDefaultsForCassandra(clusterDef, nil)
				Expect(clusterDef.Spec.Racks[0].Storage[0].PersistentVolumeClaim.AccessModes).To(ConsistOf(coreV1.ReadWriteMany))
			})

			It("should preserve path when set", func() {
				clusterDef.Spec.Racks[0].Storage[0].Path = ptr.String("/custom-cassandra-home")

				SetDefaultsForCassandra(clusterDef, nil)
				Expect(*clusterDef.Spec.Racks[0].Storage[0].Path).To(Equal("/custom-cassandra-home"))
			})

			It("should set path to default cassandra installation when not set", func() {
				clusterDef.Spec.Racks[0].Storage[0].Path = nil

				SetDefaultsForCassandra(clusterDef, nil)
				Expect(*clusterDef.Spec.Racks[0].Storage[0].Path).To(Equal("/var/lib/cassandra"))
			})
		})

	})

	Describe("Snapshot Properties", func() {
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
			snapshot2.RetentionPolicy = &v1alpha1.RetentionPolicy{CleanupSchedule: "01 11 * * *"}
			Expect(SnapshotPropertiesUpdated(snapshot1, snapshot2)).To(BeFalse())
			Expect(SnapshotPropertiesUpdated(snapshot2, snapshot1)).To(BeFalse())
		})

		It("should be found different when schedule is different", func() {
			snapshot1.Schedule = "1 10 * * *"
			snapshot2.Schedule = "2 10 * * *"
			Expect(SnapshotPropertiesUpdated(snapshot1, snapshot2)).To(BeTrue())
			Expect(SnapshotPropertiesUpdated(snapshot2, snapshot1)).To(BeTrue())
		})

		It("should be found different when one has no timeout", func() {
			snapshot1.TimeoutSeconds = nil
			snapshot2.TimeoutSeconds = ptr.Int32(1)
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
			snapshot2.Keyspaces = []string{"keyspace1", "keyspace2"}
			Expect(SnapshotPropertiesUpdated(snapshot1, snapshot2)).To(BeTrue())
			Expect(SnapshotPropertiesUpdated(snapshot2, snapshot1)).To(BeTrue())
		})

		It("should be found different when a snapshot has no keyspace", func() {
			snapshot1.Keyspaces = nil
			snapshot2.Keyspaces = []string{"keyspace1", "keyspace2"}
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

	Describe("Snapshot Cleanup Properties", func() {
		var (
			snapshot1 *v1alpha1.Snapshot
			snapshot2 *v1alpha1.Snapshot
		)

		BeforeEach(func() {
			snapshot1 = apis.ASnapshot().
				WithDefaults().
				WithRetentionPolicy(apis.ARetentionPolicy().WithDefaults()).
				Build()
			snapshot2 = snapshot1.DeepCopy()
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

		It("should be found different when the cleanup schedule is different", func() {
			snapshot1.RetentionPolicy.CleanupSchedule = "01 10 * * *"
			snapshot2.RetentionPolicy.CleanupSchedule = "01 11 * * *"
			Expect(SnapshotCleanupPropertiesUpdated(snapshot1, snapshot2)).To(BeTrue())
			Expect(SnapshotCleanupPropertiesUpdated(snapshot2, snapshot1)).To(BeTrue())
		})

		It("should be found different when one has no retention period", func() {
			snapshot1.RetentionPolicy.RetentionPeriodDays = nil
			snapshot2.RetentionPolicy.RetentionPeriodDays = ptr.Int32(10)
			Expect(SnapshotCleanupPropertiesUpdated(snapshot1, snapshot2)).To(BeTrue())
			Expect(SnapshotCleanupPropertiesUpdated(snapshot2, snapshot1)).To(BeTrue())
		})

		It("should be found different when retention period have different values", func() {
			snapshot1.RetentionPolicy.RetentionPeriodDays = ptr.Int32(1)
			snapshot2.RetentionPolicy.RetentionPeriodDays = ptr.Int32(2)
			Expect(SnapshotCleanupPropertiesUpdated(snapshot1, snapshot2)).To(BeTrue())
			Expect(SnapshotCleanupPropertiesUpdated(snapshot2, snapshot1)).To(BeTrue())
		})

		It("should be found different when one has no cleanup timeout", func() {
			snapshot1.RetentionPolicy.CleanupTimeoutSeconds = nil
			snapshot2.RetentionPolicy.CleanupTimeoutSeconds = ptr.Int32(21)
			Expect(SnapshotCleanupPropertiesUpdated(snapshot1, snapshot2)).To(BeTrue())
			Expect(SnapshotCleanupPropertiesUpdated(snapshot2, snapshot1)).To(BeTrue())
		})

		It("should be found different when cleanup timeout have different values", func() {
			snapshot1.RetentionPolicy.CleanupTimeoutSeconds = ptr.Int32(30)
			snapshot2.RetentionPolicy.CleanupTimeoutSeconds = ptr.Int32(31)
			Expect(SnapshotCleanupPropertiesUpdated(snapshot1, snapshot2)).To(BeTrue())
			Expect(SnapshotCleanupPropertiesUpdated(snapshot2, snapshot1)).To(BeTrue())
		})
	})
})
