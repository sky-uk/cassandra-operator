package modification

import (
	"fmt"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/ptr"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sky-uk/cassandra-operator/cassandra-operator/test/e2e"
)

var _ = Context("External cluster modifications", func() {
	var (
		clusterName   string
		testStartTime time.Time
	)

	BeforeEach(func() {
		testStartTime = time.Now()
		clusterName = AClusterName()
	})

	JustAfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			PrintDiagnosis(Namespace, testStartTime, clusterName)
		}
	})

	AfterEach(func() {
		DeleteCassandraResourcesForClusters(Namespace, clusterName)
		resources.ReleaseResource(resourcesToReclaim)
	})

	Context("resources are deleted unintentionally", func() {
		It("should eventually bring the cluster to the desired state", func() {
			// given
			registerResourcesUsed(2)
			racks := []v1alpha1.Rack{Rack("a", 1), Rack("b", 1)}
			AClusterWithName(clusterName).
				AndRacks(racks).
				AndScheduledSnapshot(&v1alpha1.Snapshot{
					Image:    CassandraSnapshotImageName,
					Schedule: "59 23 * * *",
					Resources: coreV1.ResourceRequirements{
						Limits: coreV1.ResourceList{
							coreV1.ResourceMemory: resource.MustParse("150Mi"),
						},
						Requests: coreV1.ResourceList{
							coreV1.ResourceMemory: resource.MustParse("150Mi"),
							coreV1.ResourceCPU:    resource.MustParse("225m"),
						},
					},
					RetentionPolicy: &v1alpha1.RetentionPolicy{
						CleanupSchedule: "11 22 1 * *",
						Resources: coreV1.ResourceRequirements{
							Limits: coreV1.ResourceList{
								coreV1.ResourceMemory: resource.MustParse("250Mi"),
							},
							Requests: coreV1.ResourceList{
								coreV1.ResourceMemory: resource.MustParse("250Mi"),
								coreV1.ResourceCPU:    resource.MustParse("125m"),
							},
						},
					},
				}).
				Exists()

			// when
			deletionTimestamp := time.Now().Truncate(time.Second)
			TheServiceIsDeletedFor(Namespace, clusterName)
			TheStatefulSetIsDeletedForRack(Namespace, clusterName, "a")
			TheSnapshotCronJobIsDeleted(Namespace, clusterName)
			TheSnapshotCleanupCronJobIsDeleted(Namespace, clusterName)

			// then
			By("creating the deleted headless service for the cluster")
			Eventually(HeadlessServiceForCluster(Namespace, clusterName), NodeStartDuration, CheckInterval).Should(And(
				Not(BeNil()),
				HaveLabel("app.kubernetes.io/name", clusterName),
				HaveLabel("app.kubernetes.io/instance", fmt.Sprintf("%s.%s", Namespace, clusterName)),
				HaveLabel("app.kubernetes.io/managed-by", "cassandra-operator"),
			))

			By("re-creating the deleted rack")
			Eventually(RacksForCluster(Namespace, clusterName), NodeStartDuration, CheckInterval).Should(And(
				HaveLen(2),
				HaveKeyWithValue("a", []string{PodName(clusterName, "a", 0)}),
				HaveKeyWithValue("b", []string{PodName(clusterName, "b", 0)}),
			))
			Eventually(PodReadyForCluster(Namespace, clusterName), NodeStartDuration, CheckInterval).
				Should(Equal(2), fmt.Sprintf("For cluster %s", clusterName))
			Eventually(PodCreationTime(Namespace, PodName(clusterName, "a", 0)), NodeStartDuration, CheckInterval).
				Should(BeTemporally(">=", deletionTimestamp))

			By("re-creating the delete snapshot job")
			By("re-creating the delete snapshot cleanup job")
			Eventually(CronJobsForCluster(Namespace, clusterName), time.Minute, CheckInterval).Should(And(
				HaveLen(2),
				Each(BeCreatedOnOrAfter(deletionTimestamp)),
			))
		})
	})

	Context("resources are modified unintentionally", func() {
		It("should eventually bring the snapshot jobs to the desired state", func() {
			// given
			registerResourcesUsed(1)
			racks := []v1alpha1.Rack{Rack("a", 1)}
			aCluster := AClusterWithName(clusterName).
				AndRacks(racks).
				AndScheduledSnapshot(&v1alpha1.Snapshot{
					Image:          CassandraSnapshotImageName,
					TimeoutSeconds: ptr.Int32(11),
					Schedule:       "59 23 * * *",
					Resources: coreV1.ResourceRequirements{
						Limits: coreV1.ResourceList{
							coreV1.ResourceMemory: resource.MustParse("57Mi"),
						},
						Requests: coreV1.ResourceList{
							coreV1.ResourceMemory: resource.MustParse("57Mi"),
							coreV1.ResourceCPU:    resource.MustParse("227m"),
						},
					},
					RetentionPolicy: &v1alpha1.RetentionPolicy{
						CleanupSchedule: "11 22 1 * *",
						Resources: coreV1.ResourceRequirements{
							Limits: coreV1.ResourceList{
								coreV1.ResourceMemory: resource.MustParse("58Mi"),
							},
							Requests: coreV1.ResourceList{
								coreV1.ResourceMemory: resource.MustParse("58Mi"),
								coreV1.ResourceCPU:    resource.MustParse("228m"),
							},
						},
					},
				}).Exists()
			aCluster.SetDefaults()

			By("verifying the snapshot job has been created")
			By("verifying the snapshot cleanup has been created")
			Eventually(CronJobsForCluster(Namespace, clusterName), 30*time.Second, CheckInterval).Should(And(
				HaveLen(2),
				ContainElement(HaveJobSpec(&JobExpectation{
					Schedule:       "59 23 * * *",
					ContainerImage: "cassandra-snapshot",
					ContainerCommand: []string{
						"/cassandra-snapshot", "create",
						"-n", Namespace,
						"-l", aCluster.CassandraPodSelector(),
						"-t", "11s",
					}}),
				),
				ContainElement(HaveJobSpec(&JobExpectation{
					Schedule:       "11 22 1 * *",
					ContainerImage: "cassandra-snapshot:",
					ContainerCommand: []string{
						"/cassandra-snapshot", "cleanup",
						"-n", Namespace,
						"-l", aCluster.CassandraPodSelector(),
						"-r", "168h0m0s",
						"-t", "10s",
					}}),
				),
				ContainElement(HaveResourcesRequirements(&ResourceRequirementsAssertion{
					ContainerName: clusterName + "-snapshot",
					MemoryRequest: ptr.String("57Mi"),
					MemoryLimit:   ptr.String("57Mi"),
					CPURequest:    ptr.String("227m"),
					CPULimit:      nil,
				}),
				),
				ContainElement(HaveResourcesRequirements(&ResourceRequirementsAssertion{
					ContainerName: clusterName + "-snapshot-cleanup",
					MemoryRequest: ptr.String("58Mi"),
					MemoryLimit:   ptr.String("58Mi"),
					CPURequest:    ptr.String("228m"),
					CPULimit:      nil,
				}),
				),
			))

			// when
			By("Modifying the snapshot cronjob definition")
			editedSnapshotResources := coreV1.ResourceRequirements{
				Limits: coreV1.ResourceList{
					coreV1.ResourceMemory: resource.MustParse("87Mi"),
				},
				Requests: coreV1.ResourceList{
					coreV1.ResourceMemory: resource.MustParse("87Mi"),
					coreV1.ResourceCPU:    resource.MustParse("287m"),
				},
			}
			jobName := fmt.Sprintf("%s-snapshot", clusterName)
			TheCronjobResourcesAreChangedTo(Namespace, jobName, editedSnapshotResources)

			By("Modifying the snapshot-cleanup cronjob definition")
			editedSnapshotCleanupResources := coreV1.ResourceRequirements{
				Limits: coreV1.ResourceList{
					coreV1.ResourceMemory: resource.MustParse("87Mi"),
				},
				Requests: coreV1.ResourceList{
					coreV1.ResourceMemory: resource.MustParse("87Mi"),
					coreV1.ResourceCPU:    resource.MustParse("287m"),
				},
			}
			cleanupJobName := fmt.Sprintf("%s-snapshot-cleanup", clusterName)
			TheCronjobResourcesAreChangedTo(Namespace, cleanupJobName, editedSnapshotCleanupResources)

			// then
			By("the operator should restore the snapshot cronjob spec to that of the cassandra spec")
			By("the operator should restore the snapshot-cleanup cronjob spec to that of the cassandra spec")
			Eventually(CronJobsForCluster(Namespace, clusterName), 30*time.Second, CheckInterval).Should(And(
				HaveLen(2),
				ContainElement(HaveJobSpec(&JobExpectation{
					Schedule:       "59 23 * * *",
					ContainerImage: "cassandra-snapshot",
					ContainerCommand: []string{
						"/cassandra-snapshot", "create",
						"-n", Namespace,
						"-l", aCluster.CassandraPodSelector(),
						"-t", "11s",
					}}),
				),
				ContainElement(HaveJobSpec(&JobExpectation{
					Schedule:       "11 22 1 * *",
					ContainerImage: "cassandra-snapshot:",
					ContainerCommand: []string{
						"/cassandra-snapshot", "cleanup",
						"-n", Namespace,
						"-l", aCluster.CassandraPodSelector(),
						"-r", "168h0m0s",
						"-t", "10s",
					}}),
				),
				ContainElement(HaveResourcesRequirements(&ResourceRequirementsAssertion{
					ContainerName: clusterName + "-snapshot",
					MemoryRequest: ptr.String("57Mi"),
					MemoryLimit:   ptr.String("57Mi"),
					CPURequest:    ptr.String("227m"),
					CPULimit:      nil,
				}),
				),
				ContainElement(HaveResourcesRequirements(&ResourceRequirementsAssertion{
					ContainerName: clusterName + "-snapshot-cleanup",
					MemoryRequest: ptr.String("58Mi"),
					MemoryLimit:   ptr.String("58Mi"),
					CPURequest:    ptr.String("228m"),
					CPULimit:      nil,
				}),
				),
			))
		})
	})

})
