package e2e

import (
	"fmt"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"k8s.io/api/apps/v1beta2"
	"k8s.io/api/batch/v1beta1"
	coreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TheClusterIsDeleted(clusterName string) {
	deleteClusterDefinitionsWatchedByOperator(Namespace, clusterName)
	deleteCassandraCustomConfigurationConfigMap(Namespace, clusterName)
	log.Infof("Deleted cluster definition and configmap for cluster %s", clusterName)
}

func TheClusterPodSpecAreChangedTo(namespace, clusterName string, podSpec v1alpha1.Pod) {
	mutateCassandraSpec(namespace, clusterName, func(spec *v1alpha1.CassandraSpec) {
		spec.Pod.Resources = podSpec.Resources
		spec.Pod.LivenessProbe = podSpec.LivenessProbe
		spec.Pod.ReadinessProbe = podSpec.ReadinessProbe
		spec.Pod.Env = podSpec.Env
	})
	log.Infof("Updated pod spec for cluster %s", clusterName)
}

func TheClusterPodResourcesSpecAreChangedTo(namespace, clusterName string, podResources coreV1.ResourceRequirements) {
	mutateCassandraSpec(namespace, clusterName, func(spec *v1alpha1.CassandraSpec) {
		spec.Pod.Resources = podResources
	})
	log.Infof("Updated pod resources spec for cluster %s", clusterName)
}

func TheClusterPodEnvVarsAreChangedTo(namespace, clusterName string, envVars *[]v1alpha1.CassEnvVar) {
	mutateCassandraSpec(namespace, clusterName, func(spec *v1alpha1.CassandraSpec) {
		spec.Pod.Env = envVars
	})
	log.Infof("Updated pod spec for cluster %s", clusterName)
}

func TheImageImmutablePropertyIsChangedTo(namespace, clusterName, imageName string) {
	mutateCassandraSpec(namespace, clusterName, func(spec *v1alpha1.CassandraSpec) {
		spec.Pod.Image = &imageName
	})
	log.Infof("Updated pod image for cluster %s", clusterName)
}

func TheBootstrapperImageImmutablePropertyIsChangedTo(namespace, clusterName, imageName string) {
	mutateCassandraSpec(namespace, clusterName, func(spec *v1alpha1.CassandraSpec) {
		spec.Pod.BootstrapperImage = &imageName
	})
	log.Infof("Updated pod image for cluster %s", clusterName)
}

func TheRackReplicationIsChangedTo(namespace, clusterName, rackName string, replicas int) {
	mutateCassandraSpec(namespace, clusterName, func(spec *v1alpha1.CassandraSpec) {
		for i := range spec.Racks {
			if spec.Racks[i].Name == rackName {
				spec.Racks[i].Replicas = int32(replicas)
			}
		}
	})
	log.Infof("Updated rack replication for cluster %s", clusterName)
}

func TheRackStorageIsChangedTo(namespace, clusterName, rackName string, storages []v1alpha1.Storage) {
	mutateCassandraSpec(namespace, clusterName, func(spec *v1alpha1.CassandraSpec) {
		for i := range spec.Racks {
			if spec.Racks[i].Name == rackName {
				spec.Racks[i].Storage = storages
			}
		}
	})
	log.Infof("Updated rack storage for cluster %s", clusterName)
}

func TheCustomConfigIsAddedForCluster(namespace, clusterName string, extraConfigFile *ExtraConfigFile) {
	_, err := customCassandraConfigMap(namespace, clusterName, false, extraConfigFile)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	log.Infof("Added custom config for cluster %s", clusterName)
}

func TheCustomConfigIsModifiedForCluster(namespace, clusterName string, extraConfigFile *ExtraConfigFile) {
	_, err := customCassandraConfigMap(namespace, clusterName, true, extraConfigFile)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	log.Infof("Added custom config for cluster %s", clusterName)
}

func TheCustomConfigIsDeletedForCluster(namespace, clusterName string) {
	cmClient := KubeClientset.CoreV1().ConfigMaps(namespace)
	err := cmClient.Delete(fmt.Sprintf("%s-config", clusterName), metaV1.NewDeleteOptions(0))
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	log.Infof("Deleted custom config for cluster %s", clusterName)
}

func TheCustomJVMOptionsConfigIsChangedForCluster(namespace, clusterName, jvmOptions string) {
	_, err := KubeClientset.CoreV1().ConfigMaps(namespace).Patch(
		fmt.Sprintf("%s-config", clusterName),
		types.StrategicMergePatchType,
		[]byte(fmt.Sprintf("{\"data\": { \"jvm.options\": \"%s\"}}", jvmOptions)),
	)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	log.Infof("Modified custom jvm options for cluster %s", clusterName)
}

func ANewRackIsAddedForCluster(namespace, clusterName string, rack v1alpha1.Rack) {
	mutateCassandraSpec(namespace, clusterName, func(spec *v1alpha1.CassandraSpec) {
		spec.Racks = append(spec.Racks, rack)
	})
	log.Infof("Added new rack %s for cluster %s", rack.Name, clusterName)
}

func ARackIsRemovedFromCluster(namespace, clusterName, rackToRemove string) {
	var racksAfterRemoval []v1alpha1.Rack
	mutateCassandraSpec(namespace, clusterName, func(spec *v1alpha1.CassandraSpec) {
		for _, rack := range spec.Racks {
			if rack.Name != rackToRemove {
				racksAfterRemoval = append(racksAfterRemoval, rack)
			}
		}
		spec.Racks = racksAfterRemoval
	})
	log.Infof("Removed rack %s for cluster %s", rackToRemove, clusterName)
}

func AScheduledSnapshotIsAddedToCluster(namespace, clusterName string, snapshot *v1alpha1.Snapshot) {
	mutateCassandraSpec(namespace, clusterName, func(spec *v1alpha1.CassandraSpec) {
		spec.Snapshot = snapshot
	})
	log.Infof("Added scheduled snapshot for cluster %s", clusterName)
}

func AScheduledSnapshotIsRemovedFromCluster(namespace, clusterName string) {
	mutateCassandraSpec(namespace, clusterName, func(spec *v1alpha1.CassandraSpec) {
		spec.Snapshot = nil
	})
	log.Infof("Removed scheduled snapshot for cluster %s", clusterName)
}

func AScheduledSnapshotIsChangedForCluster(namespace, clusterName string, snapshot *v1alpha1.Snapshot) {
	AScheduledSnapshotIsAddedToCluster(namespace, clusterName, snapshot)
	log.Infof("Updated scheduled snapshot for cluster %s", clusterName)
}

func mutateCassandraSpec(namespace, clusterName string, mutator func(*v1alpha1.CassandraSpec)) {
	cass, err := CassandraClientset.CoreV1alpha1().Cassandras(namespace).Get(clusterName, metaV1.GetOptions{})
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	cassBeforeMutation := cass.DeepCopy()

	mutator(&cass.Spec)
	var cassAfterMutation *v1alpha1.Cassandra
	cassAfterMutation, err = CassandraClientset.CoreV1alpha1().Cassandras(namespace).Update(cass)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	log.Info(spew.Sprintf("Updated cassandra spec for cluster %s, before: %+v, \nafter: %+v", clusterName, cassBeforeMutation, cassAfterMutation))
}

func EventuallyClusterIsCreatedWithRacks(namespace string, clusterName string, racks []v1alpha1.Rack) {
	var clusterSize int
	for _, rack := range racks {
		clusterSize = clusterSize + int(rack.Replicas)
	}
	clusterBootstrapDuration := (time.Duration(clusterSize) * NodeStartDuration) + (30 * time.Second)
	gomega.Eventually(PodReadyForCluster(namespace, clusterName), clusterBootstrapDuration, CheckInterval).
		Should(gomega.Equal(clusterSize), fmt.Sprintf("Cluster %s was not created within the specified time", clusterName))
}

func CassandraEventsFor(namespace, clusterName string) func() ([]coreV1.Event, error) {
	allEvents := func(coreV1.Event) bool { return true }
	return cassandraEventsFilteredFor(namespace, clusterName, allEvents)
}

func CassandraEventsSince(namespace, clusterName string, sinceTime time.Time) func() ([]coreV1.Event, error) {
	eventsOnOrAfterTime := func(event coreV1.Event) bool {
		metaSinceTime := metaV1.NewTime(sinceTime)
		return event.LastTimestamp.Equal(&metaSinceTime) || event.LastTimestamp.After(sinceTime)
	}
	return cassandraEventsFilteredFor(namespace, clusterName, eventsOnOrAfterTime)
}

func cassandraEventsFilteredFor(namespace, clusterName string, filter func(coreV1.Event) bool) func() ([]coreV1.Event, error) {
	return func() ([]coreV1.Event, error) {
		var cassandraEvents []coreV1.Event
		allEvents, err := KubeClientset.CoreV1().Events(namespace).List(metaV1.ListOptions{})
		if err != nil {
			return nil, err
		}

		for _, event := range allEvents.Items {
			if event.InvolvedObject.Kind == cassandra.Kind && event.InvolvedObject.Name == clusterName && filter(event) {
				cassandraEvents = append(cassandraEvents, event)
			}
		}
		return cassandraEvents, nil
	}
}

func DurationSeconds(seconds int32) time.Duration {
	return time.Duration(seconds) * time.Second
}

func TheStatefulSetIsDeletedForRack(namespace, clusterName, rackName string) {
	statefulSetName := fmt.Sprintf("%s-%s", clusterName, rackName)

	statefulSet, err := KubeClientset.AppsV1beta2().StatefulSets(namespace).Get(statefulSetName, metaV1.GetOptions{})
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	cascadingDelete := metaV1.DeletePropagationForeground
	err = KubeClientset.AppsV1beta1().StatefulSets(namespace).Delete(
		statefulSetName,
		&metaV1.DeleteOptions{PropagationPolicy: &cascadingDelete},
	)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	gomega.Eventually(StatefulSetDeletedSince(namespace, statefulSetName, statefulSet.CreationTimestamp.Time), NodeTerminationDuration, CheckInterval).Should(gomega.BeTrue())
	log.Infof("StatefulSet %s has been deleted since it was initially created at %s", statefulSetName, statefulSet.CreationTimestamp.Time)
}

func TheStatefulSetContainerImageNameIsChanged(namespace, statefulSetName string, containerName string, newImageName string) {
	mutateStatefulSet(namespace, statefulSetName, func(statefulSet *v1beta2.StatefulSet) {

		for i, container := range statefulSet.Spec.Template.Spec.Containers {
			if container.Name == containerName {
				statefulSet.Spec.Template.Spec.Containers[i].Image = newImageName
			}
		}
	})
	log.Infof("Updated imageName for statefulSet %s, container %s, new image name %s", statefulSetName, containerName, newImageName)
}

func TheStatefulSetContainerEnvVarsHaveChanged(namespace, statefulSetName string, containerName string, newEnvVars []coreV1.EnvVar) {
	mutateStatefulSet(namespace, statefulSetName, func(statefulSet *v1beta2.StatefulSet) {

		for i, container := range statefulSet.Spec.Template.Spec.Containers {
			if container.Name == containerName {
				statefulSet.Spec.Template.Spec.Containers[i].Env = newEnvVars
			}
		}
	})
	log.Infof("Updated envVars for statefulSet %s, container %s", statefulSetName, containerName)
}

func TheStatefulSetContainerResourcesChanged(namespace, statefulSetName string, containerName string, editedResources coreV1.ResourceRequirements) {
	mutateStatefulSet(namespace, statefulSetName, func(statefulSet *v1beta2.StatefulSet) {

		for i, container := range statefulSet.Spec.Template.Spec.Containers {
			if container.Name == containerName {
				statefulSet.Spec.Template.Spec.Containers[i].Resources = editedResources
			}
		}
	})
	log.Infof("Updated resources for statefulSet %s, container %s, new resources %s", statefulSetName, containerName, &editedResources)
}

func mutateStatefulSet(namespace, statefulSetName string, mutator func(*v1beta2.StatefulSet)) {
	statefulSet, getErr := KubeClientset.AppsV1beta2().StatefulSets(namespace).Get(statefulSetName, metaV1.GetOptions{})
	gomega.Expect(getErr).ToNot(gomega.HaveOccurred())
	statefulSetBeforeMutation := statefulSet.DeepCopy()

	mutator(statefulSet)

	var statefulSetAfterMutation *v1beta2.StatefulSet
	statefulSetAfterMutation, err := KubeClientset.AppsV1beta2().StatefulSets(namespace).Update(statefulSet)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	log.Info(spew.Sprintf("Updated statefulset spec %s, before: %+v, \nafter: %+v", statefulSetName, statefulSetBeforeMutation, statefulSetAfterMutation))
}

func TheServiceIsDeletedFor(namespace, clusterName string) {
	service, err := KubeClientset.CoreV1().Services(namespace).Get(clusterName, metaV1.GetOptions{})
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	err = KubeClientset.CoreV1().Services(namespace).Delete(clusterName, metaV1.NewDeleteOptions(0))
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	gomega.Eventually(ServiceIsDeletedSince(namespace, clusterName, service.CreationTimestamp.Time), 30*time.Second, CheckInterval).Should(gomega.BeTrue())
	log.Infof("Service %s has been deleted since it was initially created at %s", clusterName, service.CreationTimestamp)
}

func TheSnapshotCronJobIsDeleted(namespace, clusterName string) {
	TheCronJobIsDeleted(namespace, fmt.Sprintf("%s-snapshot", clusterName))
}

func TheSnapshotCleanupCronJobIsDeleted(namespace, clusterName string) {
	TheCronJobIsDeleted(namespace, fmt.Sprintf("%s-snapshot-cleanup", clusterName))
}

func TheCronJobIsDeleted(namespace, jobName string) {
	job, err := KubeClientset.BatchV1beta1().CronJobs(namespace).Get(jobName, metaV1.GetOptions{})
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	err = KubeClientset.BatchV1beta1().CronJobs(namespace).Delete(jobName, metaV1.NewDeleteOptions(0))
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	gomega.Eventually(CronJobIsDeletedSince(namespace, jobName, job.CreationTimestamp.Time), 30*time.Second, CheckInterval).Should(gomega.BeTrue())
	log.Infof("Cronjob %s has been deleted since it was initially created at %s", jobName, job.CreationTimestamp)
}

func TheCronjobResourcesAreChangedTo(namespace, jobName string, editedResources coreV1.ResourceRequirements) {
	mutateCronjobSpec(namespace, jobName, func(cronJob *v1beta1.CronJob) {
		cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Resources = editedResources
	})
	log.Infof("Updated resources for cronjob %s", jobName)
}

func mutateCronjobSpec(namespace, jobName string, mutator func(*v1beta1.CronJob)) {
	cronJob, getErr := KubeClientset.BatchV1beta1().CronJobs(namespace).Get(jobName, metaV1.GetOptions{})
	gomega.Expect(getErr).ToNot(gomega.HaveOccurred())
	cronBeforeMutation := cronJob.DeepCopy()

	mutator(cronJob)

	var cronAfterMutation *v1beta1.CronJob
	cronAfterMutation, err := KubeClientset.BatchV1beta1().CronJobs(namespace).Update(cronJob)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	log.Info(spew.Sprintf("Updated cronjob spec for job %s, before: %+v, \nafter: %+v", jobName, cronBeforeMutation, cronAfterMutation))
}

func TheCustomConfigHashIsChangedForRack(namespace, clusterName, rackName string) {
	statefulSet, err := KubeClientset.AppsV1beta2().StatefulSets(namespace).Get(fmt.Sprintf("%s-%s", clusterName, rackName), metaV1.GetOptions{})
	gomega.Expect(err).To(gomega.BeNil())
	statefulSet.Spec.Template.Annotations["clusterConfigHash"] = "something else"
	_, err = KubeClientset.AppsV1beta2().StatefulSets(namespace).Update(statefulSet)
	gomega.Expect(err).To(gomega.BeNil())
	log.Infof("Custom config has changed for rack %s", rackName)
}
