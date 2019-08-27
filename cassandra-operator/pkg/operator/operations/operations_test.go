package operations

import (
	"fmt"
	"github.com/onsi/gomega/types"
	"github.com/stretchr/testify/mock"
	"k8s.io/api/apps/v1beta2"
	"k8s.io/api/batch/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"log"
	"reflect"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	v1alpha1helpers "github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1/helpers"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/dispatcher"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/metrics"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/ptr"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestOperations(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Operations Suite", test.CreateParallelReporters("operations"))
}

var _ = Describe("operations to execute based on event", func() {
	var (
		receiver      *Receiver
		fakes         *mockAccessor
		oldClusterDef *v1alpha1.Cassandra
		newClusterDef *v1alpha1.Cassandra
		timeout       = int32(1)
		sevenDays     = int32(7)
	)

	BeforeEach(func() {
		oldClusterDef = &v1alpha1.Cassandra{
			ObjectMeta: metav1.ObjectMeta{Name: "mycluster", Namespace: "mynamespace"},
			Spec: v1alpha1.CassandraSpec{
				Racks: []v1alpha1.Rack{{Name: "a", Replicas: 1, StorageClass: "some-storage", Zone: "some-zone"}, {Name: "b", Replicas: 1, StorageClass: "some-storage", Zone: "some-zone"}},
				Pod: v1alpha1.Pod{
					Memory:      resource.MustParse("1Gi"),
					CPU:         resource.MustParse("100m"),
					StorageSize: resource.MustParse("1Gi"),
				},
				Snapshot: &v1alpha1.Snapshot{
					Schedule:       "2 * * * *",
					TimeoutSeconds: &timeout,
					RetentionPolicy: &v1alpha1.RetentionPolicy{
						Enabled:               ptr.Bool(true),
						CleanupSchedule:       "1 * * * *",
						CleanupTimeoutSeconds: &timeout,
						RetentionPeriodDays:   &sevenDays,
					},
				},
			},
		}
		v1alpha1helpers.SetDefaultsForCassandra(oldClusterDef)
		newClusterDef = oldClusterDef.DeepCopy()

		fakes = &mockAccessor{}
		receiver = NewEventReceiver(fakes, &metrics.PrometheusMetrics{}, nil)
	})

	Context("when a AddCluster event is received", func() {
		It("should return an add cluster operation", func() {
			// given
			newClusterDef.Spec.Snapshot = nil

			// when
			operations := receiver.operationsToExecute(&dispatcher.Event{Kind: AddCluster, Data: newClusterDef})

			//then
			Expect(operations).To(HaveOperationsOfType([]Operation{
				&AddClusterOperation{},
			}))
		})

		Context("a snapshot spec exists", func() {

			It("should return add cluster and add snapshot operations", func() {
				// given
				newClusterDef.Spec.Snapshot.RetentionPolicy = nil

				// when
				operations := receiver.operationsToExecute(&dispatcher.Event{Kind: AddCluster, Data: newClusterDef})

				//then
				Expect(operations).To(HaveOperationsOfType([]Operation{
					&AddClusterOperation{},
					&AddSnapshotOperation{},
				}))
			})

			It("should return add cluster and add snapshot operations when the retention policy is disabled", func() {
				// given
				newClusterDef.Spec.Snapshot.RetentionPolicy.Enabled = ptr.Bool(false)

				// when
				operations := receiver.operationsToExecute(&dispatcher.Event{Kind: AddCluster, Data: newClusterDef})

				//then
				Expect(operations).To(HaveOperationsOfType([]Operation{
					&AddClusterOperation{},
					&AddSnapshotOperation{},
				}))
			})

			It("should return add cluster, add snapshot and add cleanup snapshot operations when a snapshot retention spec exists", func() {
				// when
				operations := receiver.operationsToExecute(&dispatcher.Event{Kind: AddCluster, Data: newClusterDef})

				//then
				Expect(operations).To(HaveOperationsOfType([]Operation{
					&AddClusterOperation{},
					&AddSnapshotOperation{},
					&AddSnapshotCleanupOperation{},
				}))
			})
		})

	})

	Context("when a DeleteCluster event is received", func() {
		It("should return a delete cluster operation", func() {
			// given
			newClusterDef.Spec.Snapshot = nil

			// when
			operations := receiver.operationsToExecute(&dispatcher.Event{Kind: DeleteCluster, Data: newClusterDef})

			//then
			Expect(operations).To(HaveOperationsOfType([]Operation{
				&DeleteClusterOperation{},
			}))
		})

		Context("a snapshot spec exists", func() {
			It("should return a delete cluster and delete snapshot operations", func() {
				// given
				newClusterDef.Spec.Snapshot.RetentionPolicy = nil

				// when
				operations := receiver.operationsToExecute(&dispatcher.Event{Kind: DeleteCluster, Data: newClusterDef})

				//then
				Expect(operations).To(HaveOperationsOfType([]Operation{
					&DeleteClusterOperation{},
					&DeleteSnapshotOperation{},
				}))
			})
			It("should return a delete cluster, delete snapshot and delete snapshot cleanup operations when a retention policy is defined", func() {
				// when
				operations := receiver.operationsToExecute(&dispatcher.Event{Kind: DeleteCluster, Data: newClusterDef})

				//then
				Expect(operations).To(HaveOperationsOfType([]Operation{
					&DeleteClusterOperation{},
					&DeleteSnapshotOperation{},
					&DeleteSnapshotCleanupOperation{},
				}))
			})
			It("should return a delete cluster and delete snapshot operations when a retention policy is disabled", func() {
				// given
				newClusterDef.Spec.Snapshot.RetentionPolicy.Enabled = ptr.Bool(false)

				// when
				operations := receiver.operationsToExecute(&dispatcher.Event{Kind: DeleteCluster, Data: newClusterDef})

				//then
				Expect(operations).To(HaveOperationsOfType([]Operation{
					&DeleteClusterOperation{},
					&DeleteSnapshotOperation{},
				}))
			})

		})
	})

	Context("when a UpdateCluster event is received", func() {

		Context("the cluster does not exists or is being deleted", func() {
			It("should return no operations when cluster is not found", func() {
				// given
				fakes.cassandraDoesNotExists(newClusterDef)

				// when
				operations := receiver.operationsToExecute(&dispatcher.Event{Kind: UpdateCluster, Data: ClusterUpdate{OldCluster: oldClusterDef, NewCluster: newClusterDef}})
				Expect(operations).To(HaveLen(0))
			})

			It("should return no operations when the cluster is beind deleted", func() {
				// given
				fakes.cassandraIsBeingDeleted(newClusterDef)

				// when
				operations := receiver.operationsToExecute(&dispatcher.Event{Kind: UpdateCluster, Data: ClusterUpdate{OldCluster: oldClusterDef, NewCluster: newClusterDef}})
				Expect(operations).To(HaveLen(0))
			})

		})

		Context("the cluster exists", func() {
			BeforeEach(func() {
				fakes.cassandraExists(newClusterDef)
			})

			It("should return an update cluster operation", func() {
				// when
				operations := receiver.operationsToExecute(&dispatcher.Event{Kind: UpdateCluster, Data: ClusterUpdate{OldCluster: oldClusterDef, NewCluster: newClusterDef}})

				// then
				Expect(operations).To(HaveOperationsOfType([]Operation{
					&UpdateClusterOperation{},
				}))
			})

			Context("a snapshot or snapshot cleanup spec is removed", func() {
				It("should return update cluster, delete snapshot and delete snapshot cleanup when snapshot spec is removed", func() {
					// given
					newClusterDef.Spec.Snapshot = nil

					// when
					operations := receiver.operationsToExecute(&dispatcher.Event{Kind: UpdateCluster, Data: ClusterUpdate{OldCluster: oldClusterDef, NewCluster: newClusterDef}})

					// then
					Expect(operations).To(HaveOperationsOfType([]Operation{
						&UpdateClusterOperation{},
						&DeleteSnapshotOperation{},
						&DeleteSnapshotCleanupOperation{},
					}))
				})
				It("should return update cluster and delete snapshot cleanup when snapshot retention policy is removed", func() {
					// given
					newClusterDef.Spec.Snapshot.RetentionPolicy = nil

					// when
					operations := receiver.operationsToExecute(&dispatcher.Event{Kind: UpdateCluster, Data: ClusterUpdate{OldCluster: oldClusterDef, NewCluster: newClusterDef}})

					// then
					Expect(operations).To(HaveOperationsOfType([]Operation{
						&UpdateClusterOperation{},
						&DeleteSnapshotCleanupOperation{},
					}))
				})
				It("should return update cluster and delete snapshot cleanup when snapshot retention policy is disabled", func() {
					// given
					newClusterDef.Spec.Snapshot.RetentionPolicy.Enabled = ptr.Bool(false)

					// when
					operations := receiver.operationsToExecute(&dispatcher.Event{Kind: UpdateCluster, Data: ClusterUpdate{OldCluster: oldClusterDef, NewCluster: newClusterDef}})

					// then
					Expect(operations).To(HaveOperationsOfType([]Operation{
						&UpdateClusterOperation{},
						&DeleteSnapshotCleanupOperation{},
					}))
				})
			})

			Context("a snapshot or snapshot cleanup is added", func() {
				It("should return update cluster, add snapshot and add snapshot cleanup when snapshot with retention policy is added", func() {
					// given
					oldClusterDef.Spec.Snapshot = nil

					// when
					operations := receiver.operationsToExecute(&dispatcher.Event{Kind: UpdateCluster, Data: ClusterUpdate{OldCluster: oldClusterDef, NewCluster: newClusterDef}})

					// then
					Expect(operations).To(HaveOperationsOfType([]Operation{
						&UpdateClusterOperation{},
						&AddSnapshotOperation{},
						&AddSnapshotCleanupOperation{},
					}))
				})
				It("should return update cluster and add snapshot when snapshot without retention policy is added", func() {
					// given
					oldClusterDef.Spec.Snapshot = nil
					newClusterDef.Spec.Snapshot.RetentionPolicy = nil

					// when
					operations := receiver.operationsToExecute(&dispatcher.Event{Kind: UpdateCluster, Data: ClusterUpdate{OldCluster: oldClusterDef, NewCluster: newClusterDef}})

					// then
					Expect(operations).To(HaveOperationsOfType([]Operation{
						&UpdateClusterOperation{},
						&AddSnapshotOperation{},
					}))
				})
				It("should return update cluster and add snapshot when snapshot with retention policy disabled", func() {
					// given
					oldClusterDef.Spec.Snapshot = nil
					newClusterDef.Spec.Snapshot.RetentionPolicy.Enabled = ptr.Bool(false)

					// when
					operations := receiver.operationsToExecute(&dispatcher.Event{Kind: UpdateCluster, Data: ClusterUpdate{OldCluster: oldClusterDef, NewCluster: newClusterDef}})

					// then
					Expect(operations).To(HaveOperationsOfType([]Operation{
						&UpdateClusterOperation{},
						&AddSnapshotOperation{},
					}))
				})
			})

			Context("a snapshot is updated", func() {
				It("should return update cluster and update snapshot", func() {
					// given
					newClusterDef.Spec.Snapshot.Schedule = "1 13 4 * *"

					// when
					operations := receiver.operationsToExecute(&dispatcher.Event{Kind: UpdateCluster, Data: ClusterUpdate{OldCluster: oldClusterDef, NewCluster: newClusterDef}})

					// then
					Expect(operations).To(HaveOperationsOfType([]Operation{
						&UpdateClusterOperation{},
						&UpdateSnapshotOperation{},
					}))
				})
			})

			Context("a snapshot retention policy is updated", func() {
				It("should return update cluster and update snapshot cleanup", func() {
					// given
					newClusterDef.Spec.Snapshot.RetentionPolicy.CleanupSchedule = "1 13 4 * *"

					// when
					operations := receiver.operationsToExecute(&dispatcher.Event{Kind: UpdateCluster, Data: ClusterUpdate{OldCluster: oldClusterDef, NewCluster: newClusterDef}})

					// then
					Expect(operations).To(HaveOperationsOfType([]Operation{
						&UpdateClusterOperation{},
						&UpdateSnapshotCleanupOperation{},
					}))
				})
			})
		})
	})

	Context("when a GatherMetrics event is received", func() {
		Context("the cluster does not exists or is being deleted", func() {
			It("should return no operations when cluster is not found", func() {
				// given
				fakes.cassandraDoesNotExists(newClusterDef)

				// when
				operations := receiver.operationsToExecute(&dispatcher.Event{Kind: GatherMetrics, Data: newClusterDef})
				Expect(operations).To(HaveLen(0))
			})

			It("should return no operations when the cluster is being deleted", func() {
				// given
				fakes.cassandraIsBeingDeleted(newClusterDef)

				// when
				operations := receiver.operationsToExecute(&dispatcher.Event{Kind: GatherMetrics, Data: newClusterDef})
				Expect(operations).To(HaveLen(0))
			})

		})

		Context("the cluster exists", func() {
			BeforeEach(func() {
				fakes.cassandraExists(newClusterDef)
			})
			It("should return a gather metrics operation", func() {
				// when
				operations := receiver.operationsToExecute(&dispatcher.Event{Kind: GatherMetrics, Data: newClusterDef})

				// then
				Expect(operations).To(HaveLen(1))
				Expect(reflect.TypeOf(operations[0])).To(Equal(reflect.TypeOf(&GatherMetricsOperation{})))
			})
		})
	})

	Context("when a UpdateCustomConfig event is received", func() {
		var configMapChange ConfigMapChange

		BeforeEach(func() {
			configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "mycluster-config", Namespace: "mynamespace"}}
			configMapChange = ConfigMapChange{ConfigMap: configMap, Cassandra: newClusterDef}
		})

		Context("the cluster does not exists or is being deleted", func() {
			It("should return no operations when cluster is not found", func() {
				// given
				fakes.cassandraDoesNotExists(newClusterDef)

				// when
				operations := receiver.operationsToExecute(&dispatcher.Event{Kind: UpdateCustomConfig, Data: configMapChange})
				Expect(operations).To(HaveLen(0))
			})

			It("should return no operations when the cluster is beind deleted", func() {
				// given
				fakes.cassandraIsBeingDeleted(newClusterDef)

				// when
				operations := receiver.operationsToExecute(&dispatcher.Event{Kind: UpdateCustomConfig, Data: configMapChange})
				Expect(operations).To(HaveLen(0))
			})

		})

		Context("the cluster exists", func() {

			It("should return an update custom config operation", func() {
				// given
				fakes.cassandraExists(newClusterDef)

				// when
				operations := receiver.operationsToExecute(&dispatcher.Event{Kind: UpdateCustomConfig, Data: configMapChange})

				// then
				Expect(operations).To(HaveLen(1))
				Expect(reflect.TypeOf(operations[0])).To(Equal(reflect.TypeOf(&UpdateCustomConfigOperation{})))
			})
		})

	})

	Context("when a AddCustomConfig is received", func() {
		var configMapChange ConfigMapChange

		BeforeEach(func() {
			configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "mycluster-config", Namespace: "mynamespace"}}
			configMapChange = ConfigMapChange{ConfigMap: configMap, Cassandra: newClusterDef}
		})

		Context("the cluster does not exists or is being deleted", func() {
			It("should return no operations when cluster is not found", func() {
				// given
				fakes.cassandraDoesNotExists(newClusterDef)

				// when
				operations := receiver.operationsToExecute(&dispatcher.Event{Kind: AddCustomConfig, Data: configMapChange})
				Expect(operations).To(HaveLen(0))
			})

			It("should return no operations when the cluster is beind deleted", func() {
				// given
				fakes.cassandraIsBeingDeleted(newClusterDef)

				// when
				operations := receiver.operationsToExecute(&dispatcher.Event{Kind: AddCustomConfig, Data: configMapChange})
				Expect(operations).To(HaveLen(0))
			})

		})

		Context("the cluster exists", func() {
			It("should return an add custom config operation", func() {
				// given
				fakes.cassandraExists(newClusterDef)

				// when
				operations := receiver.operationsToExecute(&dispatcher.Event{Kind: AddCustomConfig, Data: configMapChange})

				// then
				Expect(operations).To(HaveLen(1))
				Expect(reflect.TypeOf(operations[0])).To(Equal(reflect.TypeOf(&AddCustomConfigOperation{})))
			})
		})

	})

	Context("when a DeleteCustomConfig is received", func() {

		var configMapChange ConfigMapChange

		BeforeEach(func() {
			configMapChange = ConfigMapChange{Cassandra: newClusterDef}
		})

		Context("the cluster does not exists or is being deleted", func() {
			It("should return no operations when cluster is not found", func() {
				// given
				fakes.cassandraDoesNotExists(newClusterDef)

				// when
				operations := receiver.operationsToExecute(&dispatcher.Event{Kind: DeleteCustomConfig, Data: configMapChange})
				Expect(operations).To(HaveLen(0))
			})

			It("should return no operations when the cluster is beind deleted", func() {
				// given
				fakes.cassandraIsBeingDeleted(newClusterDef)

				// when
				operations := receiver.operationsToExecute(&dispatcher.Event{Kind: DeleteCustomConfig, Data: configMapChange})
				Expect(operations).To(HaveLen(0))
			})

		})

		Context("the cluster exists", func() {
			BeforeEach(func() {
				fakes.cassandraExists(newClusterDef)
			})

			It("should return a delete custom config operation", func() {
				// when
				operations := receiver.operationsToExecute(&dispatcher.Event{Kind: DeleteCustomConfig, Data: configMapChange})

				// then
				Expect(operations).To(HaveLen(1))
				Expect(reflect.TypeOf(operations[0])).To(Equal(reflect.TypeOf(&DeleteCustomConfigOperation{})))
			})
		})
	})

	Context("when a AddService event is received", func() {
		It("should return an add service operation", func() {
			// when
			operations := receiver.operationsToExecute(&dispatcher.Event{Kind: AddService, Data: newClusterDef})

			//then
			Expect(operations).To(HaveLen(1))
			Expect(reflect.TypeOf(operations[0])).To(Equal(reflect.TypeOf(&AddServiceOperation{})))
		})
	})

})

