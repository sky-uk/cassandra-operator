package helpers

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/ptr"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test"
)

func TestHelpers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Helpers Suite", test.CreateParallelReporters("helpers"))
}

var _ = Describe("Cassandra Helpers", func() {

	Context("Cassandra.Spec", func() {
		var clusterDef *v1alpha1.Cassandra
		BeforeEach(func() {
			clusterDef = &v1alpha1.Cassandra{
				ObjectMeta: metaV1.ObjectMeta{Name: "mycluster", Namespace: "mynamespace"},
				Spec: v1alpha1.CassandraSpec{
					Racks: []v1alpha1.Rack{{Name: "a", Replicas: 1, StorageClass: "some-storage", Zone: "some-zone"}, {Name: "b", Replicas: 1, StorageClass: "some-storage", Zone: "some-zone"}},
					Pod: v1alpha1.Pod{
						Resources: coreV1.ResourceRequirements{
							Requests: coreV1.ResourceList{
								coreV1.ResourceMemory: resource.MustParse("1Gi"),
								coreV1.ResourceCPU:    resource.MustParse("100m"),
							},
							Limits: coreV1.ResourceList{
								coreV1.ResourceMemory: resource.MustParse("1Gi"),
								coreV1.ResourceCPU:    resource.MustParse("100m"),
							},
						},
						StorageSize: resource.MustParse("1Gi"),
					},
				},
			}
		})

		Context("Snapshot", func() {
			Context("TimeoutSeconds", func() {
				It("should default to 10", func() {
					clusterDef.Spec.Snapshot = &v1alpha1.Snapshot{}
					SetDefaultsForCassandra(clusterDef)
					Expect(*clusterDef.Spec.Snapshot.TimeoutSeconds).To(Equal(int32(v1alpha1.DefaultSnapshotTimeoutSeconds)))
				})
				It("should not overwrite existing value", func() {
					clusterDef.Spec.Snapshot = &v1alpha1.Snapshot{
						TimeoutSeconds: ptr.Int32(456),
					}
					SetDefaultsForCassandra(clusterDef)
					Expect(*clusterDef.Spec.Snapshot.TimeoutSeconds).To(Equal(int32(456)))
				})
			})

			It("should not change an undefined Snapshot", func() {
				clusterDef.Spec.Snapshot = nil
				SetDefaultsForCassandra(clusterDef)
				Expect(clusterDef.Spec.Snapshot).To(BeNil())
			})

			Context("RetentionPolicy", func() {
				Context("Enabled", func() {
					It("should default to true", func() {
						clusterDef.Spec.Snapshot = &v1alpha1.Snapshot{
							RetentionPolicy: &v1alpha1.RetentionPolicy{
								Enabled: nil,
							},
						}
						SetDefaultsForCassandra(clusterDef)
						Expect(*clusterDef.Spec.Snapshot.RetentionPolicy.Enabled).To(BeTrue())
					})
					It("should not overwrite existing value", func() {
						clusterDef.Spec.Snapshot = &v1alpha1.Snapshot{
							RetentionPolicy: &v1alpha1.RetentionPolicy{
								Enabled: ptr.Bool(false),
							},
						}
						SetDefaultsForCassandra(clusterDef)
						Expect(*clusterDef.Spec.Snapshot.RetentionPolicy.Enabled).To(BeFalse())
					})
				})
				Context("RetentionPeriodDays", func() {
					It("should default to 7", func() {
						clusterDef.Spec.Snapshot = &v1alpha1.Snapshot{
							RetentionPolicy: &v1alpha1.RetentionPolicy{
								RetentionPeriodDays: nil,
							},
						}
						SetDefaultsForCassandra(clusterDef)
						Expect(*clusterDef.Spec.Snapshot.RetentionPolicy.RetentionPeriodDays).To(Equal(int32(v1alpha1.DefaultRetentionPolicyRetentionPeriodDays)))
					})
					It("should not overwrite existing value", func() {
						clusterDef.Spec.Snapshot = &v1alpha1.Snapshot{
							RetentionPolicy: &v1alpha1.RetentionPolicy{
								RetentionPeriodDays: ptr.Int32(543),
							},
						}
						SetDefaultsForCassandra(clusterDef)
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
						SetDefaultsForCassandra(clusterDef)
						Expect(*clusterDef.Spec.Snapshot.RetentionPolicy.CleanupTimeoutSeconds).To(Equal(int32(v1alpha1.DefaultRetentionPolicyCleanupTimeoutSeconds)))
					})
					It("should not overwrite existing value", func() {
						clusterDef.Spec.Snapshot = &v1alpha1.Snapshot{
							RetentionPolicy: &v1alpha1.RetentionPolicy{
								CleanupTimeoutSeconds: ptr.Int32(321),
							},
						}
						SetDefaultsForCassandra(clusterDef)
						Expect(*clusterDef.Spec.Snapshot.RetentionPolicy.CleanupTimeoutSeconds).To(Equal(int32(321)))
					})
				})
				It("should not change an undefined Snapshot.RetentionPolicy", func() {
					clusterDef.Spec.Snapshot = &v1alpha1.Snapshot{}
					clusterDef.Spec.Snapshot.RetentionPolicy = nil
					SetDefaultsForCassandra(clusterDef)
					Expect(clusterDef.Spec.Snapshot.RetentionPolicy).To(BeNil())
				})
			})
		})

		Context("Defaulting datacenter", func() {
			It("should default Datacenter to dc1", func() {
				clusterDef.Spec.Datacenter = nil
				SetDefaultsForCassandra(clusterDef)
				Expect(*clusterDef.Spec.Datacenter).To(Equal("dc1"))
			})
			It("should not overwrite Datacenter ", func() {
				clusterDef.Spec.Datacenter = ptr.String("carefully-chosen-datacenter-name")
				SetDefaultsForCassandra(clusterDef)
				Expect(*clusterDef.Spec.Datacenter).To(Equal("carefully-chosen-datacenter-name"))
			})
		})

		Context("Defaulting useEmptyDir", func() {
			It("should default UseEmptyDir to false", func() {
				clusterDef.Spec.UseEmptyDir = nil
				SetDefaultsForCassandra(clusterDef)
				Expect(*clusterDef.Spec.UseEmptyDir).To(BeFalse())
			})
			It("should not overwrite UseEmptyDir ", func() {
				clusterDef.Spec.UseEmptyDir = ptr.Bool(true)
				SetDefaultsForCassandra(clusterDef)
				Expect(*clusterDef.Spec.UseEmptyDir).To(BeTrue())
			})
		})

		Context("Defaulting images", func() {
			It("should use the 3.11 version of the apache cassandra image if one is not supplied for the cluster", func() {
				clusterDef.Spec.Pod.Image = nil
				SetDefaultsForCassandra(clusterDef)
				Expect(*clusterDef.Spec.Pod.Image).To(Equal("cassandra:3.11"))
			})

			It("should use the specified version of the cassandra image if one is given", func() {
				clusterDef.Spec.Pod.Image = ptr.String("somerepo/someimage:v1.0")
				SetDefaultsForCassandra(clusterDef)
				Expect(*clusterDef.Spec.Pod.Image).To(Equal("somerepo/someimage:v1.0"))
			})

			It("should use the latest version of the cassandra bootstrapper image if one is not supplied for the cluster", func() {
				clusterDef.Spec.Pod.Image = nil
				SetDefaultsForCassandra(clusterDef)
				Expect(*clusterDef.Spec.Pod.BootstrapperImage).To(Equal("skyuk/cassandra-bootstrapper:latest"))
			})

			It("should use the specified version of the cassandra bootstrapper image if one is given", func() {
				clusterDef.Spec.Pod.BootstrapperImage = ptr.String("somerepo/some-bootstrapper-image:v1.0")
				SetDefaultsForCassandra(clusterDef)
				Expect(*clusterDef.Spec.Pod.BootstrapperImage).To(Equal("somerepo/some-bootstrapper-image:v1.0"))
			})

			It("should use the latest version of the cassandra snapshot image if one is not supplied for the cluster", func() {
				clusterDef.Spec.Snapshot = &v1alpha1.Snapshot{
					Schedule: "1 23 * * *",
				}
				clusterDef.Spec.Snapshot.Image = nil
				SetDefaultsForCassandra(clusterDef)
				Expect(*clusterDef.Spec.Snapshot.Image).To(Equal("skyuk/cassandra-snapshot:latest"))
			})

			It("should use the specified version of the cassandra snapshot image if one is given", func() {
				clusterDef.Spec.Snapshot = &v1alpha1.Snapshot{
					Schedule: "1 23 * * *",
				}
				clusterDef.Spec.Snapshot.Image = ptr.String("somerepo/some-snapshot-image:v1.0")
				SetDefaultsForCassandra(clusterDef)
				Expect(*clusterDef.Spec.Snapshot.Image).To(Equal("somerepo/some-snapshot-image:v1.0"))
			})

			It("should use the latest version of the cassandra sidecar image if one is not supplied for the cluster", func() {
				clusterDef.Spec.Pod.SidecarImage = nil
				SetDefaultsForCassandra(clusterDef)
				Expect(*clusterDef.Spec.Pod.SidecarImage).To(Equal("skyuk/cassandra-sidecar:latest"))
			})

			It("should use the specified version of the cassandra snapshot image if one is given", func() {
				clusterDef.Spec.Pod.SidecarImage = ptr.String("somerepo/some-sidecar-image:v1.0")
				SetDefaultsForCassandra(clusterDef)
				Expect(*clusterDef.Spec.Pod.SidecarImage).To(Equal("somerepo/some-sidecar-image:v1.0"))
			})
		})

		Context("Defaulting liveness probe", func() {
			It("should set the default liveness probe values if it is not configured for the cluster", func() {
				clusterDef.Spec.Pod.LivenessProbe = nil
				SetDefaultsForCassandra(clusterDef)
				Expect(clusterDef.Spec.Pod.LivenessProbe.FailureThreshold).To(Equal(ptr.Int32(3)))
				Expect(clusterDef.Spec.Pod.LivenessProbe.InitialDelaySeconds).To(Equal(ptr.Int32(30)))
				Expect(clusterDef.Spec.Pod.LivenessProbe.PeriodSeconds).To(Equal(ptr.Int32(30)))
				Expect(clusterDef.Spec.Pod.LivenessProbe.SuccessThreshold).To(Equal(ptr.Int32(1)))
				Expect(clusterDef.Spec.Pod.LivenessProbe.TimeoutSeconds).To(Equal(ptr.Int32(5)))
			})

			It("should set the default liveness probe values if the liveness probe is present but has unspecified values", func() {
				clusterDef.Spec.Pod.LivenessProbe = &v1alpha1.Probe{}
				SetDefaultsForCassandra(clusterDef)
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
				SetDefaultsForCassandra(clusterDef)
				Expect(clusterDef.Spec.Pod.LivenessProbe.SuccessThreshold).To(Equal(ptr.Int32(1)))
				Expect(clusterDef.Spec.Pod.LivenessProbe.PeriodSeconds).To(Equal(ptr.Int32(2)))
				Expect(clusterDef.Spec.Pod.LivenessProbe.InitialDelaySeconds).To(Equal(ptr.Int32(3)))
				Expect(clusterDef.Spec.Pod.LivenessProbe.FailureThreshold).To(Equal(ptr.Int32(4)))
				Expect(clusterDef.Spec.Pod.LivenessProbe.TimeoutSeconds).To(Equal(ptr.Int32(5)))
			})
		})

		Context("Defaulting readiness probe", func() {

			It("should set the default readiness probe values if it is not configured for the cluster", func() {
				clusterDef.Spec.Pod.ReadinessProbe = nil
				SetDefaultsForCassandra(clusterDef)
				Expect(clusterDef.Spec.Pod.ReadinessProbe.FailureThreshold).To(Equal(ptr.Int32(3)))
				Expect(clusterDef.Spec.Pod.ReadinessProbe.InitialDelaySeconds).To(Equal(ptr.Int32(30)))
				Expect(clusterDef.Spec.Pod.ReadinessProbe.PeriodSeconds).To(Equal(ptr.Int32(15)))
				Expect(clusterDef.Spec.Pod.ReadinessProbe.SuccessThreshold).To(Equal(ptr.Int32(1)))
				Expect(clusterDef.Spec.Pod.ReadinessProbe.TimeoutSeconds).To(Equal(ptr.Int32(5)))
			})

			It("should set the default readiness probe values if the readiness probe is present but has unspecified values", func() {
				clusterDef.Spec.Pod.ReadinessProbe = &v1alpha1.Probe{}
				SetDefaultsForCassandra(clusterDef)
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
				SetDefaultsForCassandra(clusterDef)
				Expect(clusterDef.Spec.Pod.ReadinessProbe.SuccessThreshold).To(Equal(ptr.Int32(1)))
				Expect(clusterDef.Spec.Pod.ReadinessProbe.PeriodSeconds).To(Equal(ptr.Int32(2)))
				Expect(clusterDef.Spec.Pod.ReadinessProbe.InitialDelaySeconds).To(Equal(ptr.Int32(3)))
				Expect(clusterDef.Spec.Pod.ReadinessProbe.FailureThreshold).To(Equal(ptr.Int32(4)))
				Expect(clusterDef.Spec.Pod.ReadinessProbe.TimeoutSeconds).To(Equal(ptr.Int32(5)))
			})
		})

	})

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

		It("should be found disabled when RetentionPolicy.Enabled is nil", func() {
			snapshot.RetentionPolicy = &v1alpha1.RetentionPolicy{
				Enabled: nil,
			}
			Expect(HasRetentionPolicyEnabled(snapshot)).To(BeFalse())
		})

		It("should be found disabled when retention policy is not enabled", func() {
			snapshot.RetentionPolicy = &v1alpha1.RetentionPolicy{
				Enabled:         ptr.Bool(false),
				CleanupSchedule: "11 11 * * *",
			}
			Expect(HasRetentionPolicyEnabled(snapshot)).To(BeFalse())
		})

		It("should be found enabled when retention policy is enabled", func() {
			snapshot.RetentionPolicy = &v1alpha1.RetentionPolicy{
				Enabled:         ptr.Bool(true),
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
					Enabled:               ptr.Bool(true),
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
					Enabled:               ptr.Bool(true),
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
			snapshot1.RetentionPolicy.Enabled = ptr.Bool(false)
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
