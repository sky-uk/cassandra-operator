package event

import (
	"fmt"
	"github.com/onsi/gomega/types"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/operator/operations"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test/apis"
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
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestReceiver(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Receiver Suite", test.CreateParallelReporters("receiver"))
}

var _ = Describe("operations to execute based on event", func() {
	var (
		receiver      *OperatorEventReceiver
		fakes         *mockAccessor
		oldClusterDef *v1alpha1.Cassandra
		newClusterDef *v1alpha1.Cassandra
	)

	BeforeEach(func() {
		oldClusterDef = apis.ACassandra().WithDefaults().WithSpec(
			apis.ACassandraSpec().
				WithDefaults().
				WithSnapshot(
					apis.ASnapshot().
						WithDefaults().
						WithRetentionPolicy(apis.ARetentionPolicy().WithDefaults()))).
			Build()
		newClusterDef = oldClusterDef.DeepCopy()

		fakes = &mockAccessor{}
		receiver = &OperatorEventReceiver{
			clusterAccessor:  fakes,
			operationFactory: operations.NewOperationFactory(nil, nil, nil, nil),
		}
	})

	Context("when a AddCluster event is received", func() {
		It("should return an add cluster operation", func() {
			// given
			newClusterDef.Spec.Snapshot = nil

			// when
			ops := receiver.operationsToExecute(&Event{Kind: AddCluster, Data: newClusterDef})

			//then
			Expect(ops).To(HaveOperationsOfType([]operations.Operation{
				&operations.AddClusterOperation{},
			}))
		})

		Context("a snapshot spec exists", func() {

			It("should return add cluster and add snapshot operations", func() {
				// given
				newClusterDef.Spec.Snapshot.RetentionPolicy = nil

				// when
				ops := receiver.operationsToExecute(&Event{Kind: AddCluster, Data: newClusterDef})

				//then
				Expect(ops).To(HaveOperationsOfType([]operations.Operation{
					&operations.AddClusterOperation{},
					&operations.AddSnapshotOperation{},
				}))
			})

			It("should return add cluster, add snapshot and add cleanup snapshot operations when a snapshot retention spec exists", func() {
				// when
				ops := receiver.operationsToExecute(&Event{Kind: AddCluster, Data: newClusterDef})

				//then
				Expect(ops).To(HaveOperationsOfType([]operations.Operation{
					&operations.AddClusterOperation{},
					&operations.AddSnapshotOperation{},
					&operations.AddSnapshotCleanupOperation{},
				}))
			})
		})

	})

	Context("when a DeleteCluster event is received", func() {
		It("should return a delete cluster operation", func() {
			// given
			newClusterDef.Spec.Snapshot = nil

			// when
			ops := receiver.operationsToExecute(&Event{Kind: DeleteCluster, Data: newClusterDef})

			//then
			Expect(ops).To(HaveOperationsOfType([]operations.Operation{
				&operations.DeleteClusterOperation{},
			}))
		})

		Context("a snapshot spec exists", func() {
			It("should return a delete cluster and delete snapshot operations", func() {
				// given
				newClusterDef.Spec.Snapshot.RetentionPolicy = nil

				// when
				ops := receiver.operationsToExecute(&Event{Kind: DeleteCluster, Data: newClusterDef})

				//then
				Expect(ops).To(HaveOperationsOfType([]operations.Operation{
					&operations.DeleteClusterOperation{},
					&operations.DeleteSnapshotOperation{},
				}))
			})
			It("should return a delete cluster, delete snapshot and delete snapshot cleanup operations when a retention policy is defined", func() {
				// when
				ops := receiver.operationsToExecute(&Event{Kind: DeleteCluster, Data: newClusterDef})

				//then
				Expect(ops).To(HaveOperationsOfType([]operations.Operation{
					&operations.DeleteClusterOperation{},
					&operations.DeleteSnapshotOperation{},
					&operations.DeleteSnapshotCleanupOperation{},
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
				ops := receiver.operationsToExecute(&Event{Kind: UpdateCluster, Data: ClusterUpdate{OldCluster: oldClusterDef, NewCluster: newClusterDef}})
				Expect(ops).To(HaveLen(0))
			})

			It("should return no operations when the cluster is beind deleted", func() {
				// given
				fakes.cassandraIsBeingDeleted(newClusterDef)

				// when
				ops := receiver.operationsToExecute(&Event{Kind: UpdateCluster, Data: ClusterUpdate{OldCluster: oldClusterDef, NewCluster: newClusterDef}})
				Expect(ops).To(HaveLen(0))
			})

		})

		Context("the cluster exists", func() {
			BeforeEach(func() {
				fakes.cassandraExists(newClusterDef)
			})

			It("should return an update cluster operation", func() {
				// when
				ops := receiver.operationsToExecute(&Event{Kind: UpdateCluster, Data: ClusterUpdate{OldCluster: oldClusterDef, NewCluster: newClusterDef}})

				// then
				Expect(ops).To(HaveOperationsOfType([]operations.Operation{
					&operations.UpdateClusterOperation{},
				}))
			})

			Context("a snapshot or snapshot cleanup spec is removed", func() {
				It("should return update cluster, delete snapshot and delete snapshot cleanup when snapshot spec is removed", func() {
					// given
					newClusterDef.Spec.Snapshot = nil

					// when
					ops := receiver.operationsToExecute(&Event{Kind: UpdateCluster, Data: ClusterUpdate{OldCluster: oldClusterDef, NewCluster: newClusterDef}})

					// then
					Expect(ops).To(HaveOperationsOfType([]operations.Operation{
						&operations.UpdateClusterOperation{},
						&operations.DeleteSnapshotOperation{},
						&operations.DeleteSnapshotCleanupOperation{},
					}))
				})
				It("should return update cluster and delete snapshot cleanup when snapshot retention policy is removed", func() {
					// given
					newClusterDef.Spec.Snapshot.RetentionPolicy = nil

					// when
					ops := receiver.operationsToExecute(&Event{Kind: UpdateCluster, Data: ClusterUpdate{OldCluster: oldClusterDef, NewCluster: newClusterDef}})

					// then
					Expect(ops).To(HaveOperationsOfType([]operations.Operation{
						&operations.UpdateClusterOperation{},
						&operations.DeleteSnapshotCleanupOperation{},
					}))
				})
			})

			Context("a snapshot or snapshot cleanup is added", func() {
				It("should return update cluster, add snapshot and add snapshot cleanup when snapshot with retention policy is added", func() {
					// given
					oldClusterDef.Spec.Snapshot = nil

					// when
					ops := receiver.operationsToExecute(&Event{Kind: UpdateCluster, Data: ClusterUpdate{OldCluster: oldClusterDef, NewCluster: newClusterDef}})

					// then
					Expect(ops).To(HaveOperationsOfType([]operations.Operation{
						&operations.UpdateClusterOperation{},
						&operations.AddSnapshotOperation{},
						&operations.AddSnapshotCleanupOperation{},
					}))
				})
				It("should return update cluster and add snapshot when snapshot without retention policy is added", func() {
					// given
					oldClusterDef.Spec.Snapshot = nil
					newClusterDef.Spec.Snapshot.RetentionPolicy = nil

					// when
					ops := receiver.operationsToExecute(&Event{Kind: UpdateCluster, Data: ClusterUpdate{OldCluster: oldClusterDef, NewCluster: newClusterDef}})

					// then
					Expect(ops).To(HaveOperationsOfType([]operations.Operation{
						&operations.UpdateClusterOperation{},
						&operations.AddSnapshotOperation{},
					}))
				})
			})

			Context("a snapshot is updated", func() {
				It("should return update cluster and update snapshot", func() {
					// given
					newClusterDef.Spec.Snapshot.Schedule = "1 13 4 * *"

					// when
					ops := receiver.operationsToExecute(&Event{Kind: UpdateCluster, Data: ClusterUpdate{OldCluster: oldClusterDef, NewCluster: newClusterDef}})

					// then
					Expect(ops).To(HaveOperationsOfType([]operations.Operation{
						&operations.UpdateClusterOperation{},
						&operations.UpdateSnapshotOperation{},
					}))
				})
			})

			Context("a snapshot retention policy is updated", func() {
				It("should return update cluster and update snapshot cleanup", func() {
					// given
					newClusterDef.Spec.Snapshot.RetentionPolicy.CleanupSchedule = "1 13 4 * *"

					// when
					ops := receiver.operationsToExecute(&Event{Kind: UpdateCluster, Data: ClusterUpdate{OldCluster: oldClusterDef, NewCluster: newClusterDef}})

					// then
					Expect(ops).To(HaveOperationsOfType([]operations.Operation{
						&operations.UpdateClusterOperation{},
						&operations.UpdateSnapshotCleanupOperation{},
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
				ops := receiver.operationsToExecute(&Event{Kind: GatherMetrics, Data: newClusterDef})
				Expect(ops).To(HaveLen(0))
			})

			It("should return no operations when the cluster is being deleted", func() {
				// given
				fakes.cassandraIsBeingDeleted(newClusterDef)

				// when
				ops := receiver.operationsToExecute(&Event{Kind: GatherMetrics, Data: newClusterDef})
				Expect(ops).To(HaveLen(0))
			})

		})

		Context("the cluster exists", func() {
			BeforeEach(func() {
				fakes.cassandraExists(newClusterDef)
			})
			It("should return a gather metrics operation", func() {
				// when
				ops := receiver.operationsToExecute(&Event{Kind: GatherMetrics, Data: newClusterDef})

				// then
				Expect(ops).To(HaveLen(1))
				Expect(reflect.TypeOf(ops[0])).To(Equal(reflect.TypeOf(&operations.GatherMetricsOperation{})))
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
				ops := receiver.operationsToExecute(&Event{Kind: UpdateCustomConfig, Data: configMapChange})
				Expect(ops).To(HaveLen(0))
			})

			It("should return no operations when the cluster is beind deleted", func() {
				// given
				fakes.cassandraIsBeingDeleted(newClusterDef)

				// when
				ops := receiver.operationsToExecute(&Event{Kind: UpdateCustomConfig, Data: configMapChange})
				Expect(ops).To(HaveLen(0))
			})

		})

		Context("the cluster exists", func() {

			It("should return an update custom config operation", func() {
				// given
				fakes.cassandraExists(newClusterDef)

				// when
				ops := receiver.operationsToExecute(&Event{Kind: UpdateCustomConfig, Data: configMapChange})

				// then
				Expect(ops).To(HaveLen(1))
				Expect(reflect.TypeOf(ops[0])).To(Equal(reflect.TypeOf(&operations.UpdateCustomConfigOperation{})))
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
				ops := receiver.operationsToExecute(&Event{Kind: AddCustomConfig, Data: configMapChange})
				Expect(ops).To(HaveLen(0))
			})

			It("should return no operations when the cluster is beind deleted", func() {
				// given
				fakes.cassandraIsBeingDeleted(newClusterDef)

				// when
				ops := receiver.operationsToExecute(&Event{Kind: AddCustomConfig, Data: configMapChange})
				Expect(ops).To(HaveLen(0))
			})

		})

		Context("the cluster exists", func() {
			It("should return an add custom config operation", func() {
				// given
				fakes.cassandraExists(newClusterDef)

				// when
				ops := receiver.operationsToExecute(&Event{Kind: AddCustomConfig, Data: configMapChange})

				// then
				Expect(ops).To(HaveLen(1))
				Expect(reflect.TypeOf(ops[0])).To(Equal(reflect.TypeOf(&operations.AddCustomConfigOperation{})))
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
				ops := receiver.operationsToExecute(&Event{Kind: DeleteCustomConfig, Data: configMapChange})
				Expect(ops).To(HaveLen(0))
			})

			It("should return no operations when the cluster is beind deleted", func() {
				// given
				fakes.cassandraIsBeingDeleted(newClusterDef)

				// when
				ops := receiver.operationsToExecute(&Event{Kind: DeleteCustomConfig, Data: configMapChange})
				Expect(ops).To(HaveLen(0))
			})

		})

		Context("the cluster exists", func() {
			BeforeEach(func() {
				fakes.cassandraExists(newClusterDef)
			})

			It("should return a delete custom config operation", func() {
				// when
				ops := receiver.operationsToExecute(&Event{Kind: DeleteCustomConfig, Data: configMapChange})

				// then
				Expect(ops).To(HaveLen(1))
				Expect(reflect.TypeOf(ops[0])).To(Equal(reflect.TypeOf(&operations.DeleteCustomConfigOperation{})))
			})
		})
	})

	Context("when a AddService event is received", func() {
		It("should return an add service operation", func() {
			// when
			ops := receiver.operationsToExecute(&Event{Kind: AddService, Data: newClusterDef})

			//then
			Expect(ops).To(HaveLen(1))
			Expect(reflect.TypeOf(ops[0])).To(Equal(reflect.TypeOf(&operations.AddServiceOperation{})))
		})
	})
})

var _ = Describe("operations execution", func() {
	var (
		receiver *OperatorEventReceiver
		fakes    *mockOperationFactory
	)

	BeforeEach(func() {
		fakes = &mockOperationFactory{}
		receiver = &OperatorEventReceiver{
			operationFactory:  fakes,
			operationComposer: operations.NewOperationComposer(),
		}
	})

	It("should execute subsequent operations when one fails", func() {
		// given
		clusterDef := apis.ACassandra().WithDefaults().WithSpec(
			apis.ACassandraSpec().
				WithDefaults().
				WithSnapshot(
					apis.ASnapshot().
						WithDefaults().
						WithRetentionPolicy(apis.ARetentionPolicy().WithDefaults()))).
			Build()
		fakes.On("NewAddCluster", clusterDef).Return(failureOperation("some error"))
		fakes.On("NewAddSnapshot", clusterDef).Return(failureOperation("some other error"))
		fakes.On("NewAddSnapshotCleanup", clusterDef).Return(successfulOperation())

		// when
		err := receiver.Receive(&Event{Kind: AddCluster, Key: "mycluster", Data: clusterDef})

		// then
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("some error"))
		Expect(err.Error()).To(ContainSubstring("some other error"))
	})
})

type stubOperation struct {
	msg     string
	success bool
}

func successfulOperation() operations.Operation {
	return &stubOperation{"success", true}
}

func failureOperation(msg string) operations.Operation {
	return &stubOperation{msg: msg, success: false}
}

func (o *stubOperation) Execute() (bool, error) {
	if o.success {
		return false, nil
	}

	return false, fmt.Errorf(o.msg)
}

func (o *stubOperation) String() string {
	return "stubOperation"
}

//
type mockOperationFactory struct {
	mock.Mock
}

var _ operations.OperationFactory = &mockOperationFactory{}

func (o *mockOperationFactory) NewAddCluster(cassandra *v1alpha1.Cassandra) operations.Operation {
	args := o.Called(cassandra)
	return args.Get(0).(operations.Operation)
}
func (o *mockOperationFactory) NewAddService(cassandra *v1alpha1.Cassandra) operations.Operation {
	args := o.Called(cassandra)
	return args.Get(0).(operations.Operation)
}
func (o *mockOperationFactory) NewDeleteCluster(cassandra *v1alpha1.Cassandra) operations.Operation {
	args := o.Called(cassandra)
	return args.Get(0).(operations.Operation)
}
func (o *mockOperationFactory) NewDeleteSnapshot(cassandra *v1alpha1.Cassandra) operations.Operation {
	args := o.Called(cassandra)
	return args.Get(0).(operations.Operation)
}
func (o *mockOperationFactory) NewDeleteSnapshotCleanup(cassandra *v1alpha1.Cassandra) operations.Operation {
	args := o.Called(cassandra)
	return args.Get(0).(operations.Operation)
}
func (o *mockOperationFactory) NewAddSnapshot(cassandra *v1alpha1.Cassandra) operations.Operation {
	args := o.Called(cassandra)
	return args.Get(0).(operations.Operation)
}
func (o *mockOperationFactory) NewAddSnapshotCleanup(cassandra *v1alpha1.Cassandra) operations.Operation {
	args := o.Called(cassandra)
	return args.Get(0).(operations.Operation)
}
func (o *mockOperationFactory) NewUpdateCluster(oldCassandra, newCassandra *v1alpha1.Cassandra) operations.Operation {
	args := o.Called(oldCassandra, newCassandra)
	return args.Get(0).(operations.Operation)
}
func (o *mockOperationFactory) NewUpdateSnapshot(cassandra *v1alpha1.Cassandra) operations.Operation {
	args := o.Called(cassandra)
	return args.Get(0).(operations.Operation)
}
func (o *mockOperationFactory) NewUpdateSnapshotCleanup(cassandra *v1alpha1.Cassandra) operations.Operation {
	args := o.Called(cassandra)
	return args.Get(0).(operations.Operation)
}
func (o *mockOperationFactory) NewGatherMetrics(cassandra *v1alpha1.Cassandra) operations.Operation {
	args := o.Called(cassandra)
	return args.Get(0).(operations.Operation)
}
func (o *mockOperationFactory) NewUpdateCustomConfig(cassandra *v1alpha1.Cassandra, configMap *corev1.ConfigMap) operations.Operation {
	args := o.Called(cassandra)
	return args.Get(0).(operations.Operation)
}
func (o *mockOperationFactory) NewAddCustomConfig(cassandra *v1alpha1.Cassandra, configMap *corev1.ConfigMap) operations.Operation {
	args := o.Called(cassandra)
	return args.Get(0).(operations.Operation)
}
func (o *mockOperationFactory) NewDeleteCustomConfig(cassandra *v1alpha1.Cassandra) operations.Operation {
	args := o.Called(cassandra)
	return args.Get(0).(operations.Operation)
}

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
func HaveOperationsOfType(operations []operations.Operation) types.GomegaMatcher {
	return &haveOperations{expectedOperations: operations}
}

type haveOperations struct {
	expectedOperations []operations.Operation
}

func (m *haveOperations) Match(actual interface{}) (success bool, err error) {
	ops := actual.([]operations.Operation)
	return reflect.DeepEqual(m.typesOf(ops), m.typesOf(m.expectedOperations)), nil
}

func (m *haveOperations) FailureMessage(actual interface{}) (message string) {
	ops := actual.([]operations.Operation)
	return fmt.Sprintf("Expected operations type to be the same. Expected: %v. Actual: %v.", m.typesOf(m.expectedOperations), m.typesOf(ops))
}

func (m *haveOperations) NegatedFailureMessage(actual interface{}) (message string) {
	ops := actual.([]operations.Operation)
	return fmt.Sprintf("Expected operations type not to be the same. Expected: %v. Actual: %v.", m.typesOf(m.expectedOperations), m.typesOf(ops))
}

func (m *haveOperations) typesOf(ops []operations.Operation) []reflect.Type {
	var opTypes []reflect.Type
	for _, op := range ops {
		opTypes = append(opTypes, reflect.TypeOf(op))
	}
	return opTypes
}
