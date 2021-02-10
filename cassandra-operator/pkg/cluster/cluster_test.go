package cluster

import (
	"fmt"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test/apis"
	"reflect"
	"strconv"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	v1alpha1helpers "github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1/helpers"
	appsv1 "k8s.io/api/apps/v1beta2"
	"k8s.io/api/batch/v1beta1"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/util/ptr"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/test"
)

const (
	CLUSTER   = "mycluster"
	NAMESPACE = "mynamespace"
)

func TestCluster(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Cluster Suite", test.CreateParallelReporters("cluster"))
}

var _ = Describe("identification of custom config maps", func() {
	It("should look like a custom configmap when it ending with the correct suffix", func() {
		configMap := coreV1.ConfigMap{ObjectMeta: metaV1.ObjectMeta{Name: "cluster1-config"}}
		Expect(LooksLikeACassandraConfigMap(&configMap)).To(BeTrue())
	})

	It("should not look like a custom configmap when not ending with the correct suffix", func() {
		configMap := coreV1.ConfigMap{ObjectMeta: metaV1.ObjectMeta{Name: "cluster1-config-more"}}
		Expect(LooksLikeACassandraConfigMap(&configMap)).To(BeFalse())
	})

	It("should identify when a custom config map is not related to any cluster", func() {
		clusters := map[string]*Cluster{"cluster1": {definition: &v1alpha1.Cassandra{ObjectMeta: metaV1.ObjectMeta{Name: "cluster1"}}}}
		configMap := coreV1.ConfigMap{ObjectMeta: metaV1.ObjectMeta{Name: "cluster1-config-for-something-else"}}

		Expect(ConfigMapBelongsToAManagedCluster(clusters, &configMap)).To(BeFalse())
	})
})

