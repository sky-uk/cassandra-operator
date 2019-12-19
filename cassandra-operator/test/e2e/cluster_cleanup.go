package e2e

import (
	"fmt"
	log "github.com/sirupsen/logrus"

	"time"

	"github.com/onsi/gomega"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/cluster"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc" // required for connectivity into dev cluster
)

func DeleteCassandraResourcesInNamespace(namespace string) {
	cassandras, err := CassandraDefinitions(namespace)
	if err != nil {
		log.Infof("Error while searching for cassandras in namespace %s: %v", namespace, err)
	}

	// delete cluster definitions first, to stop the operator from trying to reconcile the state
	for _, cassandra := range cassandras {
		deleteClusterDefinitionsWatchedByOperator(namespace, cassandra.Name)
	}
	deleteCassandraResourcesWithLabel(namespace, fmt.Sprintf("%s=%s", cluster.ManagedByLabel, cluster.ManagedByCassandraOperator))
	gomega.Eventually(cassandraPodsInNamespace(namespace), 240*time.Second, time.Second).Should(gomega.BeZero())
}

func DeleteCassandraResourcesForClusters(namespace string, clusterNames ...string) {
	for _, clusterName := range clusterNames {
		deleteCassandraResourcesForCluster(namespace, clusterName)
	}
}

func deleteCassandraCustomConfigurationConfigMap(namespace, clusterName string) {
	deleteConfigMapsWithLabel(namespace, labelSelectorForCluster(namespace, clusterName))
}

func deleteCassandraResourcesForCluster(namespace, clusterName string) {
	// delete cluster definitions first, to stop the operator from trying to reconcile the state
	// and bring back the delete resources
	deleteClusterDefinitionsWatchedByOperator(namespace, clusterName)
	deleteCassandraResourcesWithLabel(namespace, labelSelectorForCluster(namespace, clusterName))
}

func deleteCassandraResourcesWithLabel(namespace, labelSelector string) {
	deleteStatefulSetsWithLabel(namespace, labelSelector)
	deletePodsWithLabels(namespace, labelSelector)
	deletePvcsWithLabel(namespace, labelSelector)
	deleteServicesWithLabel(namespace, labelSelector)
	deleteConfigMapsWithLabel(namespace, labelSelector)
	deletePodsWithLabel(namespace, labelSelector)
	deleteJobsWithLabel(namespace, labelSelector)
}

func deleteClusterDefinitionsWatchedByOperator(namespace, clusterName string) {
	log.Infof("Deleting cassandra definition in namespace %s: %v", namespace, clusterName)
	// orphans or foreground policies result in an additional unneeded cluster update event, instead of just the delete cluster event
	propagationPolicy := metaV1.DeletePropagationBackground
	if err := CassandraClientset.CoreV1alpha1().Cassandras(namespace).Delete(clusterName, &metaV1.DeleteOptions{PropagationPolicy: &propagationPolicy}); err != nil {
		log.Infof("Error while deleting cassandra resources in namespace %s: %v", namespace, err)
	}
}

func deleteStatefulSetsWithLabel(namespace, labelSelector string) {
	ssClient := KubeClientset.AppsV1beta2().StatefulSets(namespace)
	statefulSetList, err := ssClient.List(metaV1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		log.Infof("Error while searching for stateful set in namespace %s, with labelSelector %s: %v", namespace, labelSelector, err)
	}
	var ssToDelete []string
	for _, ss := range statefulSetList.Items {
		ssToDelete = append(ssToDelete, ss.Name)
	}

	log.Infof("Deleting statefulsets in namespace %s, with labelSelector %s: %v", namespace, labelSelector, ssToDelete)
	deleteImmediately := int64(0)
	cascadingDelete := metaV1.DeletePropagationBackground
	if err := ssClient.DeleteCollection(
		&metaV1.DeleteOptions{PropagationPolicy: &cascadingDelete, GracePeriodSeconds: &deleteImmediately},
		metaV1.ListOptions{LabelSelector: labelSelector}); err != nil {
		log.Infof("Unable to delete stateful set with cascading in namespace %s, with labelSelector %s: %v", namespace, labelSelector, err)
	}
	gomega.Eventually(statefulSetsWithLabel(namespace, labelSelector), durationSecondsPerItem(ssToDelete, 60), time.Second).
		Should(gomega.HaveLen(0), fmt.Sprintf("When deleting statefulsets: %v", ssToDelete))
}

