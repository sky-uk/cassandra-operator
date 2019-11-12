package modification

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/ptr"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test/apis"
	. "github.com/sky-uk/cassandra-operator/cassandra-operator/test/e2e"
	"k8s.io/api/apps/v1beta2"
	"k8s.io/api/core/v1"
	"time"
)

var _ = Context("interrupted operations", func() {
	var (
		clusterName         string
		testStartTime       time.Time
		withCustomConfig    = true
		withoutCustomConfig = false
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

	Context("cluster definition changes", func() {
		It("should abandon a stuck operation when the cluster definition is updated", func() {
			// given
			aStuckClusterExists(clusterName, withoutCustomConfig)

			// when
			bootstrapperImageName := "bootstrapper-image"
			TheBootstrapperImageImmutablePropertyIsChangedTo(Namespace, clusterName, bootstrapperImageName)

			// then
			Eventually(statefulSetBootstrapperImage(Namespace, clusterName, "cassandra-bootstrapper"), NodeStartDuration, time.Second).Should(Each(Equal(bootstrapperImageName)))
		})

		It("should abandon a stuck operation when the cluster definition is deleted", func() {
			// given
			aStuckClusterExists(clusterName, withoutCustomConfig)

			// when
			TheClusterIsDeleted(clusterName)

			// then
			Eventually(StatefulSetsForCluster(Namespace, clusterName), NodeTerminationDuration, time.Second).Should(BeEmpty())
		})
	})

	Context("custom config changes", func() {
		It("should abandon a stuck operation when custom config is added to a cluster", func() {
			// given
			aStuckClusterExists(clusterName, withoutCustomConfig)
			Expect(StatefulSetForRack(Namespace, clusterName, "a")).ToNot(HaveVolumeForConfigMap(fmt.Sprintf("%s-config", clusterName)))

			// when
			TheCustomConfigIsAddedForCluster(Namespace, clusterName, nil)

			// then
			Eventually(statefulSetGeneration(Namespace, clusterName), NodeStartDuration, time.Second).Should(Each(Equal(int64(2))))
			Expect(StatefulSetForRack(Namespace, clusterName, "a")).To(HaveVolumeForConfigMap(fmt.Sprintf("%s-config", clusterName)))
		})

		It("should abandon a stuck operation when custom config is removed from a cluster", func() {
			// given
			aStuckClusterExists(clusterName, withCustomConfig)
			Expect(StatefulSetForRack(Namespace, clusterName, "a")).To(HaveVolumeForConfigMap(fmt.Sprintf("%s-config", clusterName)))

			// when
			TheCustomConfigIsDeletedForCluster(Namespace, clusterName)

			// then
			Eventually(statefulSetGeneration(Namespace, clusterName), NodeStartDuration, time.Second).Should(Each(Equal(int64(2))))
			Expect(StatefulSetForRack(Namespace, clusterName, "a")).ToNot(HaveVolumeForConfigMap(fmt.Sprintf("%s-config", clusterName)))
		})

		It("should abandon a stuck operation when custom config is modified for a cluster", func() {
			// given
			aStuckClusterExists(clusterName, withCustomConfig)
			Expect(StatefulSetForRack(Namespace, clusterName, "a")).To(HaveVolumeForConfigMap(fmt.Sprintf("%s-config", clusterName)))
			originalClusterConfigHash := ClusterConfigHashForRack(Namespace, clusterName, "a")

			// when
			TheCustomConfigIsModifiedForCluster(Namespace, clusterName, &ExtraConfigFile{Name: "blah.txt", Content: "test"})

			// then
			Eventually(statefulSetGeneration(Namespace, clusterName), NodeStartDuration, time.Second).Should(Each(Equal(int64(2))))
			Expect(ClusterConfigHashForRack(Namespace, clusterName, "a")).ToNot(Equal(originalClusterConfigHash))
		})
	})
})

func aStuckClusterExists(clusterName string, withCustomConfig bool) {
	registerResourcesUsed(1)
	racks := []v1alpha1.Rack{RackWithEmptyDir("a", 1)}
	podSpec := apis.APod().WithDefaults().
		WithBootstrapperImageName(ptr.String("cassandra:invalid")).
		WithSidecarImageName(CassandraSidecarImageName).Build()

	cluster := AClusterWithName(clusterName).AndRacks(racks).AndPodSpec(podSpec)
	if withCustomConfig {
		cluster = cluster.AndCustomConfig(nil)
	} else {
		cluster = cluster.WithoutCustomConfig()
	}
	cluster.IsDefined()

	Eventually(initContainerState(Namespace, clusterName, fmt.Sprintf("%s-a-0", clusterName), "cassandra-bootstrapper"), 60*time.Second, 2*time.Second).
		Should(Equal(v1.ContainerState{Waiting: &v1.ContainerStateWaiting{
			Reason:  "ErrImagePull",
			Message: "rpc error: code = Unknown desc = failed to resolve image \"docker.io/library/cassandra:invalid\": no available registry endpoint: docker.io/library/cassandra:invalid not found",
		}}))
}

func initContainerState(namespace, clusterName, podName, initContainerName string) func() (v1.ContainerState, error) {
	return func() (v1.ContainerState, error) {
		pods, err := PodsForCluster(namespace, clusterName)()
		if err != nil {
			return v1.ContainerState{}, err
		}

		for _, pod := range pods {
			if pod.Resource.(v1.Pod).Name == podName {
				for _, initContainerStatus := range pod.Resource.(v1.Pod).Status.InitContainerStatuses {
					if initContainerStatus.Name == initContainerName {
						return initContainerStatus.State, nil
					}
				}
			}
		}

		return v1.ContainerState{}, fmt.Errorf("initContainer %s in pod %s not found", initContainerName, podName)
	}
}

func statefulSetBootstrapperImage(namespace, clusterName, initContainerName string) func() ([]string, error) {
	return func() ([]string, error) {
		sets, err := StatefulSetsForCluster(namespace, clusterName)()
		if err != nil {
			return nil, err
		}

		var images []string
		for _, sts := range sets {
			for _, initContainer := range sts.Resource.(v1beta2.StatefulSet).Spec.Template.Spec.InitContainers {
				if initContainer.Name == initContainerName {
					images = append(images, initContainer.Image)
				}
			}
		}

		return images, nil
	}
}

func statefulSetGeneration(namespace, clusterName string) func() ([]int64, error) {
	return func() ([]int64, error) {
		sets, err := StatefulSetsForCluster(namespace, clusterName)()
		if err != nil {
			return nil, err
		}

		var generations []int64
		for _, sts := range sets {
			generations = append(generations, sts.Resource.(v1beta2.StatefulSet).ObjectMeta.Generation)
		}

		return generations, nil
	}
}