var _ = Describe("creation of stateful sets", func() {
	var clusterDef *v1alpha1.Cassandra
	var configMap = &coreV1.ConfigMap{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      "mycluster-config",
			Namespace: NAMESPACE,
		},
		Data: map[string]string{
			"test": "value",
		},
	}
	BeforeEach(func() {
		clusterDef = apis.ACassandra().WithDefaults().WithName(CLUSTER).WithNamespace(NAMESPACE).Build()
	})

	Describe("Init Containers", func() {
		It("should be added for config initialisation and bootstrapping", func() {
			// given
			clusterDef.Spec.Pod.BootstrapperImage = ptr.String("skyuk/cassandra-bootstrapper:latest")
			cluster := ACluster(clusterDef)

			// when
			statefulSet := cluster.CreateStatefulSetForRack(&clusterDef.Spec.Racks[0], nil)

			// then
			Expect(statefulSet.Spec.Template.Spec.InitContainers).To(HaveLen(2))

			Expect(statefulSet.Spec.Template.Spec.InitContainers[0].Name).To(Equal("init-config"))
			Expect(statefulSet.Spec.Template.Spec.InitContainers[0].Image).To(Equal(*cluster.definition.Spec.Pod.Image))
			Expect(statefulSet.Spec.Template.Spec.InitContainers[0].Command).To(Equal([]string{"sh", "-c", "cp -vr /etc/cassandra/* /configuration"}))

			Expect(statefulSet.Spec.Template.Spec.InitContainers[1].Name).To(Equal("cassandra-bootstrapper"))
			Expect(statefulSet.Spec.Template.Spec.InitContainers[1].Image).To(ContainSubstring("skyuk/cassandra-bootstrapper:latest"))
		})

		It("should be created with the specified image if one is given", func() {
			clusterDef.Spec.Pod.BootstrapperImage = ptr.String("somerepo/abootstapperimage:v1")
			cluster := ACluster(clusterDef)

			statefulSet := cluster.CreateStatefulSetForRack(&clusterDef.Spec.Racks[0], nil)
			Expect(statefulSet.Spec.Template.Spec.InitContainers[1].Name).To(Equal("cassandra-bootstrapper"))
			Expect(statefulSet.Spec.Template.Spec.InitContainers[1].Image).To(Equal("somerepo/abootstapperimage:v1"))
		})

		It("should define environment variables for pod memory and cpu in bootstrapper init-container", func() {
			clusterDef.Spec.Pod.Resources.Requests[coreV1.ResourceMemory] = resource.MustParse("1Gi")
			clusterDef.Spec.Pod.Resources.Requests[coreV1.ResourceCPU] = resource.MustParse("100m")
			cluster := ACluster(clusterDef)

			statefulSet := cluster.CreateStatefulSetForRack(&clusterDef.Spec.Racks[0], nil)
			Expect(statefulSet.Spec.Template.Spec.InitContainers).To(HaveLen(2))
			Expect(statefulSet.Spec.Template.Spec.InitContainers[1].Env).To(ContainElement(coreV1.EnvVar{Name: "POD_CPU_MILLICORES", Value: "100"}))
			Expect(statefulSet.Spec.Template.Spec.InitContainers[1].Env).To(ContainElement(coreV1.EnvVar{Name: "POD_MEMORY_BYTES", Value: strconv.Itoa(1024 * 1024 * 1024)}))
		})

		It("should be assigned no cpu", func() {
			cluster := ACluster(clusterDef)

			statefulSet := cluster.CreateStatefulSetForRack(&clusterDef.Spec.Racks[0], nil)
			Expect(statefulSet.Spec.Template.Spec.InitContainers).To(HaveLen(2))
			Expect(statefulSet.Spec.Template.Spec.InitContainers[0].Resources.Requests[coreV1.ResourceCPU]).To(Equal(resource.Quantity{}))
			Expect(statefulSet.Spec.Template.Spec.InitContainers[0].Resources.Limits[coreV1.ResourceCPU]).To(Equal(resource.Quantity{}))
			Expect(statefulSet.Spec.Template.Spec.InitContainers[1].Resources.Requests[coreV1.ResourceCPU]).To(Equal(resource.Quantity{}))
			Expect(statefulSet.Spec.Template.Spec.InitContainers[1].Resources.Limits[coreV1.ResourceCPU]).To(Equal(resource.Quantity{}))
		})

		It("should be assigned minimum memory resources", func() {
			clusterDef.Spec.Pod.Resources.Requests[coreV1.ResourceMemory] = resource.MustParse("1Gi")

			cluster := ACluster(clusterDef)
			statefulSet := cluster.CreateStatefulSetForRack(&clusterDef.Spec.Racks[0], nil)
			Expect(statefulSet.Spec.Template.Spec.InitContainers).To(HaveLen(2))
			Expect(statefulSet.Spec.Template.Spec.InitContainers[0].Resources.Requests.Memory().String()).To(Equal("100Mi"))
			Expect(statefulSet.Spec.Template.Spec.InitContainers[0].Resources.Limits.Memory().String()).To(Equal("100Mi"))
			Expect(statefulSet.Spec.Template.Spec.InitContainers[1].Resources.Requests.Memory().String()).To(Equal("100Mi"))
			Expect(statefulSet.Spec.Template.Spec.InitContainers[1].Resources.Limits.Memory().String()).To(Equal("100Mi"))
		})

		It("should be assigned memory resources no greater than the pod is configured with", func() {
			clusterDef.Spec.Pod.Resources.Requests[coreV1.ResourceMemory] = resource.MustParse("10Mi")

			cluster := ACluster(clusterDef)
			statefulSet := cluster.CreateStatefulSetForRack(&clusterDef.Spec.Racks[0], nil)
			Expect(statefulSet.Spec.Template.Spec.InitContainers).To(HaveLen(2))
			Expect(statefulSet.Spec.Template.Spec.InitContainers[0].Resources.Requests.Memory().String()).To(Equal("10Mi"))
			Expect(statefulSet.Spec.Template.Spec.InitContainers[0].Resources.Limits.Memory().String()).To(Equal("10Mi"))
			Expect(statefulSet.Spec.Template.Spec.InitContainers[1].Resources.Requests.Memory().String()).To(Equal("10Mi"))
			Expect(statefulSet.Spec.Template.Spec.InitContainers[1].Resources.Limits.Memory().String()).To(Equal("10Mi"))
		})
	})

	Describe("Labelling", func() {
		It("should have the set of recommended labels defined on the statefulset itself", func() {
			// given
			cluster := ACluster(clusterDef)

			// when
			statefulSet := cluster.CreateStatefulSetForRack(&cluster.Racks()[0], nil)
			labels := statefulSet.ObjectMeta.Labels

			// then
			Expect(labels).To(HaveKeyWithValue("app.kubernetes.io/name", cluster.Name()))
			Expect(labels).To(HaveKeyWithValue("app.kubernetes.io/instance", cluster.QualifiedName()))
			Expect(labels).To(HaveKeyWithValue("app.kubernetes.io/version", "3.11"))
			Expect(labels).To(HaveKeyWithValue("app.kubernetes.io/managed-by", "cassandra-operator"))
		})

		It("should have the set of recommended labels defined on the pod template", func() {
			// given
			cluster := ACluster(clusterDef)

			// when
			statefulSet := cluster.CreateStatefulSetForRack(&cluster.Racks()[0], nil)
			podLabels := statefulSet.Spec.Template.ObjectMeta.Labels

			// then
			Expect(podLabels).To(HaveKeyWithValue("app.kubernetes.io/name", cluster.Name()))
			Expect(podLabels).To(HaveKeyWithValue("app.kubernetes.io/instance", cluster.QualifiedName()))
			Expect(podLabels).To(HaveKeyWithValue("app.kubernetes.io/version", "3.11"))
			Expect(podLabels).To(HaveKeyWithValue("app.kubernetes.io/managed-by", "cassandra-operator"))
		})

		It("should use the managed-by, instance and rack labels in its selector", func() {
			// given
			cluster := ACluster(clusterDef)

			// when
			statefulSet := cluster.CreateStatefulSetForRack(&cluster.Racks()[0], nil)
			matchLabels := statefulSet.Spec.Selector.MatchLabels

			// then
			Expect(matchLabels).To(HaveKeyWithValue("app.kubernetes.io/instance", cluster.QualifiedName()))
			Expect(matchLabels).To(HaveKeyWithValue("cassandra-operator/rack", cluster.Racks()[0].Name))
			Expect(matchLabels).To(HaveKeyWithValue("app.kubernetes.io/managed-by", "cassandra-operator"))
		})
	})

	Describe("Cassandra PreStop Lifecycle Hook", func() {
		It("should be defined with the correct nodetool argument environment variable", func() {
			// given
			cluster := ACluster(clusterDef)

			// when
			statefulSet := cluster.CreateStatefulSetForRack(&cluster.Racks()[0], nil)

			// then
			execCommand := statefulSet.Spec.Template.Spec.Containers[0].Lifecycle.PreStop.Exec.Command
			Expect(execCommand).To(HaveLen(3))
			Expect(execCommand).To(Equal([]string{"/bin/sh", "-c", "nodetool ${NODETOOL_ARGS} drain"}))
		})
	})

	Describe("Storage", func() {

		It("should define emptyDir volumes for configuration and extra libraries", func() {
			// given
			cluster := ACluster(clusterDef)

			// when
			statefulSet := cluster.CreateStatefulSetForRack(&cluster.Racks()[0], nil)

			// then
			volumes := statefulSet.Spec.Template.Spec.Volumes
			Expect(volumes).To(HaveLen(2))
			Expect(volumes).To(haveExactly(1, matchingEmptyDir("configuration")))
			Expect(volumes).To(haveExactly(1, matchingEmptyDir("extra-lib")))
		})

		It("should mount the configuration and extra-lib volumes in the main container", func() {
			// given
			cluster := ACluster(clusterDef)

			// when
			statefulSet := cluster.CreateStatefulSetForRack(&cluster.Racks()[0], configMap)

			// then
			mainContainerVolumeMounts := statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts
			Expect(mainContainerVolumeMounts).To(HaveLen(3))
			Expect(mainContainerVolumeMounts).To(haveExactly(1, matchingVolumeMount("configuration", "/etc/cassandra")))
			Expect(mainContainerVolumeMounts).To(haveExactly(1, matchingVolumeMount("extra-lib", "/extra-lib")))
		})

		Context("using persistent volume as storage", func() {

			It("should create a persistent volume claim", func() {
				// given
				storageIndex := 0
				cluster := ACluster(clusterDef)

				// when
				statefulSet := cluster.CreateStatefulSetForRack(&cluster.Racks()[0], nil)

				// then
				volumeClaims := statefulSet.Spec.VolumeClaimTemplates
				Expect(volumeClaims).To(HaveLen(1))
				Expect(volumeClaims[0].Name).To(Equal("var-lib-cassandra"))
				Expect(&volumeClaims[0].Spec).To(Equal(clusterDef.Spec.Racks[0].Storage[storageIndex].PersistentVolumeClaim))
			})

			It("should mount a persistent volume claim at the default path", func() {
				// given
				cluster := ACluster(clusterDef)

				// when
				statefulSet := cluster.CreateStatefulSetForRack(&cluster.Racks()[0], nil)

				// then
				mainContainerVolumeMounts := statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts
				Expect(mainContainerVolumeMounts).To(HaveLen(3))
				Expect(mainContainerVolumeMounts).To(haveExactly(1, matchingVolumeMount("var-lib-cassandra", "/var/lib/cassandra")))
			})

			It("should mount a persistent volume claim at the given path", func() {
				// given
				storageIndex := 0
				clusterDef.Spec.Racks[0].Storage[storageIndex].Path = ptr.String("/my-cassandra-home")
				cluster := ACluster(clusterDef)

				// when
				statefulSet := cluster.CreateStatefulSetForRack(&cluster.Racks()[0], nil)

				// then
				mainContainerVolumeMounts := statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts
				Expect(mainContainerVolumeMounts).To(HaveLen(3))
				Expect(mainContainerVolumeMounts).To(haveExactly(1, matchingVolumeMount("my-cassandra-home", "/my-cassandra-home")))
			})

			It("should attach the correct labels to the persistent volume claim", func() {
				// given
				cluster := ACluster(clusterDef)

				// when
				statefulSet := cluster.CreateStatefulSetForRack(&cluster.Racks()[0], nil)

				// then
				volumeClaims := statefulSet.Spec.VolumeClaimTemplates
				Expect(volumeClaims).To(HaveLen(1))
				Expect(volumeClaims[0].Name).To(Equal("var-lib-cassandra"))
				Expect(volumeClaims[0].Labels).To(And(
					HaveKeyWithValue(ApplicationNameLabel, cluster.Name()),
					HaveKeyWithValue(ApplicationInstanceLabel, cluster.QualifiedName()),
					HaveKeyWithValue(ManagedByLabel, ManagedByCassandraOperator),
					HaveKeyWithValue("cassandra-operator/rack", "a"),
				))
			})
		})

		Context("using emptyDir as storage", func() {
			BeforeEach(func() {
				clusterDef = apis.ACassandra().
					WithDefaults().
					WithSpec(apis.ACassandraSpec().
						WithDefaults().
						WithRacks(
							apis.ARack("a", 1).WithDefaults().WithStorages(apis.AnEmptyDir()),
							apis.ARack("b", 1).WithDefaults().WithStorages(apis.AnEmptyDir()),
						)).
					Build()
			})

			It("should create an emptyDir volume", func() {
				// given
				cluster := ACluster(clusterDef)

				// when
				statefulSet := cluster.CreateStatefulSetForRack(&cluster.Racks()[0], nil)

				// then
				volumes := statefulSet.Spec.Template.Spec.Volumes
				Expect(volumes).To(HaveLen(3))
				Expect(volumes).To(haveExactly(1, matchingEmptyDir("var-lib-cassandra")))
			})

			It("should mount an emptyDir at the default path", func() {
				// given
				cluster := ACluster(clusterDef)

				// when
				statefulSet := cluster.CreateStatefulSetForRack(&cluster.Racks()[0], nil)

				// then
				mainContainerVolumeMounts := statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts
				Expect(mainContainerVolumeMounts).To(HaveLen(3))
				Expect(mainContainerVolumeMounts).To(haveExactly(1, matchingVolumeMount("var-lib-cassandra", "/var/lib/cassandra")))
			})

			It("should mount an emptyDir at the given path", func() {
				// given
				storageIndex := 0
				clusterDef.Spec.Racks[0].Storage[storageIndex].Path = ptr.String("/my-other-cassandra-home")
				cluster := ACluster(clusterDef)

				// when
				statefulSet := cluster.CreateStatefulSetForRack(&cluster.Racks()[0], nil)

				// then
				mainContainerVolumeMounts := statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts
				Expect(mainContainerVolumeMounts).To(HaveLen(3))
				Expect(mainContainerVolumeMounts).To(haveExactly(1, matchingVolumeMount("my-other-cassandra-home", "/my-other-cassandra-home")))
			})
		})

		Context("when multiple storages are defined", func() {
			BeforeEach(func() {
				clusterDef = apis.ACassandra().WithDefaults().WithSpec(
					apis.ACassandraSpec().WithDefaults().WithRacks(
						apis.ARack("a", 1).WithDefaults().WithStorages(
							apis.AnEmptyDir().AtPath("/emptydir-path-1"),
							apis.APersistentVolume().AtPath("/pv-cassandra").OfSize("100Gi").WithStorageClass("fast"),
							apis.APersistentVolume().AtPath("/pv-logs").OfSize("1Gi").WithStorageClass("standard"),
							apis.AnEmptyDir().AtPath("/emptydir-path-2"),
						),
					)).Build()
			})

			It("should create 1 volume for each emptyDir", func() {
				// given
				cluster := ACluster(clusterDef)

				// when
				statefulSet := cluster.CreateStatefulSetForRack(&cluster.Racks()[0], nil)

				// then
				volumes := statefulSet.Spec.Template.Spec.Volumes
				Expect(volumes).To(HaveLen(4))
				Expect(volumes).To(haveExactly(1, matchingEmptyDir("emptydir-path-1")))
				Expect(volumes).To(haveExactly(1, matchingEmptyDir("emptydir-path-2")))
			})

			It("should create 1 volume claim for each persistent volume", func() {
				// given
				cluster := ACluster(clusterDef)

				// when
				statefulSet := cluster.CreateStatefulSetForRack(&cluster.Racks()[0], nil)

				// then
				volumeClaims := statefulSet.Spec.VolumeClaimTemplates
				Expect(volumeClaims).To(HaveLen(2))
				Expect(volumeClaims[0].Name).To(Equal("pv-cassandra"))
				Expect(volumeClaims[1].Name).To(Equal("pv-logs"))
				Expect(&volumeClaims[0].Spec).To(Equal(clusterDef.Spec.Racks[0].Storage[1].PersistentVolumeClaim))
				Expect(&volumeClaims[1].Spec).To(Equal(clusterDef.Spec.Racks[0].Storage[2].PersistentVolumeClaim))
			})

			It("should mount all volumes at their given path", func() {
				// given
				cluster := ACluster(clusterDef)

				// when
				statefulSet := cluster.CreateStatefulSetForRack(&cluster.Racks()[0], nil)

				// then
				mainContainerVolumeMounts := statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts
				Expect(mainContainerVolumeMounts).To(HaveLen(6))
				Expect(mainContainerVolumeMounts).To(haveExactly(1, matchingVolumeMount("emptydir-path-1", "/emptydir-path-1")))
				Expect(mainContainerVolumeMounts).To(haveExactly(1, matchingVolumeMount("pv-cassandra", "/pv-cassandra")))
				Expect(mainContainerVolumeMounts).To(haveExactly(1, matchingVolumeMount("pv-logs", "/pv-logs")))
				Expect(mainContainerVolumeMounts).To(haveExactly(1, matchingVolumeMount("emptydir-path-2", "/emptydir-path-2")))
			})
		})
	})

	Describe("Affinity rules", func() {
		It("should define zone specific affinity rules when zone is provided", func() {
			// given
			cluster := ACluster(clusterDef)

			// when
			statefulSet := cluster.CreateStatefulSetForRack(&cluster.Racks()[0], configMap)

			// then
			nodeAffinity := statefulSet.Spec.Template.Spec.Affinity.NodeAffinity
			Expect(nodeAffinity).NotTo(BeNil())
			nodeSelectorTerms := nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
			Expect(nodeSelectorTerms).To(HaveLen(1))
			matchExpr := nodeSelectorTerms[0].MatchExpressions[0]
			Expect(matchExpr.Key).To(Equal("failure-domain.beta.kubernetes.io/zone"))
			Expect(matchExpr.Values).To(ConsistOf([]string{cluster.Racks()[0].Zone}))
		})

		It("should define a pod anti-affinity rule preventing scheduling with another pod in the same cluster", func() {
			// given
			cluster := ACluster(clusterDef)

			// when
			statefulSet := cluster.CreateStatefulSetForRack(&cluster.Racks()[0], nil)
			podAntiAffinity := statefulSet.Spec.Template.Spec.Affinity.PodAntiAffinity

			// then
			Expect(podAntiAffinity).ToNot(BeNil())
			Expect(podAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution).To(HaveLen(1))

			antiAffinityLabels := podAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].LabelSelector.MatchLabels
			Expect(antiAffinityLabels).To(HaveKeyWithValue("app.kubernetes.io/managed-by", "cassandra-operator"))
			Expect(antiAffinityLabels).To(HaveKeyWithValue("app.kubernetes.io/instance", cluster.QualifiedName()))
		})
	})

	Describe("custom ConfigMap", func() {
		Context("a cluster with a custom configMap is created", func() {
			It("should mount the configuration volume in the init-config container", func() {
				// given
				cluster := ACluster(clusterDef)

				// when
				statefulSet := cluster.CreateStatefulSetForRack(&cluster.Racks()[0], configMap)

				// then
				initConfigContainerVolumeMounts := statefulSet.Spec.Template.Spec.InitContainers[0].VolumeMounts
				Expect(initConfigContainerVolumeMounts).To(HaveLen(1))
				Expect(initConfigContainerVolumeMounts).To(haveExactly(1, matchingVolumeMount("configuration", "/configuration")))
			})

			It("should mount the configMap, configuration and extra-lib volumes in the bootstrap container", func() {
				// given
				cluster := ACluster(clusterDef)

				// when
				statefulSet := cluster.CreateStatefulSetForRack(&cluster.Racks()[0], configMap)

				// then
				volumes := statefulSet.Spec.Template.Spec.Volumes
				Expect(volumes).To(HaveLen(3))
				Expect(volumes).To(haveExactly(1, matchingConfigMap("cassandra-custom-config-mycluster", "mycluster-config")))

				bootstrapContainerVolumeMounts := statefulSet.Spec.Template.Spec.InitContainers[1].VolumeMounts
				Expect(bootstrapContainerVolumeMounts).To(HaveLen(3))
				Expect(bootstrapContainerVolumeMounts).To(haveExactly(1, matchingVolumeMount("configuration", "/configuration")))
				Expect(bootstrapContainerVolumeMounts).To(haveExactly(1, matchingVolumeMount("extra-lib", "/extra-lib")))
				Expect(bootstrapContainerVolumeMounts).To(haveExactly(1, matchingVolumeMount("cassandra-custom-config-mycluster", "/custom-config")))
			})
		})

		Context("a cluster without a custom configMap is created", func() {
			It("should not create the volume configMap and its corresponding mount in the bootstrap container", func() {
				// given
				cluster := ACluster(clusterDef)

				// when
				statefulSet := cluster.CreateStatefulSetForRack(&cluster.Racks()[0], nil)

				Expect(statefulSet.Spec.Template.Spec.Volumes).To(HaveLen(2))

				bootstrapContainerVolumeMounts := statefulSet.Spec.Template.Spec.InitContainers[1].VolumeMounts
				Expect(bootstrapContainerVolumeMounts).To(HaveLen(2))
				Expect(bootstrapContainerVolumeMounts).To(haveExactly(0, matchingVolumeMount("cassandra-custom-config-mycluster", "/custom-config")))
				Expect(bootstrapContainerVolumeMounts).To(haveExactly(1, matchingVolumeMount("configuration", "/configuration")))
				Expect(bootstrapContainerVolumeMounts).To(haveExactly(1, matchingVolumeMount("extra-lib", "/extra-lib")))
			})
		})
	})

	Describe("sidecar container", func() {
		It("configure the environment variables required by the cassandra-sidecar server", func() {
			// given
			cluster := ACluster(clusterDef)
			statefulSet := cluster.CreateStatefulSetForRack(&cluster.Racks()[0], nil)

			// when
			sidecarContainer := statefulSet.Spec.Template.Spec.Containers[1]

			// then
			Expect(sidecarContainer.Name).To(Equal("cassandra-sidecar"))
			Expect(sidecarContainer.Env).To(And(
				ContainElement(coreV1.EnvVar{Name: "NODE_LISTEN_ADDRESS", ValueFrom: &coreV1.EnvVarSource{
					FieldRef: &coreV1.ObjectFieldSelector{
						FieldPath: "status.podIP",
					},
				}}),
				ContainElement(coreV1.EnvVar{Name: "CLUSTER_NAME", Value: cluster.Name()}),
				ContainElement(coreV1.EnvVar{Name: "CLUSTER_NAMESPACE", Value: cluster.Namespace()}),
			))
		})

		It("should have specified memory and cpu resources", func() {
			clusterDef.Spec.Pod.Sidecar.Resources = coreV1.ResourceRequirements{
				Limits: coreV1.ResourceList{
					coreV1.ResourceMemory: resource.MustParse("68Mi"),
					coreV1.ResourceCPU:    resource.MustParse("185m"),
				},
				Requests: coreV1.ResourceList{
					coreV1.ResourceMemory: resource.MustParse("68Mi"),
				},
			}
			cluster := ACluster(clusterDef)
			statefulSet := cluster.CreateStatefulSetForRack(&cluster.Racks()[0], nil)

			// when
			sidecarContainer := statefulSet.Spec.Template.Spec.Containers[1]

			Expect(sidecarContainer.Resources).To(Equal(coreV1.ResourceRequirements{
				Limits: coreV1.ResourceList{
					coreV1.ResourceMemory: resource.MustParse("68Mi"),
					coreV1.ResourceCPU:    resource.MustParse("185m"),
				},
				Requests: coreV1.ResourceList{
					coreV1.ResourceMemory: resource.MustParse("68Mi"),
				},
			}))
		})

		It("should allow CPU bursting configurations", func() {
			// given
			clusterDef.Spec.Pod.Sidecar.Resources.Requests[coreV1.ResourceCPU] = resource.MustParse("0")
			cluster := ACluster(clusterDef)
			statefulSet := cluster.CreateStatefulSetForRack(&cluster.Racks()[0], nil)

			// when
			sidecarContainer := statefulSet.Spec.Template.Spec.Containers[1]

			// then
			Expect(sidecarContainer.Name).To(Equal("cassandra-sidecar"))
			Expect(*sidecarContainer.Resources.Requests.Cpu()).To(Equal(resource.MustParse("0")))
		})
	})

	Describe("security context", func() {
		It("should set runAsUser, runAsGroup and fsGroup to the correct values on the pod spec", func() {
			// when
			statefulSets := createStatefulSetsFor(clusterDef)

			// then
			Expect(statefulSets.Items).To(HaveLen(2))
			for _, statefulSet := range statefulSets.Items {
				securityContext := statefulSet.Spec.Template.Spec.SecurityContext
				Expect(*securityContext.RunAsUser).To(Equal(UserID))
				Expect(*securityContext.FSGroup).To(Equal(GroupID))
				Expect(*securityContext.RunAsGroup).To(Equal(GroupID))
			}
		})
	})

	Describe("environment variables", func() {
		It("should define environment variable for extra classpath in main container without being specified and defaultEnv var in builder", func() {
			cluster := ACluster(clusterDef)

			statefulSet := cluster.CreateStatefulSetForRack(&clusterDef.Spec.Racks[0], nil)
			Expect(statefulSet.Spec.Template.Spec.Containers[0].Env).To(ContainElement(coreV1.EnvVar{Name: "EXTRA_CLASSPATH", Value: "/extra-lib/cassandra-seed-provider.jar"}))
			Expect(statefulSet.Spec.Template.Spec.Containers[0].Env).To(ContainElement(coreV1.EnvVar{Name: "defaultEnvName", Value: "defaultEnvValue"}))
			Expect(statefulSet.Spec.Template.Spec.Containers[0].Env).To(ContainElement(coreV1.EnvVar{Name: "NODETOOL_ARGS", Value: "-DnodetoolArg=testArg"}))
			Expect(statefulSet.Spec.Template.Spec.Containers[0].Env).To(HaveLen(3))
		})

		It("should define environment variable for extra classpath in main container and nothing else when pod.env is empty", func() {
			clusterDef.Spec.Pod.Env = &[]v1alpha1.CassEnvVar{}
			cluster := ACluster(clusterDef)

			statefulSet := cluster.CreateStatefulSetForRack(&clusterDef.Spec.Racks[0], nil)
			Expect(statefulSet.Spec.Template.Spec.Containers[0].Env).To(ContainElement(coreV1.EnvVar{Name: "EXTRA_CLASSPATH", Value: "/extra-lib/cassandra-seed-provider.jar"}))
			Expect(statefulSet.Spec.Template.Spec.Containers[0].Env).To(HaveLen(1))
		})

		It("should define environment variable for supplied values and extra classpath", func() {
			clusterDef.Spec.Pod.Env = &[]v1alpha1.CassEnvVar{{Name: "SomeVal", Value: "SomeOtherVal"}}
			cluster := ACluster(clusterDef)

			statefulSet := cluster.CreateStatefulSetForRack(&clusterDef.Spec.Racks[0], nil)
			Expect(statefulSet.Spec.Template.Spec.Containers[0].Env).To(ContainElement(coreV1.EnvVar{Name: "EXTRA_CLASSPATH", Value: "/extra-lib/cassandra-seed-provider.jar"}))
			Expect(statefulSet.Spec.Template.Spec.Containers[0].Env).To(ContainElement(coreV1.EnvVar{Name: "SomeVal", Value: "SomeOtherVal"}))
			Expect(statefulSet.Spec.Template.Spec.Containers[0].Env).To(HaveLen(2))
		})

		It("should define environment variable for supplied secret and extra classpath", func() {
			secretKeyRef := coreV1.SecretKeySelector{Key: "namespace-secret"}
			clusterDef.Spec.Pod.Env = &[]v1alpha1.CassEnvVar{
				{Name: "SomeVal", ValueFrom: &v1alpha1.CassEnvVarSource{SecretKeyRef: secretKeyRef}},
			}
			cluster := ACluster(clusterDef)

			statefulSet := cluster.CreateStatefulSetForRack(&clusterDef.Spec.Racks[0], nil)
			Expect(statefulSet.Spec.Template.Spec.Containers[0].Env).To(ContainElement(coreV1.EnvVar{Name: "EXTRA_CLASSPATH", Value: "/extra-lib/cassandra-seed-provider.jar"}))
			Expect(statefulSet.Spec.Template.Spec.Containers[0].Env).To(ContainElement(coreV1.EnvVar{Name: "SomeVal", ValueFrom: &coreV1.EnvVarSource{SecretKeyRef: &secretKeyRef}}))
			Expect(statefulSet.Spec.Template.Spec.Containers[0].Env).To(HaveLen(2))
		})
	})
})

