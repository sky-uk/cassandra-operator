package cluster

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/record"

	"github.com/prometheus/common/log"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/client/clientset/versioned"
	"k8s.io/api/apps/v1beta2"
	"k8s.io/api/batch/v1beta1"
	v1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// Accessor exposes operations to access various kubernetes resources belonging to a Cluster
type Accessor struct {
	kubeClientset      *kubernetes.Clientset
	cassandraClientset *versioned.Clientset
	eventRecorder      record.EventRecorder
}

// NewAccessor creates a new Accessor
func NewAccessor(kubeClientset *kubernetes.Clientset, cassandraClientset *versioned.Clientset, eventRecorder record.EventRecorder) *Accessor {
	return &Accessor{
		kubeClientset:      kubeClientset,
		cassandraClientset: cassandraClientset,
		eventRecorder:      eventRecorder,
	}
}

// GetCassandraForCluster finds the Kubernetes resource which matches the supplied cluster definition
func (h *Accessor) GetCassandraForCluster(c *Cluster) (*v1alpha1.Cassandra, error) {
	return h.cassandraClientset.CoreV1alpha1().Cassandras(c.Namespace()).Get(c.Name(), metaV1.GetOptions{})
}

// CreateServiceForCluster creates a Kubernetes service from the supplied cluster definition
func (h *Accessor) CreateServiceForCluster(c *Cluster) (*v1.Service, error) {
	return h.kubeClientset.CoreV1().Services(c.Namespace()).Create(c.CreateService())
}

// FindCustomConfigMap looks for a custom config map which is associated with a given named cluster in a given
// Kubernetes namespace.
func (h *Accessor) FindCustomConfigMap(namespace, clusterName string) *v1.ConfigMap {
	customConfigMapName := fmt.Sprintf("%s-config", clusterName)
	configMap, err := h.kubeClientset.CoreV1().ConfigMaps(namespace).Get(customConfigMapName, metaV1.GetOptions{})
	if err == nil {
		return configMap
	}
	return nil
}

// CreateStatefulSetForRack creates a StatefulSet for a given within the supplied cluster definition
func (h *Accessor) CreateStatefulSetForRack(c *Cluster, rack *v1alpha1.Rack, customConfigMap *v1.ConfigMap) (*v1beta2.StatefulSet, error) {
	return h.kubeClientset.AppsV1beta2().StatefulSets(c.Namespace()).Create(c.CreateStatefulSetForRack(rack, customConfigMap))
}

// PatchStatefulSet applies a patch to a stateful set corresponding to the supplied rack in the supplied cluster
func (h *Accessor) PatchStatefulSet(c *Cluster, rack *v1alpha1.Rack, patch string) (*v1beta2.StatefulSet, error) {
	return h.kubeClientset.AppsV1beta2().StatefulSets(c.Namespace()).Patch(fmt.Sprintf("%s-%s", c.Name(), rack.Name), types.StrategicMergePatchType, []byte(patch))
}

// GetStatefulSetForRack returns the stateful set associated with the supplied rack in the supplied cluster
func (h *Accessor) GetStatefulSetForRack(c *Cluster, rack *v1alpha1.Rack) (*v1beta2.StatefulSet, error) {
	return h.kubeClientset.AppsV1beta2().StatefulSets(c.Namespace()).Get(fmt.Sprintf("%s-%s", c.Name(), rack.Name), metaV1.GetOptions{})
}

// UpdateStatefulSet updates the stateful set associated with the supplied cluster
func (h *Accessor) UpdateStatefulSet(c *Cluster, statefulSet *v1beta2.StatefulSet) (*v1beta2.StatefulSet, error) {
	return h.kubeClientset.AppsV1beta2().StatefulSets(c.Namespace()).Update(statefulSet)
}

// FindExistingResourcesFor finds Kubernetes services, stateful sets and pods associated with the supplied cluster
func (h *Accessor) FindExistingResourcesFor(c *Cluster) []string {
	labelSelector := fmt.Sprintf("%s=%s", OperatorLabel, c.Name())
	log.Infof("Searching for resources with label %s", labelSelector)
	listOptions := metaV1.ListOptions{LabelSelector: labelSelector}

	var foundResources []string
	services := h.serviceForCluster(c, listOptions)
	for _, service := range services {
		foundResources = append(foundResources, fmt.Sprintf("service:%s", service))
	}

	statefulSets := h.statefulSetsForCluster(c, listOptions)
	for _, statefulSet := range statefulSets {
		foundResources = append(foundResources, fmt.Sprintf("statefulset:%s", statefulSet))
	}

	pods := h.podsForCluster(c, listOptions)
	for _, pod := range pods {
		foundResources = append(foundResources, fmt.Sprintf("pod:%s", pod))
	}

	return foundResources
}

