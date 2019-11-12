package cluster

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/record"

	"github.com/prometheus/common/log"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/client/clientset/versioned"
	"k8s.io/api/apps/v1beta2"
	"k8s.io/api/batch/v1beta1"
	"k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// ActiveReconciliation is a record of a reconciliation which is currently in progress. It offers a way to mark a
// reconciliation as complete or interrupted.
type ActiveReconciliation struct {
	CassandraRevision string
	ConfigMapRevision string
	interruptChannel  chan struct{}
	complete          bool

	sync.Mutex
}

// NewReconciliation creates a new ActiveReconciliation
func NewReconciliation(cassandraRevision, configMapRevision string) *ActiveReconciliation {
	return &ActiveReconciliation{
		CassandraRevision: cassandraRevision,
		ConfigMapRevision: configMapRevision,
		interruptChannel:  make(chan struct{}),
		complete:          false,
	}
}

// Interrupt will send an interrupt message down the interruptChannel, if the reconciliation isn't already complete.
func (a *ActiveReconciliation) Interrupt() {
	a.Lock()
	defer a.Unlock()

	if !a.complete {
		a.complete = true
		a.interruptChannel <- struct{}{}
	}
}

// Complete will mark a not-yet-complete reconciliation as complete, and close its interruptChannel.
func (a *ActiveReconciliation) Complete() {
	a.Lock()
	defer a.Unlock()

	if !a.complete {
		a.complete = true
		close(a.interruptChannel)
	}
}

// InterruptChannel returns a receive-only channel which should be checked for an interrupt message, which would
// mean the reconciliation should be interrupted.
func (a *ActiveReconciliation) InterruptChannel() <-chan struct{} {
	return a.interruptChannel
}

// ErrReconciliationInterrupted is an error which signifies that a reconciliation was interrupted.
var ErrReconciliationInterrupted = errors.New("reconciliation interrupted by a more recent version")

// Accessor exposes operations to access various kubernetes resources belonging to a Cluster
type Accessor interface {
	GetCassandraForCluster(namespace, clusterName string) (*v1alpha1.Cassandra, error)
	CreateServiceForCluster(c *Cluster) (*v1.Service, error)
	FindCustomConfigMap(namespace, clusterName string) *v1.ConfigMap
	UpdateCronJob(job *v1beta1.CronJob) error
	DeleteCronJob(job *v1beta1.CronJob) error
	FindCronJobForCluster(cassandra *v1alpha1.Cassandra, label string) (*v1beta1.CronJob, error)
	CreateCronJobForCluster(c *Cluster, cronJob *v1beta1.CronJob) (*v1beta1.CronJob, error)
	WaitUntilRackChangeApplied(cluster *Cluster, statefulSet *v1beta2.StatefulSet) error
	UpdateStatefulSet(c *Cluster, statefulSet *v1beta2.StatefulSet) (*v1beta2.StatefulSet, error)
	GetStatefulSetForRack(c *Cluster, rack *v1alpha1.Rack) (*v1beta2.StatefulSet, error)
	PatchStatefulSet(c *Cluster, rack *v1alpha1.Rack, patch string) (*v1beta2.StatefulSet, error)
	CreateStatefulSetForRack(c *Cluster, rack *v1alpha1.Rack, customConfigMap *v1.ConfigMap) (*v1beta2.StatefulSet, error)
}

// clientsetBasedAccessor implements Accessor
type clientsetBasedAccessor struct {
	kubeClientset         *kubernetes.Clientset
	cassandraClientset    *versioned.Clientset
	eventRecorder         record.EventRecorder
	activeReconciliations *sync.Map
}

// NewAccessor creates a new Accessor
func NewAccessor(kubeClientset *kubernetes.Clientset, cassandraClientset *versioned.Clientset, eventRecorder record.EventRecorder, activeReconciliations *sync.Map) Accessor {
	return &clientsetBasedAccessor{
		kubeClientset:         kubeClientset,
		cassandraClientset:    cassandraClientset,
		eventRecorder:         eventRecorder,
		activeReconciliations: activeReconciliations,
	}
}

// GetCassandraForCluster finds the Kubernetes resource which matches the supplied cluster definition
func (h *clientsetBasedAccessor) GetCassandraForCluster(namespace, clusterName string) (*v1alpha1.Cassandra, error) {
	return h.cassandraClientset.CoreV1alpha1().Cassandras(namespace).Get(clusterName, metaV1.GetOptions{})
}