var _ = Describe("headless service", func() {
	It("should have a selector which selects based on labels attached to the pods", func() {
		// given
		clusterDef := apis.ACassandra().WithDefaults().WithName(CLUSTER).WithNamespace(NAMESPACE).Build()
		cluster := ACluster(clusterDef)

		// when
		service := cluster.CreateService()

		// then
		Expect(service.Spec.Selector).To(And(
			HaveKeyWithValue(ApplicationInstanceLabel, cluster.QualifiedName()),
			HaveKeyWithValue(ManagedByLabel, ManagedByCassandraOperator),
		))
	})
})

var _ = Describe("modification of stateful sets", func() {
	var clusterDef *v1alpha1.Cassandra
	var configMap = &coreV1.ConfigMap{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      "mycluster-config",
			Namespace: NAMESPACE,
		},
		Data: map[string]string{
			"test": "value",
		},
	}
	BeforeEach(func() {
		clusterDef = apis.ACassandra().WithDefaults().WithName(CLUSTER).WithNamespace(NAMESPACE).Build()
	})

	Context("the statefulset is updated to the desired state", func() {
		var (
			unmodifiedStatefulSet *appsv1.StatefulSet
			currentStatefulSet    *appsv1.StatefulSet
		)

		BeforeEach(func() {
			unmodifiedStatefulSet = &appsv1.StatefulSet{
				TypeMeta:   metaV1.TypeMeta{Kind: "statefulset", APIVersion: "v1beta2"},
				ObjectMeta: metaV1.ObjectMeta{Name: "current-statefulset", Namespace: NAMESPACE},
				Spec: appsv1.StatefulSetSpec{
					Replicas: ptr.Int32(1),
				},
				Status: appsv1.StatefulSetStatus{
					Replicas: 1,
					Conditions: []appsv1.StatefulSetCondition{
						{
							Status:  coreV1.ConditionTrue,
							Message: "this is ready",
						},
					},
				},
			}
			currentStatefulSet = unmodifiedStatefulSet.DeepCopy()
		})

		Specify("its typemeta, metadata and status should be left unchanged", func() {
			// given
			cluster := ACluster(clusterDef)
			rackToUpdate := cluster.Racks()[0]

			// when
			cluster.UpdateStatefulSetToDesiredState(currentStatefulSet, &rackToUpdate, configMap)

			// then
			Expect(currentStatefulSet.TypeMeta).To(Equal(unmodifiedStatefulSet.TypeMeta))
			Expect(currentStatefulSet.ObjectMeta).To(Equal(unmodifiedStatefulSet.ObjectMeta))
			Expect(currentStatefulSet.Status).To(Equal(unmodifiedStatefulSet.Status))
		})

		Specify("its spec should be updated to match the target rack definition", func() {
			// given
			cluster := ACluster(clusterDef)
			rackToUpdate := cluster.Racks()[0]

			// when
			cluster.UpdateStatefulSetToDesiredState(currentStatefulSet, &rackToUpdate, configMap)

			// then
			expectedStatefulSet := cluster.CreateStatefulSetForRack(&rackToUpdate, configMap)
			Expect(currentStatefulSet.Spec).To(Equal(expectedStatefulSet.Spec))
		})

	})

	Context("the custom configMap is added", func() {
		It("should add the configMap volume and its corresponding mount to the cassandra-bootstrapper init-container", func() {
			// given
			cluster := ACluster(clusterDef)
			statefulSet := cluster.CreateStatefulSetForRack(&cluster.Racks()[0], nil)

			// when
			cluster.AddCustomConfigVolumeToStatefulSet(statefulSet, nil, configMap)

			// then
			Expect(statefulSet.Spec.Template.Spec.Volumes).To(HaveLen(3))
			Expect(statefulSet.Spec.Template.Spec.Volumes).To(haveExactly(1, matchingConfigMap("cassandra-custom-config-mycluster", "mycluster-config")))

			Expect(statefulSet.Spec.Template.Spec.InitContainers[1].VolumeMounts).To(HaveLen(3))
			Expect(statefulSet.Spec.Template.Spec.InitContainers[1].VolumeMounts).To(haveExactly(1, matchingVolumeMount("cassandra-custom-config-mycluster", "/custom-config")))
		})

		It("should add a config map hash annotation to the pod spec", func() {
			// given
			cluster := ACluster(clusterDef)
			statefulSet := cluster.CreateStatefulSetForRack(&cluster.Racks()[0], nil)

			// when
			cluster.AddCustomConfigVolumeToStatefulSet(statefulSet, nil, configMap)

			// then
			Expect(statefulSet.Spec.Template.Annotations[ConfigHashAnnotation]).ToNot(BeEmpty())
		})

		It("should do nothing when the configMap volume already exists", func() {
			// given
			cluster := ACluster(clusterDef)
			statefulSet := cluster.CreateStatefulSetForRack(&cluster.Racks()[0], configMap)

			// when
			cluster.AddCustomConfigVolumeToStatefulSet(statefulSet, nil, configMap)

			// then
			Expect(statefulSet.Spec.Template.Spec.Volumes).To(HaveLen(3))
			Expect(statefulSet.Spec.Template.Spec.Volumes).To(haveExactly(1, matchingConfigMap("cassandra-custom-config-mycluster", "mycluster-config")))

			Expect(statefulSet.Spec.Template.Spec.InitContainers[1].VolumeMounts).To(HaveLen(3))
			Expect(statefulSet.Spec.Template.Spec.InitContainers[1].VolumeMounts).To(haveExactly(1, matchingVolumeMount("cassandra-custom-config-mycluster", "/custom-config")))

		})
	})

	Context("the custom configMap is removed", func() {
		It("should remove the configMap volume and its corresponding mount in the cassandra-bootstrapper init-container", func() {
			// given
			cluster := ACluster(clusterDef)
			statefulSet := cluster.CreateStatefulSetForRack(&cluster.Racks()[0], configMap)

			// when
			cluster.RemoveCustomConfigVolumeFromStatefulSet(statefulSet, nil, nil)

			// then
			Expect(statefulSet.Spec.Template.Spec.Volumes).To(HaveLen(2))
			Expect(statefulSet.Spec.Template.Spec.InitContainers[1].VolumeMounts).To(HaveLen(2))
			Expect(statefulSet.Spec.Template.Spec.InitContainers[1].VolumeMounts).To(haveExactly(1, matchingVolumeMount("configuration", "/configuration")))
			Expect(statefulSet.Spec.Template.Spec.InitContainers[1].VolumeMounts).To(haveExactly(1, matchingVolumeMount("extra-lib", "/extra-lib")))
		})

		It("should remove the config map hash annotation from the pod spec", func() {
			// given
			cluster := ACluster(clusterDef)
			statefulSet := cluster.CreateStatefulSetForRack(&cluster.Racks()[0], configMap)

			// when
			cluster.RemoveCustomConfigVolumeFromStatefulSet(statefulSet, nil, nil)

			// then
			Expect(statefulSet.Spec.Template.Annotations[ConfigHashAnnotation]).To(BeEmpty())
		})
	})
})

