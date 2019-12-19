package e2e

import (
	"fmt"
	"github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	"io/ioutil"
	appsV1 "k8s.io/api/apps/v1beta2"
	"k8s.io/api/batch/v1beta1"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"regexp"
	"strings"
	"time"
)

func PersistentVolumeClaimsForCluster(namespace, clusterName string) func() ([]*kubernetesResource, error) {
	return persistentVolumeClaimsWithLabel(namespace, labelSelectorForCluster(namespace, clusterName))
}

func persistentVolumeClaimsWithLabel(namespace, label string) func() ([]*kubernetesResource, error) {
	return func() ([]*kubernetesResource, error) {
		pvcClient := KubeClientset.CoreV1().PersistentVolumeClaims(namespace)
		pvcList, err := pvcClient.List(metaV1.ListOptions{LabelSelector: label})
		if err != nil {
			return nil, err
		}

		var kubernetesResources []*kubernetesResource
		for _, item := range pvcList.Items {
			kubernetesResources = append(kubernetesResources, &kubernetesResource{item})
		}
		return kubernetesResources, nil
	}
}

func StatefulSetsForCluster(namespace, clusterName string) func() ([]*kubernetesResource, error) {
	return statefulSetsWithLabel(namespace, labelSelectorForCluster(namespace, clusterName))
}

func statefulSetsWithLabel(namespace, label string) func() ([]*kubernetesResource, error) {
	return func() ([]*kubernetesResource, error) {
		ssClient := KubeClientset.AppsV1beta2().StatefulSets(namespace)
		result, err := ssClient.List(metaV1.ListOptions{LabelSelector: label})
		if err != nil {
			return nil, err
		}

		var kubernetesResources []*kubernetesResource
		for _, item := range result.Items {
			kubernetesResources = append(kubernetesResources, &kubernetesResource{item})
		}
		return kubernetesResources, nil
	}
}

func StatefulSetForRack(namespace, clusterName, rack string) (*kubernetesResource, error) {
	statefulSet, err := KubeClientset.AppsV1beta2().StatefulSets(namespace).Get(fmt.Sprintf("%s-%s", clusterName, rack), metaV1.GetOptions{})
	return &kubernetesResource{statefulSet}, err
}

func HeadlessServiceForCluster(namespace, clusterName string) func() (*kubernetesResource, error) {
	return func() (*kubernetesResource, error) {
		svcClient := KubeClientset.CoreV1().Services(namespace)
		result, err := svcClient.Get(clusterName, metaV1.GetOptions{})
		if err != nil {
			return nil, errorUnlessNotFound(err)
		}

		return &kubernetesResource{result}, nil
	}
}

func PodsForCluster(namespace, clusterName string) func() ([]*kubernetesResource, error) {
	return podsWithLabel(namespace, labelSelectorForCluster(namespace, clusterName))
}

func podsWithLabel(namespace, label string) func() ([]*kubernetesResource, error) {
	return func() ([]*kubernetesResource, error) {
		podInterface := KubeClientset.CoreV1().Pods(namespace)
		podList, err := podInterface.List(metaV1.ListOptions{LabelSelector: label})
		if err != nil {
			return nil, err
		}

		var kubernetesResources []*kubernetesResource
		for _, item := range podList.Items {
			kubernetesResources = append(kubernetesResources, &kubernetesResource{item})
		}
		return kubernetesResources, nil
	}
}

func CronJobsForCluster(namespace, clusterName string) func() ([]*kubernetesResource, error) {
	return func() ([]*kubernetesResource, error) {
		jobList, err := KubeClientset.BatchV1beta1().CronJobs(namespace).List(metaV1.ListOptions{
			LabelSelector: labelSelectorForCluster(namespace, clusterName),
		})
		if err != nil {
			return nil, err
		}

		var kubernetesResources []*kubernetesResource
		for _, item := range jobList.Items {
			kubernetesResources = append(kubernetesResources, &kubernetesResource{item})
		}
		return kubernetesResources, nil
	}
}

func CronJob(namespace, jobName string) func() (*kubernetesResource, error) {
	return func() (*kubernetesResource, error) {
		job, err := KubeClientset.BatchV1beta1().CronJobs(namespace).Get(jobName, metaV1.GetOptions{})
		if err != nil {
			return nil, errorUnlessNotFound(err)
		}

		return &kubernetesResource{job}, nil
	}
}

type kubernetesResource struct {
	Resource interface{}
}

func (k *kubernetesResource) Labels() map[string]string {
	return k.ObjectMeta().Labels
}

func (k *kubernetesResource) CreationTimestamp() metaV1.Time {
	return k.ObjectMeta().CreationTimestamp
}