// CreateServiceForCluster creates a Kubernetes service from the supplied cluster definition
func (h *clientsetBasedAccessor) CreateServiceForCluster(c *Cluster) (*v1.Service, error) {
	return h.kubeClientset.CoreV1().Services(c.Namespace()).Create(c.CreateService())
}

// FindCustomConfigMap looks for a custom config map which is associated with a given named cluster in a given
// Kubernetes namespace.
func (h *clientsetBasedAccessor) FindCustomConfigMap(namespace, clusterName string) *v1.ConfigMap {
	customConfigMapName := fmt.Sprintf("%s-config", clusterName)
	configMap, err := h.kubeClientset.CoreV1().ConfigMaps(namespace).Get(customConfigMapName, metaV1.GetOptions{})
	if err == nil {
		return configMap
	}
	return nil
}

// CreateStatefulSetForRack creates a StatefulSet for a given within the supplied cluster definition
func (h *clientsetBasedAccessor) CreateStatefulSetForRack(c *Cluster, rack *v1alpha1.Rack, customConfigMap *v1.ConfigMap) (*v1beta2.StatefulSet, error) {
	return h.kubeClientset.AppsV1beta2().StatefulSets(c.Namespace()).Create(c.CreateStatefulSetForRack(rack, customConfigMap))
}

// PatchStatefulSet applies a patch to a stateful set corresponding to the supplied rack in the supplied cluster
func (h *clientsetBasedAccessor) PatchStatefulSet(c *Cluster, rack *v1alpha1.Rack, patch string) (*v1beta2.StatefulSet, error) {
	return h.kubeClientset.AppsV1beta2().StatefulSets(c.Namespace()).Patch(fmt.Sprintf("%s-%s", c.Name(), rack.Name), types.StrategicMergePatchType, []byte(patch))
}

// GetStatefulSetForRack returns the stateful set associated with the supplied rack in the supplied cluster
func (h *clientsetBasedAccessor) GetStatefulSetForRack(c *Cluster, rack *v1alpha1.Rack) (*v1beta2.StatefulSet, error) {
	return h.kubeClientset.AppsV1beta2().StatefulSets(c.Namespace()).Get(fmt.Sprintf("%s-%s", c.Name(), rack.Name), metaV1.GetOptions{})
}

// UpdateStatefulSet updates the stateful set associated with the supplied cluster
func (h *clientsetBasedAccessor) UpdateStatefulSet(c *Cluster, statefulSet *v1beta2.StatefulSet) (*v1beta2.StatefulSet, error) {
	return h.kubeClientset.AppsV1beta2().StatefulSets(c.Namespace()).Update(statefulSet)
}

// WaitUntilRackChangeApplied waits until all pods related to the supplied rack in the supplied cluster are reporting as ready
func (h *clientsetBasedAccessor) WaitUntilRackChangeApplied(cluster *Cluster, statefulSet *v1beta2.StatefulSet) error {
	log.Infof("waiting for stateful set %s.%s to be ready", statefulSet.Namespace, statefulSet.Name)
	h.recordWaitEvent(cluster, statefulSet)

	// there's no point running the first check until at least enough time has passed for the readiness check to pass
	// on all replicas of the stateful set. similarly, there's no point in checking more often than the readiness probe
	// checks.
	readinessProbe := cluster.definition.Spec.Pod.ReadinessProbe
	timeBeforeFirstCheck := time.Duration(*statefulSet.Spec.Replicas**readinessProbe.InitialDelaySeconds) * time.Second

	// have a lower limit of 5 seconds for time between checks, to avoid spamming events.
	timeBetweenChecks := time.Duration(max(*readinessProbe.PeriodSeconds, 5)) * time.Second

	// time.Sleep is fine for us to use because this check is executed in its own goroutine and won't block any other
	// operations on other clusters.
	time.Sleep(timeBeforeFirstCheck)

	reconciliation := h.activeReconciliationForCluster(cluster)
	h.activeReconciliations.Store(cluster.definition.QualifiedName(), reconciliation)
	defer func() {
		h.activeReconciliations.Delete(cluster.definition.QualifiedName())
		reconciliation.Complete()
	}()

	if err := wait.PollImmediateInfinite(timeBetweenChecks, h.statefulSetChangeApplied(cluster, statefulSet, reconciliation.InterruptChannel())); err != nil {
		if err == ErrReconciliationInterrupted {
			return err
		}

		return fmt.Errorf("error while waiting for stateful set %s.%s creation to complete: %v", statefulSet.Namespace, statefulSet.Name, err)
	}

	h.recordWaitCompleteEvent(cluster, statefulSet)
	log.Infof("stateful set %s.%s is ready", statefulSet.Namespace, statefulSet.Name)

	return nil
}