var _ = Describe("creation of snapshot job", func() {
	var (
		clusterDef      *v1alpha1.Cassandra
		snapshotTimeout = int32(10)
	)

	BeforeEach(func() {
		clusterDef = apis.ACassandra().
			WithDefaults().
			WithName(CLUSTER).
			WithNamespace(NAMESPACE).
			WithSpec(apis.ACassandraSpec().WithDefaults().WithSnapshot(
				apis.ASnapshot().
					WithDefaults().
					WithSchedule("01 23 * * *").
					WithTimeoutSeconds(snapshotTimeout))).
			Build()

	})

	It("should create a cronjob named after the cluster that will trigger at the specified schedule", func() {
		cluster := ACluster(clusterDef)

		cronJob := cluster.CreateSnapshotJob()
		Expect(cronJob.Name).To(Equal(fmt.Sprintf("%s-snapshot", clusterDef.Name)))
		Expect(cronJob.Namespace).To(Equal(clusterDef.Namespace))
		Expect(cronJob.Labels).To(And(
			HaveKeyWithValue(ApplicationInstanceLabel, clusterDef.QualifiedName()),
			HaveKeyWithValue(ApplicationComponentLabel, "snapshot"),
			HaveKeyWithValue(ManagedByLabel, ManagedByCassandraOperator),
		))
		Expect(cronJob.Spec.Schedule).To(Equal("01 23 * * *"))
		Expect(cronJob.Spec.ConcurrencyPolicy).To(Equal(v1beta1.ForbidConcurrent))
	})

	It("should create a cronjob with its associated job named after the cluster in the same namespace", func() {
		cluster := ACluster(clusterDef)

		cronJob := cluster.CreateSnapshotJob()
		backupJob := cronJob.Spec.JobTemplate
		Expect(backupJob.Name).To(Equal(fmt.Sprintf("%s-snapshot", clusterDef.Name)))
		Expect(backupJob.Namespace).To(Equal(clusterDef.Namespace))
		Expect(backupJob.Labels).To(And(
			HaveKeyWithValue(ApplicationInstanceLabel, clusterDef.QualifiedName()),
			HaveKeyWithValue(ApplicationComponentLabel, "snapshot"),
			HaveKeyWithValue(ManagedByLabel, ManagedByCassandraOperator),
		))

		backupPod := cronJob.Spec.JobTemplate.Spec.Template
		Expect(backupPod.Name).To(Equal(fmt.Sprintf("%s-snapshot", clusterDef.Name)))
		Expect(backupPod.Namespace).To(Equal(clusterDef.Namespace))
		Expect(backupPod.Labels).To(And(
			HaveKeyWithValue(ApplicationInstanceLabel, clusterDef.QualifiedName()),
			HaveKeyWithValue(ApplicationComponentLabel, "snapshot"),
			HaveKeyWithValue(ManagedByLabel, ManagedByCassandraOperator),
		))
	})

	It("should create a cronjob that will trigger a snapshot creation for the whole cluster when no keyspace specified", func() {
		clusterDef.Spec.Snapshot.Keyspaces = nil
		clusterDef.Spec.Snapshot.TimeoutSeconds = &snapshotTimeout
		cluster := ACluster(clusterDef)

		cronJob := cluster.CreateSnapshotJob()
		Expect(cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers).To(HaveLen(1))

		snapshotContainer := cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
		Expect(snapshotContainer.Name).To(Equal(fmt.Sprintf("%s-snapshot", clusterDef.Name)))
		Expect(snapshotContainer.Command).To(Equal([]string{
			"/cassandra-snapshot", "create",
			"-n", cluster.Namespace(),
			"-l", cluster.CassandraPodSelector(),
			"-t", durationSeconds(&snapshotTimeout).String(),
		}))
		Expect(snapshotContainer.Image).To(ContainSubstring("skyuk/cassandra-snapshot:latest"))
	})

	It("should create a cronjob that will trigger a snapshot creation for the specified keyspaces", func() {
		clusterDef.Spec.Snapshot.Keyspaces = []string{"keyspace1", "keyspace50"}
		clusterDef.Spec.Snapshot.TimeoutSeconds = &snapshotTimeout
		cluster := ACluster(clusterDef)

		cronJob := cluster.CreateSnapshotJob()
		Expect(cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers).To(HaveLen(1))

		snapshotContainer := cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
		Expect(snapshotContainer.Name).To(Equal(fmt.Sprintf("%s-snapshot", clusterDef.Name)))
		Expect(snapshotContainer.Command).To(Equal([]string{
			"/cassandra-snapshot", "create",
			"-n", cluster.Namespace(),
			"-l", cluster.CassandraPodSelector(),
			"-t", durationSeconds(&snapshotTimeout).String(),
			"-k", "keyspace1,keyspace50",
		}))
		Expect(snapshotContainer.Image).To(ContainSubstring("skyuk/cassandra-snapshot:latest"))
	})

	It("should create a cronjob which pod will restart in case of failure", func() {
		cluster := ACluster(clusterDef)

		cronJob := cluster.CreateSnapshotJob()

		snapshotPod := cronJob.Spec.JobTemplate.Spec.Template.Spec
		Expect(snapshotPod.RestartPolicy).To(Equal(coreV1.RestartPolicyOnFailure))
	})

	It("should not create a snapshot job if none is specified in the cluster spec", func() {
		clusterDef.Spec.Snapshot = nil
		cluster := ACluster(clusterDef)

		cronJob := cluster.CreateSnapshotJob()

		Expect(cronJob).To(BeNil())
	})

	It("should create a cronjob which pod is using the specified snapshot image", func() {
		img := "somerepo/snapshot:v1"
		clusterDef.Spec.Snapshot.Image = &img
		cluster := ACluster(clusterDef)

		cronJob := cluster.CreateSnapshotJob()

		snapshotContainer := cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
		Expect(snapshotContainer.Image).To(ContainSubstring("somerepo/snapshot:v1"))
	})

	It("should set runAsUser, runAsGroup and fsGroup to the correct values on the pod spec", func() {
		// given
		cluster := ACluster(clusterDef)

		// when
		cronJob := cluster.CreateSnapshotJob()

		// then
		securityContext := cronJob.Spec.JobTemplate.Spec.Template.Spec.SecurityContext
		Expect(*securityContext.RunAsUser).To(Equal(UserID))
		Expect(*securityContext.FSGroup).To(Equal(GroupID))
		Expect(*securityContext.RunAsGroup).To(Equal(GroupID))
	})
	It("should create a cronjob with the specified memory and cpu resources", func() {
		clusterDef.Spec.Snapshot.Resources = coreV1.ResourceRequirements{
			Limits: coreV1.ResourceList{
				coreV1.ResourceMemory: resource.MustParse("66Mi"),
				coreV1.ResourceCPU:    resource.MustParse("175m"),
			},
			Requests: coreV1.ResourceList{
				coreV1.ResourceMemory: resource.MustParse("66Mi"),
			},
		}
		cluster := ACluster(clusterDef)

		cronJob := cluster.CreateSnapshotJob()
		Expect(cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers).To(HaveLen(1))

		snapshotContainer := cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
		Expect(snapshotContainer.Resources).To(Equal(coreV1.ResourceRequirements{
			Limits: coreV1.ResourceList{
				coreV1.ResourceMemory: resource.MustParse("66Mi"),
				coreV1.ResourceCPU:    resource.MustParse("175m"),
			},
			Requests: coreV1.ResourceList{
				coreV1.ResourceMemory: resource.MustParse("66Mi"),
			},
		}))
	})
})