// WaitUntilRackChangeApplied waits until all pods related to the supplied rack in the supplied cluster are reporting as ready
func (h *Accessor) WaitUntilRackChangeApplied(cluster *Cluster, statefulSet *v1beta2.StatefulSet) error {
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

	if err := wait.PollImmediateInfinite(timeBetweenChecks, h.statefulSetChangeApplied(cluster, statefulSet)); err != nil {
		return fmt.Errorf("error while waiting for stateful set %s.%s creation to complete: %v", statefulSet.Namespace, statefulSet.Name, err)
	}

	h.recordWaitCompleteEvent(cluster, statefulSet)
	log.Infof("stateful set %s.%s is ready", statefulSet.Namespace, statefulSet.Name)

	return nil
}

// CreateCronJobForCluster creates a cronjob that will trigger the data snapshot for the supplied cluster
func (h *Accessor) CreateCronJobForCluster(c *Cluster, cronJob *v1beta1.CronJob) (*v1beta1.CronJob, error) {
	return h.kubeClientset.BatchV1beta1().CronJobs(c.Namespace()).Create(cronJob)
}

// FindCronJobForCluster finds the snapshot job for the specified cluster
func (h *Accessor) FindCronJobForCluster(cassandra *v1alpha1.Cassandra, label string) (*v1beta1.CronJob, error) {
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
func (h *Accessor) DeleteCronJob(job *v1beta1.CronJob) error {
	deletePropagation := metaV1.DeletePropagationBackground
	return h.kubeClientset.BatchV1beta1().CronJobs(job.Namespace).Delete(job.Name, &metaV1.DeleteOptions{PropagationPolicy: &deletePropagation})
}

// UpdateCronJob updates the given job
func (h *Accessor) UpdateCronJob(job *v1beta1.CronJob) error {
	_, err := h.kubeClientset.BatchV1beta1().CronJobs(job.Namespace).Update(job)
	return err
}

func (h *Accessor) serviceForCluster(c *Cluster, listOptions metaV1.ListOptions) []string {
	services, err := h.kubeClientset.CoreV1().Services(c.Namespace()).List(listOptions)
	var serviceNames []string
	if err != nil {
		log.Warnf("Unable to determine if service exists for cluster %s, assuming it doesn't: %v", c.QualifiedName(), err)
	} else {
		for _, svc := range services.Items {
			serviceNames = append(serviceNames, svc.Name)
		}
	}

	return serviceNames
}

func (h *Accessor) statefulSetsForCluster(c *Cluster, listOptions metaV1.ListOptions) []string {
	statefulSets, err := h.kubeClientset.AppsV1beta1().StatefulSets(c.Namespace()).List(listOptions)
	var setNames []string
	if err != nil {
		log.Warnf("Unable to determine if stateful sets exist for cluster %s, assuming they don't: %v", c.QualifiedName(), err)
	} else {
		for _, ss := range statefulSets.Items {
			setNames = append(setNames, ss.Name)
		}
	}

	return setNames
}

func (h *Accessor) podsForCluster(c *Cluster, listOptions metaV1.ListOptions) []string {
	pods, err := h.kubeClientset.CoreV1().Pods(c.Namespace()).List(listOptions)
	var podNames []string
	if err != nil {
		log.Warnf("Unable to determine if pods exist for cluster %s, assuming they don't: %v", c.QualifiedName(), err)
	} else {
		for _, pod := range pods.Items {
			podNames = append(podNames, pod.Name)
		}
	}

	return podNames
}

func (h *Accessor) statefulSetChangeApplied(cluster *Cluster, appliedStatefulSet *v1beta2.StatefulSet) func() (bool, error) {
	return func() (bool, error) {
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

func (h *Accessor) recordWaitEvent(cluster *Cluster, statefulSet *v1beta2.StatefulSet) {
	h.eventRecorder.Eventf(cluster.definition, v1.EventTypeNormal, WaitingForStatefulSetChange, "Waiting for stateful set %s.%s to be ready", statefulSet.Namespace, statefulSet.Name)
}

func (h *Accessor) recordWaitCompleteEvent(cluster *Cluster, statefulSet *v1beta2.StatefulSet) {
	h.eventRecorder.Eventf(cluster.definition, v1.EventTypeNormal, StatefulSetChangeComplete, "Stateful set %s.%s is ready", statefulSet.Namespace, statefulSet.Name)
}

func max(x, y int32) int32 {
	if x > y {
		return x
	}

	return y
}