func (h *clientsetBasedAccessor) activeReconciliationForCluster(cluster *Cluster) *ActiveReconciliation {
	configMap := h.FindCustomConfigMap(cluster.definition.Namespace, cluster.definition.Name)
	var configMapVersion string
	if configMap != nil {
		configMapVersion = configMap.ResourceVersion
	}
	reconciliation := NewReconciliation(cluster.definition.ResourceVersion, configMapVersion)
	return reconciliation
}

// CreateCronJobForCluster creates a cronjob that will trigger the data snapshot for the supplied cluster
func (h *clientsetBasedAccessor) CreateCronJobForCluster(c *Cluster, cronJob *v1beta1.CronJob) (*v1beta1.CronJob, error) {
	return h.kubeClientset.BatchV1beta1().CronJobs(c.Namespace()).Create(cronJob)
}

// FindCronJobForCluster finds the snapshot job for the specified cluster
func (h *clientsetBasedAccessor) FindCronJobForCluster(cassandra *v1alpha1.Cassandra, label string) (*v1beta1.CronJob, error) {
	cronJobList, err := h.kubeClientset.BatchV1beta1().CronJobs(cassandra.Namespace).List(metaV1.ListOptions{LabelSelector: label})
	if err != nil {
		return nil, err
	}

	if len(cronJobList.Items) > 1 {
		return nil, fmt.Errorf("found %d cronjobs with label %s for cluster %s when expecting just one", len(cronJobList.Items), label, cassandra.QualifiedName())
	} else if len(cronJobList.Items) == 0 {
		return nil, nil
	}

	return &cronJobList.Items[0], nil
}

// DeleteCronJob deletes the given job
func (h *clientsetBasedAccessor) DeleteCronJob(job *v1beta1.CronJob) error {
	deletePropagation := metaV1.DeletePropagationBackground
	return h.kubeClientset.BatchV1beta1().CronJobs(job.Namespace).Delete(job.Name, &metaV1.DeleteOptions{PropagationPolicy: &deletePropagation})
}

// UpdateCronJob updates the given job
func (h *clientsetBasedAccessor) UpdateCronJob(job *v1beta1.CronJob) error {
	_, err := h.kubeClientset.BatchV1beta1().CronJobs(job.Namespace).Update(job)
	return err
}

func (h *clientsetBasedAccessor) statefulSetChangeApplied(cluster *Cluster, appliedStatefulSet *v1beta2.StatefulSet, stopCh <-chan struct{}) func() (bool, error) {
	return func() (bool, error) {
		select {
		case <-stopCh:
			return false, ErrReconciliationInterrupted

		default:
			currentStatefulSet, err := h.kubeClientset.AppsV1beta1().StatefulSets(appliedStatefulSet.Namespace).Get(appliedStatefulSet.Name, metaV1.GetOptions{})
			if err != nil {
				return false, err
			}

			controllerObservedChange := currentStatefulSet.Status.ObservedGeneration != nil &&
				*currentStatefulSet.Status.ObservedGeneration >= appliedStatefulSet.Generation
			updateCompleted := currentStatefulSet.Status.UpdateRevision == currentStatefulSet.Status.CurrentRevision
			allReplicasReady := currentStatefulSet.Status.ReadyReplicas == currentStatefulSet.Status.Replicas

			return controllerObservedChange && updateCompleted && allReplicasReady, nil
		}
	}
}

func (h *clientsetBasedAccessor) recordWaitEvent(cluster *Cluster, statefulSet *v1beta2.StatefulSet) {
	h.eventRecorder.Eventf(cluster.definition, v1.EventTypeNormal, WaitingForStatefulSetChange, "Waiting for stateful set %s.%s to be ready", statefulSet.Namespace, statefulSet.Name)
}

func (h *clientsetBasedAccessor) recordWaitCompleteEvent(cluster *Cluster, statefulSet *v1beta2.StatefulSet) {
	h.eventRecorder.Eventf(cluster.definition, v1.EventTypeNormal, StatefulSetChangeComplete, "Stateful set %s.%s is ready", statefulSet.Namespace, statefulSet.Name)
}

func max(x, y int32) int32 {
	if x > y {
		return x
	}

	return y
}