var _ = Describe("creation of snapshot cleanup job", func() {
	var (
		clusterDef      *v1alpha1.Cassandra
		cleanupTimeout  = int32(5)
		retentionPeriod = int32(1)
	)

	BeforeEach(func() {
		clusterDef = apis.ACassandra().WithDefaults().WithSpec(
			apis.ACassandraSpec().
				WithDefaults().
				WithSnapshot(
					apis.ASnapshot().
						WithDefaults().
						WithRetentionPolicy(
							apis.ARetentionPolicy().
								WithDefaults().
								WithRetentionPeriodDays(retentionPeriod).
								WithTimeoutSeconds(cleanupTimeout).
								WithCleanupScheduled("0 9 * * *")))).
			Build()
	})

	It("should not create a cleanup job if no retention policy is specified in the cluster spec", func() {
		clusterDef.Spec.Snapshot.RetentionPolicy = nil
		cluster := ACluster(clusterDef)

		cronJob := cluster.CreateSnapshotCleanupJob()

		Expect(cronJob).To(BeNil())
	})

	It("should create a cronjob named after the cluster that will trigger at the specified schedule", func() {
		cluster := ACluster(clusterDef)

		cronJob := cluster.CreateSnapshotCleanupJob()
		Expect(cronJob.Name).To(Equal(fmt.Sprintf("%s-snapshot-cleanup", clusterDef.Name)))
		Expect(cronJob.Namespace).To(Equal(clusterDef.Namespace))
		Expect(cronJob.Labels).To(And(
			HaveKeyWithValue(ApplicationNameLabel, clusterDef.Name),
			HaveKeyWithValue(ApplicationInstanceLabel, clusterDef.QualifiedName()),
			HaveKeyWithValue(ApplicationComponentLabel, "snapshot-cleanup"),
			HaveKeyWithValue(ManagedByLabel, ManagedByCassandraOperator),
		))
		Expect(cronJob.Spec.Schedule).To(Equal("0 9 * * *"))
	})

	It("should create a cronjob with its associated job named after the cluster in the same namespace", func() {
		cluster := ACluster(clusterDef)

		cronJob := cluster.CreateSnapshotCleanupJob()
		cleanupJob := cronJob.Spec.JobTemplate
		Expect(cleanupJob.Name).To(Equal(fmt.Sprintf("%s-snapshot-cleanup", clusterDef.Name)))
		Expect(cleanupJob.Namespace).To(Equal(clusterDef.Namespace))
		Expect(cleanupJob.Labels).To(And(
			HaveKeyWithValue(ApplicationNameLabel, clusterDef.Name),
			HaveKeyWithValue(ApplicationInstanceLabel, clusterDef.QualifiedName()),
			HaveKeyWithValue(ApplicationComponentLabel, "snapshot-cleanup"),
			HaveKeyWithValue(ManagedByLabel, ManagedByCassandraOperator),
		))

		cleanupPod := cronJob.Spec.JobTemplate.Spec.Template
		Expect(cleanupPod.Name).To(Equal(fmt.Sprintf("%s-snapshot-cleanup", clusterDef.Name)))
		Expect(cleanupPod.Namespace).To(Equal(clusterDef.Namespace))
		Expect(cleanupPod.Labels).To(And(
			HaveKeyWithValue(ApplicationNameLabel, clusterDef.Name),
			HaveKeyWithValue(ApplicationInstanceLabel, clusterDef.QualifiedName()),
			HaveKeyWithValue(ApplicationComponentLabel, "snapshot-cleanup"),
			HaveKeyWithValue(ManagedByLabel, ManagedByCassandraOperator),
		))
	})

	It("should create a cronjob that will trigger a snapshot cleanup", func() {
		cluster := ACluster(clusterDef)

		cronJob := cluster.CreateSnapshotCleanupJob()
		Expect(cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers).To(HaveLen(1))

		cleanupContainer := cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
		Expect(cleanupContainer.Name).To(Equal(fmt.Sprintf("%s-snapshot-cleanup", clusterDef.Name)))
		Expect(cleanupContainer.Command).To(Equal([]string{
			"/cassandra-snapshot", "cleanup",
			"-n", cluster.Namespace(),
			"-l", cluster.CassandraPodSelector(),
			"-r", durationDays(&retentionPeriod).String(),
			"-t", durationSeconds(&cleanupTimeout).String(),
		}))
		Expect(cleanupContainer.Image).To(ContainSubstring("skyuk/cassandra-snapshot:latest"))
	})

	It("should create a cronjob which pod is using the specified snapshot image", func() {
		img := "somerepo/snapshot:v1"
		clusterDef.Spec.Snapshot.Image = &img
		cluster := ACluster(clusterDef)

		cronJob := cluster.CreateSnapshotCleanupJob()

		cleanupContainer := cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
		Expect(cleanupContainer.Image).To(ContainSubstring("somerepo/snapshot:v1"))
	})

	It("should set runAsUser, runAsGroup and fsGroup to the correct values on the pod spec", func() {
		// given
		cluster := ACluster(clusterDef)

		// when
		cronJob := cluster.CreateSnapshotCleanupJob()

		// then
		securityContext := cronJob.Spec.JobTemplate.Spec.Template.Spec.SecurityContext
		Expect(*securityContext.RunAsUser).To(Equal(UserID))
		Expect(*securityContext.FSGroup).To(Equal(GroupID))
		Expect(*securityContext.RunAsGroup).To(Equal(GroupID))
	})

	It("should create a cleanup cronjob with the specified memory and cpu resources", func() {
		clusterDef.Spec.Snapshot.RetentionPolicy.Resources = coreV1.ResourceRequirements{
			Limits: coreV1.ResourceList{
				coreV1.ResourceMemory: resource.MustParse("68Mi"),
				coreV1.ResourceCPU:    resource.MustParse("185m"),
			},
			Requests: coreV1.ResourceList{
				coreV1.ResourceMemory: resource.MustParse("68Mi"),
			},
		}
		cluster := ACluster(clusterDef)

		cronJob := cluster.CreateSnapshotCleanupJob()
		Expect(cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers).To(HaveLen(1))

		snapshotCleanupContainer := cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
		Expect(snapshotCleanupContainer.Resources).To(Equal(coreV1.ResourceRequirements{
			Limits: coreV1.ResourceList{
				coreV1.ResourceMemory: resource.MustParse("68Mi"),
				coreV1.ResourceCPU:    resource.MustParse("185m"),
			},
			Requests: coreV1.ResourceList{
				coreV1.ResourceMemory: resource.MustParse("68Mi"),
			},
		}))
	})
})