func (k *kubernetesResource) ObjectMeta() *metaV1.ObjectMeta {
	switch r := k.Resource.(type) {
	case *coreV1.Service:
		return &r.ObjectMeta
	case coreV1.Service:
		return &r.ObjectMeta
	case *appsV1.StatefulSet:
		return &r.ObjectMeta
	case appsV1.StatefulSet:
		return &r.ObjectMeta
	case *coreV1.PersistentVolumeClaim:
		return &r.ObjectMeta
	case coreV1.PersistentVolumeClaim:
		return &r.ObjectMeta
	case *coreV1.Pod:
		return &r.ObjectMeta
	case coreV1.Pod:
		return &r.ObjectMeta
	case *v1beta1.CronJob:
		return &r.ObjectMeta
	case v1beta1.CronJob:
		return &r.ObjectMeta
	default:
		fmt.Printf("Unknown resource type %v. Cannot locate ObjectMeta", r)
		return nil
	}
}

func OperatorMetrics(namespace string) func() (string, error) {
	return func() (string, error) {
		resp, err := KubeClientset.CoreV1().Services(namespace).
			ProxyGet("", "cassandra-operator", "http", "metrics", map[string]string{}).
			Stream()
		if err != nil {
			log.Errorf("error while retrieving metrics via Kube ApiServer Proxy, %v", err)
			return "", err
		}
		body, err := ioutil.ReadAll(resp)
		if err != nil {
			log.Errorf("error while reading response body via Kube ApiServer Proxy, %v", err)
			return "", err
		}
		return string(body), nil
	}
}

func PodReadinessStatus(namespace, podName string) func() (bool, error) {
	return func() (bool, error) {
		pod, err := KubeClientset.CoreV1().Pods(namespace).Get(podName, metaV1.GetOptions{})
		if err != nil {
			return false, err
		}

		if !podReady(pod) {
			return false, fmt.Errorf("at least one container for pod %s is not ready", podName)

		}
		return true, nil
	}
}

func PodCreationTime(namespace, podName string) func() (time.Time, error) {
	return func() (time.Time, error) {
		pod, err := KubeClientset.CoreV1().Pods(namespace).Get(podName, metaV1.GetOptions{})
		if err != nil {
			return time.Unix(0, 0), err
		}

		return pod.CreationTimestamp.Time, nil
	}
}

func PodRestartForCluster(namespace, clusterName string) func() (int, error) {
	return func() (int, error) {
		pods, err := KubeClientset.CoreV1().Pods(namespace).List(metaV1.ListOptions{LabelSelector: fmt.Sprintf("app=%s", clusterName)})
		if err != nil {
			return 0, err
		}
		var podRestartCount int
		for _, pod := range pods.Items {
			for _, containerStatus := range pod.Status.ContainerStatuses {
				podRestartCount += int(containerStatus.RestartCount)
			}
		}
		return podRestartCount, nil
	}
}

func PodReadyForCluster(namespace, clusterName string) func() (int, error) {
	return func() (int, error) {
		racks, err := KubeClientset.AppsV1beta1().StatefulSets(namespace).List(metaV1.ListOptions{
			LabelSelector: labelSelectorForCluster(namespace, clusterName),
		})
		if err != nil {
			return 0, err
		}

		podReadyCount := 0
		for _, rack := range racks.Items {
			if rack.Status.CurrentRevision == rack.Status.UpdateRevision &&
				rack.Status.ObservedGeneration != nil &&
				*rack.Status.ObservedGeneration == rack.Generation {
				podReadyCount += int(rack.Status.ReadyReplicas)
			}
		}

		return podReadyCount, nil
	}
}

func podReady(pod *coreV1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == coreV1.PodReady {
			return condition.Status == coreV1.ConditionTrue
		}
	}
	return false
}

func RacksForCluster(namespace, clusterName string) func() (map[string][]string, error) {
	return func() (map[string][]string, error) {
		pods, err := KubeClientset.CoreV1().Pods(namespace).List(metaV1.ListOptions{
			LabelSelector: labelSelectorForCluster(namespace, clusterName),
		})
		if err != nil {
			return nil, err
		}

		podsByRack := map[string][]string{}
		for _, pod := range pods.Items {
			var podList []string
			var ok bool
			rackKey := pod.Labels["cassandra-operator/rack"]
			if podList, ok = podsByRack[rackKey]; !ok {
				podList = []string{}
			}
			podList = append(podList, pod.Name)
			podsByRack[rackKey] = podList
		}
		return podsByRack, nil
	}
}

func DataCenterForCluster(namespace, clusterName string) func() (string, error) {
	return func() (string, error) {
		pods, err := KubeClientset.CoreV1().Pods(namespace).List(metaV1.ListOptions{
			LabelSelector: labelSelectorForCluster(namespace, clusterName),
		})
		if err != nil {
			return "", err
		}
		if len(pods.Items) == 0 {
			return "", fmt.Errorf("no pods found for cluster %s.%s", namespace, clusterName)
		}

		command, outputBytes, err := Kubectl(namespace, "exec", pods.Items[0].Name, "--", "sh", "-c", "nodetool status | grep \"Datacenter: \"")
		if err != nil {
			return "", fmt.Errorf("command was %v.\nOutput of exec was:\n%s\n. Error: %v", command, outputBytes, err)
		}
		output := strings.TrimSpace(string(outputBytes))
		dataCenterRegExp := regexp.MustCompile("Datacenter: (.*)")
		matches := dataCenterRegExp.FindStringSubmatch(output)
		if matches == nil {
			return "", fmt.Errorf("no match found in datacenter string")
		}
		return matches[1], nil
	}
}