func deletePodsWithLabels(namespace, labelSelector string) {
	client := KubeClientset.CoreV1().Pods(namespace)
	podList, err := client.List(metaV1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		log.Infof("Error while searching for stateful set in namespace %s, with labelSelector %s: %v", namespace, labelSelector, err)
	}

	deleteImmediately := int64(0)
	if err := client.DeleteCollection(
		&metaV1.DeleteOptions{GracePeriodSeconds: &deleteImmediately},
		metaV1.ListOptions{LabelSelector: labelSelector}); err != nil {
		log.Infof("Unable to delete pods in namespace %s, with labelSelector %s: %v", namespace, labelSelector, err)
	}
	gomega.Eventually(podsWithLabel(namespace, labelSelector), time.Duration(len(podList.Items)*60)*time.Second, time.Second).
		Should(gomega.HaveLen(0), fmt.Sprintf("When deleting pods: %v", podList.Items))
}

func deletePvcsWithLabel(namespace, labelSelector string) {
	pvcClient := KubeClientset.CoreV1().PersistentVolumeClaims(namespace)
	persistentVolumeClaimList, err := pvcClient.List(metaV1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		log.Infof("Error while searching for pvc in namespace %s, with labelSelector %s: %v", namespace, labelSelector, err)
	}
	var pvcToDelete []string
	for _, pvc := range persistentVolumeClaimList.Items {
		pvcToDelete = append(pvcToDelete, pvc.Name)
	}

	log.Infof("Deleting pvcs in namespace %s, with labelSelector %s: %v", namespace, labelSelector, pvcToDelete)
	if err := pvcClient.DeleteCollection(metaV1.NewDeleteOptions(0),
		metaV1.ListOptions{LabelSelector: labelSelector}); err != nil {
		log.Infof("Unable to delete persistent volume claims in namespace %s, with labelSelector %s: %v", namespace, labelSelector, err)
	}
	gomega.Eventually(persistentVolumeClaimsWithLabel(namespace, labelSelector), durationSecondsPerItem(pvcToDelete, 30), time.Second).
		Should(gomega.HaveLen(0), fmt.Sprintf("When deleting pvc: %v", pvcToDelete))
}

func deleteServicesWithLabel(namespace, labelSelector string) {
	svcClient := KubeClientset.CoreV1().Services(namespace)
	list, err := svcClient.List(metaV1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		log.Infof("Error while searching for services in namespace %s, with labelSelector %s: %v", namespace, labelSelector, err)
	}
	var svcToDelete []string
	for _, svc := range list.Items {
		svcToDelete = append(svcToDelete, svc.Name)
	}

	log.Infof("Deleting services in namespace %s, with labelSelector %s: %v", namespace, labelSelector, svcToDelete)
	for _, svc := range svcToDelete {
		if err := svcClient.Delete(svc, metaV1.NewDeleteOptions(0)); err != nil {
			log.Infof("Unable to delete service %s, in namespace %s: %v", svc, namespace, err)
		}
	}
}

func deleteConfigMapsWithLabel(namespace, labelSelector string) {
	configMapClient := KubeClientset.CoreV1().ConfigMaps(namespace)
	list, err := configMapClient.List(metaV1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		log.Infof("Error while searching for configMaps in namespace %s, with labelSelector %s: %v", namespace, labelSelector, err)
	}
	var configMapToDelete []string
	for _, svc := range list.Items {
		configMapToDelete = append(configMapToDelete, svc.Name)
	}

	log.Infof("Deleting configMaps in namespace %s, with labelSelector %s: %v", namespace, labelSelector, configMapToDelete)
	for _, configMap := range configMapToDelete {
		if err := configMapClient.Delete(configMap, metaV1.NewDeleteOptions(0)); err != nil {
			log.Infof("Unable to delete configMap %s, in namespace %s: %v", configMap, namespace, err)
		}
	}
}

func deletePodsWithLabel(namespace, labelSelector string) {
	podClient := KubeClientset.CoreV1().Pods(namespace)
	err := podClient.DeleteCollection(&metaV1.DeleteOptions{}, metaV1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		log.Infof("Unable to delete pods labelled %s in namespace %s: %v", labelSelector, namespace, err)
	}
}

func deleteJobsWithLabel(namespace, labelSelector string) {
	deleteInForeground := metaV1.DeletePropagationForeground
	jobClient := KubeClientset.BatchV1beta1().CronJobs(namespace)
	err := jobClient.DeleteCollection(&metaV1.DeleteOptions{PropagationPolicy: &deleteInForeground}, metaV1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		log.Infof("Unable to delete jobs labelled %s in namespace %s: %v", labelSelector, namespace, err)
	}
}

func cassandraPodsInNamespace(namespace string) func() int {
	return func() int {
		podClient := KubeClientset.CoreV1().Pods(namespace)
		pods, err := podClient.List(metaV1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", cluster.ManagedByLabel, cluster.ManagedByCassandraOperator)})
		if err == nil {
			return 0
		}

		return len(pods.Items)
	}
}

func durationSecondsPerItem(items []string, durationPerItem int) time.Duration {
	return time.Duration(len(items)*durationPerItem) * time.Second
}