// implements Accessor
type mockAccessor struct {
	mock.Mock
}

// implements Accessor at compilation time
var _ cluster.Accessor = &mockAccessor{}

func (m *mockAccessor) cassandraIsBeingDeleted(clusterDef *v1alpha1.Cassandra) {
	now := metav1.Now()
	cassandraBeingDeleted := &v1alpha1.Cassandra{
		ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &now},
	}
	m.On("GetCassandraForCluster", clusterDef.Namespace, clusterDef.Name).
		Return(cassandraBeingDeleted, nil)
}

func (m *mockAccessor) cassandraDoesNotExists(clusterDef *v1alpha1.Cassandra) {
	m.On("GetCassandraForCluster", clusterDef.Namespace, clusterDef.Name).
		Return(nil, errors.NewNotFound(v1alpha1.Resource("cassandra"), clusterDef.Name))
}

func (m *mockAccessor) cassandraExists(clusterDef *v1alpha1.Cassandra) {
	m.On("GetCassandraForCluster", clusterDef.Namespace, clusterDef.Name).Return(clusterDef, nil)
}

func (m *mockAccessor) GetCassandraForCluster(namespace, clusterName string) (*v1alpha1.Cassandra, error) {
	args := m.Called(namespace, clusterName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*v1alpha1.Cassandra), args.Error(1)
}
func (m *mockAccessor) CreateServiceForCluster(c *cluster.Cluster) (*corev1.Service, error) {
	log.Fatal("not implemented")
	return nil, nil
}
func (m *mockAccessor) FindCustomConfigMap(namespace, clusterName string) *corev1.ConfigMap {
	log.Fatal("not implemented")
	return nil
}
func (m *mockAccessor) UpdateCronJob(job *v1beta1.CronJob) error {
	log.Fatal("not implemented")
	return nil
}
func (m *mockAccessor) DeleteCronJob(job *v1beta1.CronJob) error {
	log.Fatal("not implemented")
	return nil
}
func (m *mockAccessor) FindCronJobForCluster(cassandra *v1alpha1.Cassandra, label string) (*v1beta1.CronJob, error) {
	log.Fatal("not implemented")
	return nil, nil
}
func (m *mockAccessor) CreateCronJobForCluster(c *cluster.Cluster, cronJob *v1beta1.CronJob) (*v1beta1.CronJob, error) {
	log.Fatal("not implemented")
	return nil, nil
}
func (m *mockAccessor) WaitUntilRackChangeApplied(cluster *cluster.Cluster, statefulSet *v1beta2.StatefulSet) error {
	log.Fatal("not implemented")
	return nil
}
func (m *mockAccessor) UpdateStatefulSet(c *cluster.Cluster, statefulSet *v1beta2.StatefulSet) (*v1beta2.StatefulSet, error) {
	log.Fatal("not implemented")
	return nil, nil
}
func (m *mockAccessor) GetStatefulSetForRack(c *cluster.Cluster, rack *v1alpha1.Rack) (*v1beta2.StatefulSet, error) {
	log.Fatal("not implemented")
	return nil, nil
}
func (m *mockAccessor) PatchStatefulSet(c *cluster.Cluster, rack *v1alpha1.Rack, patch string) (*v1beta2.StatefulSet, error) {
	log.Fatal("not implemented")
	return nil, nil
}
func (m *mockAccessor) CreateStatefulSetForRack(c *cluster.Cluster, rack *v1alpha1.Rack, customConfigMap *corev1.ConfigMap) (*v1beta2.StatefulSet, error) {
	log.Fatal("not implemented")
	return nil, nil
}


//
// HaveOperationsOfType matcher
//
func HaveOperationsOfType(operations []Operation) types.GomegaMatcher {
	return &haveOperations{expectedOperations: operations}
}

type haveOperations struct {
	expectedOperations []Operation
}

func (m *haveOperations) Match(actual interface{}) (success bool, err error) {
	operations := actual.([]Operation)
	return reflect.DeepEqual(m.typesOf(operations), m.typesOf(m.expectedOperations)), nil
}

func (m *haveOperations) FailureMessage(actual interface{}) (message string) {
	operations := actual.([]Operation)
	return fmt.Sprintf("Expected operations type to be the same. Expected: %v. Actual: %v.", m.typesOf(m.expectedOperations), m.typesOf(operations))
}

func (m *haveOperations) NegatedFailureMessage(actual interface{}) (message string) {
	operations := actual.([]Operation)
	return fmt.Sprintf("Expected operations type not to be the same. Expected: %v. Actual: %v.", m.typesOf(m.expectedOperations), m.typesOf(operations))
}

func (m *haveOperations) typesOf(ops []Operation) []reflect.Type {
	var opTypes []reflect.Type
	for _, op := range ops {
		opTypes = append(opTypes, reflect.TypeOf(op))
	}
	return opTypes
}