func ACluster(clusterDef *v1alpha1.Cassandra) *Cluster {
	v1alpha1helpers.SetDefaultsForCassandra(clusterDef, nil)
	return New(clusterDef)
}

//

func haveExactly(count int, subMatcher types.GomegaMatcher) types.GomegaMatcher {
	return &haveExactlyMatcher{count, subMatcher}
}

type haveExactlyMatcher struct {
	count      int
	subMatcher types.GomegaMatcher
}

func (h *haveExactlyMatcher) Match(actual interface{}) (success bool, err error) {
	arr := reflect.ValueOf(actual)

	if arr.Kind() != reflect.Slice {
		return false, fmt.Errorf("expected []interface{}, got %v", arr.Kind())
	}

	if arr.Len() == 0 {
		fmt.Printf("zero-length slice")
		return false, fmt.Errorf("zero-length slice")
	}

	matching := 0
	for i := 0; i < arr.Len(); i++ {
		item := arr.Index(i).Interface()
		if success, _ := h.subMatcher.Match(item); success {
			matching++
		}
	}

	return matching == h.count, nil
}

func (h *haveExactlyMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("expected exactly one element of %v to match %v", actual, h.subMatcher)
}

func (h *haveExactlyMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("did not expect exactly one element of %v to match %v", actual, h.subMatcher)
}

