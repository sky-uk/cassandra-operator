package e2e

import (
	"fmt"
	"strings"
	"time"

	coreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func PrintDiagnosis(namespace string, testStartTime time.Time, clusterNames ...string) {
	var diagnosis []string
	diagnosis = append(diagnosis, "\n\t\t=== OPERATOR DIAGNOSIS ===\n")
	diagnosis = append(diagnosis, operatorDiagnosis(namespace, testStartTime))
	for _, clusterName := range clusterNames {
		diagnosis = append(diagnosis, fmt.Sprintf("\n\t\t=== CLUSTER %s DIAGNOSIS ===\n", clusterName))
		diagnosis = append(diagnosis, clusterDiagnosis(namespace, clusterName))
	}
	fmt.Printf("\n\n\t\t== DIAGNOSIS at %s ==\n\n%s", time.Now().Format(time.RFC3339), strings.Join(diagnosis, "\n"))
}

func operatorDiagnosis(namespace string, logSince time.Time) string {
	var diagnosis []string
	pods, err := KubeClientset.CoreV1().Pods(namespace).List(metaV1.ListOptions{LabelSelector: fmt.Sprintf("app=cassandra-operator")})
	if err != nil {
		return fmt.Sprintf("error while retrieving the cassandra-operator pods in namespace %s: %v", namespace, err)
	}

	if len(pods.Items) > 1 {
		return fmt.Sprintf("more than one operator was found in this namespace %s, when we expected one: %v", namespace, pods)
	}
	operatorPod := pods.Items[0]
	diagnosis = append(diagnosis, fmt.Sprintf("==== Logs for Operator pod %s since %v =====", operatorPod.Name, logSince))
	diagnosis = append(diagnosis, fmt.Sprintf("%v", podLogsSince(namespace, &operatorPod, "cassandra-operator", logSince)))
	return strings.Join(diagnosis, "\n")
}

func clusterDiagnosis(namespace, clusterName string) string {
	var diagnosis []string
	diagnosis = append(diagnosis, fmt.Sprintf("\n==== Cluster %s =====", clusterName))
	diagnosis = append(diagnosis, fmt.Sprintf("%v", clusterPodsWide(namespace, clusterName)))
	diagnosis = append(diagnosis, fmt.Sprintf("\n==== Cassandra definition ====="))
	diagnosis = append(diagnosis, fmt.Sprintf("%v", cassandraDefinition(namespace, clusterName)))
	diagnosis = append(diagnosis, fmt.Sprintf("\n==== Describing statefulsets ====="))
	diagnosis = append(diagnosis, fmt.Sprintf("%v", KubectlOutputAsString(namespace, "describe", "statefulset", "-l", fmt.Sprintf("sky.uk/cassandra-operator=%s", clusterName))))
	diagnosis = append(diagnosis, fmt.Sprintf("\n==== Describing jobs ====="))
	diagnosis = append(diagnosis, fmt.Sprintf("%v", KubectlOutputAsString(namespace, "describe", "job", "-l", fmt.Sprintf("sky.uk/cassandra-operator=%s", clusterName))))

	pods, err := KubeClientset.CoreV1().Pods(namespace).List(metaV1.ListOptions{LabelSelector: fmt.Sprintf("sky.uk/cassandra-operator=%s", clusterName)})
	if err != nil {
		return fmt.Sprintf("error while retrieving the pods list for cluster %s.%s: %v", namespace, clusterName, err)
	}
	for _, pod := range pods.Items {
		diagnosis = append(diagnosis, fmt.Sprintf("\n==== Describing pod %s =====", pod.Name))
		diagnosis = append(diagnosis, fmt.Sprintf("%v", podDescription(namespace, &pod)))
		diagnosis = append(diagnosis, fmt.Sprintf("\n==== Logs for pod %s, container cassandra ====", pod.Name))
		diagnosis = append(diagnosis, fmt.Sprintf("%v", podLogs(namespace, &pod, "cassandra")))
		diagnosis = append(diagnosis, fmt.Sprintf("\n==== Logs for pod %s, previous container cassandra ====", pod.Name))
		diagnosis = append(diagnosis, fmt.Sprintf("%v", podPreviousLogs(namespace, &pod, "cassandra")))
		diagnosis = append(diagnosis, fmt.Sprintf("\n==== Logs for pod %s, container cassandra-sidecar ====", pod.Name))
		diagnosis = append(diagnosis, fmt.Sprintf("%v", podLogs(namespace, &pod, "cassandra-sidecar")))
		diagnosis = append(diagnosis, "\n\n")
	}
	return strings.Join(diagnosis, "\n")
}

func cassandraDefinition(namespace, clusterName string) string {
	return KubectlOutputAsString(namespace, "get", "cassandra", clusterName, "-o", "yaml")
}

func clusterPodsWide(namespace, clusterName string) string {
	return KubectlOutputAsString(namespace, "get", "pod", "-o", "wide", "-l", fmt.Sprintf("app=%s", clusterName))
}

func podDescription(namespace string, pod *coreV1.Pod) string {
	return KubectlOutputAsString(namespace, "describe", "pod", pod.Name)
}

func podLogsSince(namespace string, pod *coreV1.Pod, containerName string, logSince time.Time) string {
	return KubectlOutputAsString(namespace, "logs", "--container", containerName, pod.Name, fmt.Sprintf("--since-time=%s", logSince.Format(time.RFC3339)))
}

func podLogs(namespace string, pod *coreV1.Pod, containerName string) string {
	return KubectlOutputAsString(namespace, "logs", "--container", containerName, pod.Name)
}

func podPreviousLogs(namespace string, pod *coreV1.Pod, containerName string) string {
	return KubectlOutputAsString(namespace, "logs", "-p", "--container", containerName, pod.Name)
}