func UniqueNodesUsed(namespace, clusterName string) ([]string, error) {
	pods, err := KubeClientset.CoreV1().Pods(namespace).List(metaV1.ListOptions{
		LabelSelector: labelSelectorForCluster(namespace, clusterName),
	})
	if err != nil {
		return nil, err
	}

	nodesUsed := make(map[string]string)
	for _, pod := range pods.Items {
		nodesUsed[pod.Status.HostIP] = "dont care"
	}

	keys := make([]string, 0, len(nodesUsed))
	for k := range nodesUsed {
		keys = append(keys, k)
	}

	return keys, nil
}

func FileExistsInConfigurationDirectory(namespace string, podName string, filename string) func() (bool, error) {
	return func() (bool, error) {
		command, output, err := Kubectl(namespace, "exec", podName, "ls", fmt.Sprintf("/etc/cassandra/%s", filename))
		if err != nil {
			return false, fmt.Errorf("command was %v.\nOutput of exec was:\n%s\n. Error: %v", command, output, err)
		}

		return true, nil
	}
}

func SnapshotJobsFor(clusterName string) func() (int, error) {
	return func() (int, error) {
		result, err := KubeClientset.BatchV1().Jobs(Namespace).List(metaV1.ListOptions{
			LabelSelector: labelSelectorForCluster(Namespace, clusterName),
		})
		if err != nil {
			return 0, err
		}

		return len(result.Items), nil
	}
}

func CassandraDefinitions(namespace string) ([]v1alpha1.Cassandra, error) {
	cassandras, err := CassandraClientset.CoreV1alpha1().Cassandras(namespace).List(metaV1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return cassandras.Items, nil
}

func errorUnlessNotFound(err error) error {
	if !errors.IsNotFound(err) {
		return err
	}
	return nil
}

func StatefulSetRevision(namespace, statefulSetName string) (string, error) {
	statefulSet, err := KubeClientset.AppsV1beta2().StatefulSets(namespace).Get(statefulSetName, metaV1.GetOptions{})
	if err != nil {
		return "", err
	}
	return statefulSet.Status.CurrentRevision, err
}

func StatefulSetDeletedSince(namespace, statefulSetName string, initialCreationTime time.Time) func() (bool, error) {
	getStatefulSet := func(namespace, resourceName string) (*kubernetesResource, error) {
		statefulSet, err := KubeClientset.AppsV1beta2().StatefulSets(namespace).Get(statefulSetName, metaV1.GetOptions{})
		return &kubernetesResource{statefulSet}, err
	}
	return kubernetesResourceIsDeletedSince(namespace, statefulSetName, getStatefulSet, initialCreationTime)
}

func ServiceIsDeletedSince(namespace, serviceName string, initialCreationTime time.Time) func() (bool, error) {
	getService := func(namespace, resourceName string) (*kubernetesResource, error) {
		service, err := KubeClientset.CoreV1().Services(namespace).Get(serviceName, metaV1.GetOptions{})
		return &kubernetesResource{service}, err
	}
	return kubernetesResourceIsDeletedSince(namespace, serviceName, getService, initialCreationTime)
}

func CronJobIsDeletedSince(namespace, jobName string, initialCreationTime time.Time) func() (bool, error) {
	getJob := func(namespace, resourceName string) (*kubernetesResource, error) {
		service, err := KubeClientset.BatchV1beta1().CronJobs(namespace).Get(jobName, metaV1.GetOptions{})
		return &kubernetesResource{service}, err
	}
	return kubernetesResourceIsDeletedSince(namespace, jobName, getJob, initialCreationTime)
}

func kubernetesResourceIsDeletedSince(namespace, resourceName string, getResource func(namespace, resourceName string) (*kubernetesResource, error), initialCreationTime time.Time) func() (bool, error) {
	return func() (bool, error) {
		kubernetesResource, err := getResource(namespace, resourceName)
		if errors.IsNotFound(err) {
			return true, nil
		} else if err != nil {
			return false, err
		}

		// Check creation time in case the resource was recreated in between this function invocations
		return kubernetesResource.CreationTimestamp().After(initialCreationTime), nil
	}
}

func ClusterConfigHashForRack(namespace, clusterName, rack string) string {
	statefulSet, err := KubeClientset.AppsV1beta2().StatefulSets(namespace).Get(fmt.Sprintf("%s-%s", clusterName, rack), metaV1.GetOptions{})
	gomega.Expect(err).To(gomega.BeNil())
	rackHash, ok := statefulSet.Spec.Template.Annotations["clusterConfigHash"]
	gomega.Expect(ok).To(gomega.BeTrue())
	return rackHash
}

func labelSelectorForCluster(namespace, clusterName string) string {
	return fmt.Sprintf("%s=%s,%s=%s", cluster.ApplicationInstanceLabel, fmt.Sprintf("%s.%s", namespace, clusterName), cluster.ManagedByLabel, cluster.ManagedByCassandraOperator)
}