//

func matchingConfigMap(volumeName, localObjectReference string) types.GomegaMatcher {
	return &configMapMatcher{volumeName, localObjectReference}
}

type configMapMatcher struct {
	volumeName           string
	localObjectReference string
}

func (h *configMapMatcher) Match(actual interface{}) (success bool, err error) {
	switch v := actual.(type) {
	case coreV1.Volume:
		return v.Name == h.volumeName && v.ConfigMap != nil && v.ConfigMap.LocalObjectReference.Name == h.localObjectReference, nil
	default:
		return false, fmt.Errorf("expected v1.Volume, got %v", actual)
	}
}

func (h *configMapMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("expected volume with name %s referencing config map %s", h.volumeName, h.localObjectReference)
}

func (h *configMapMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("did not expect volume with name %s referencing config map %s", h.volumeName, h.localObjectReference)
}

//

func matchingEmptyDir(volumeName string) types.GomegaMatcher {
	return &emptyDirMatcher{volumeName}
}

type emptyDirMatcher struct {
	volumeName string
}

func (h *emptyDirMatcher) Match(actual interface{}) (success bool, err error) {
	switch v := actual.(type) {
	case coreV1.Volume:
		return v.Name == h.volumeName && v.EmptyDir != nil, nil
	default:
		return false, fmt.Errorf("expected v1.Volume, got %v", actual)
	}
}

func (h *emptyDirMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("expected emptyDir volume with name %s", h.volumeName)
}

func (h *emptyDirMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("did not expect emptyDir volume with name %s", h.volumeName)
}

//

func matchingVolumeMount(mount, path string) types.GomegaMatcher {
	return &volumeMountMatcher{mount, path}
}

type volumeMountMatcher struct {
	mount string
	path  string
}

func (h *volumeMountMatcher) Match(actual interface{}) (success bool, err error) {
	switch m := actual.(type) {
	case coreV1.VolumeMount:
		return m.Name == h.mount && m.MountPath == h.path, nil
	default:
		return false, fmt.Errorf("expected v1.VolumeMount, got %v", actual)
	}
}

func (h *volumeMountMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("expected volume mount with name %s and path %s", h.mount, h.path)
}

func (h *volumeMountMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("did not expect volume mount with name %s and path %s", h.mount, h.path)
}

var _ = Describe("utility functions", func() {
	DescribeTable(
		"minQuantity",
		func(q1, q2, q3 string) {
			actual := minQuantity(resource.MustParse(q1), resource.MustParse(q2))
			expected := resource.MustParse(q3)
			Expect(actual).To(Equal(expected))
		},
		Entry("q1 > q2", "2Mi", "1Mi", "1Mi"),
		Entry("q1 < q2", "300m", "200m", "200m"),
		Entry("q1 == q2", "4", "4000m", "4"),
		Entry("q1 == q2 (retain scale)", "4000m", "4", "4000m"),
	)
})
