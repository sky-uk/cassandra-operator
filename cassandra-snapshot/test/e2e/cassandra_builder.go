package e2e

import (
	"fmt"
	"github.com/onsi/gomega"
	"k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

var (
	terminateImmediately = int64(0)
)

type CassandraPodBuilder struct {
	envVars []v1.EnvVar
}

func ACassandraPodWithDefaults() *CassandraPodBuilder {
	return &CassandraPodBuilder{
		envVars: []v1.EnvVar{
			{
				Name:  "NODETOOL_ARGS",
				Value: "-DhelloWorld=testEnvVar",
			},
		},
	}
}

func (c *CassandraPodBuilder) WithoutEnvironmentVariables() *CassandraPodBuilder {
	c.envVars = nil
	return c
}

func (c *CassandraPodBuilder) Exists(labelsAndValues ...string) *v1.Pod {
	podName := fmt.Sprintf("cassandra-pod-%s", randomString(5))
	labels := make(map[string]string)
	for i := 0; i < len(labelsAndValues)-1; i += 2 {
		labels[labelsAndValues[i]] = labelsAndValues[i+1]
	}

	var pod *v1.Pod
	var err error
	if UseMockedImage {
		pod, err = createCassandraPod(labels, podName, c)
	} else {
		pod, err = createCassandraPodWithCustomConfig(labels, podName, c)
	}

	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	gomega.Eventually(PodIsReady(pod), NodeStartDuration, 2*time.Second).Should(gomega.BeTrue())
	return pod
}

func createCassandraPod(labels map[string]string, podName string, builder *CassandraPodBuilder) (*v1.Pod, error) {
	return KubeClientset.CoreV1().Pods(Namespace).Create(&v1.Pod{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      podName,
			Namespace: Namespace,
			Labels:    labels,
		},
		Spec: v1.PodSpec{
			SecurityContext: securityContext,
			Containers: []v1.Container{
				{
					Name:            "cassandra",
					Image:           CassandraImageName,
					ReadinessProbe:  CassandraReadinessProbe,
					Resources:       resourceRequirementsOf("50Mi"),
					Env:             builder.envVars,
					ImagePullPolicy: v1.PullAlways,
				},
			},
			TerminationGracePeriodSeconds: &terminateImmediately,
		},
	})
}

func createCassandraPodWithCustomConfig(labels map[string]string, podName string, builder *CassandraPodBuilder) (*v1.Pod, error) {
	configMap, err := cassandraConfigMap(Namespace, podName)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	return KubeClientset.CoreV1().Pods(Namespace).Create(&v1.Pod{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      podName,
			Namespace: Namespace,
			Labels:    labels,
		},
		Spec: v1.PodSpec{
			SecurityContext: securityContext,
			InitContainers: []v1.Container{
				{
					Name:      "copy-default-cassandra-config",
					Image:     CassandraImageName,
					Command:   []string{"sh", "-c", "cp -vr /etc/cassandra/* /config"},
					Resources: resourceRequirementsOf("50Mi"),
					VolumeMounts: []v1.VolumeMount{
						{Name: "config", MountPath: "/config"},
					},
				},
				{
					Name:      "copy-custom-config",
					Image:     "busybox",
					Command:   []string{"sh", "-c", "cp -rLv /custom-config/* /config"},
					Resources: resourceRequirementsOf("50Mi"),
					VolumeMounts: []v1.VolumeMount{
						{Name: "config", MountPath: "/config"},
						{Name: "custom-config", MountPath: "/custom-config"},
					},
				},
			},
			Containers: []v1.Container{
				{
					Name:           "cassandra",
					Image:          CassandraImageName,
					ReadinessProbe: CassandraReadinessProbe,
					Resources:      resourceRequirementsOf("1Gi"),
					VolumeMounts: []v1.VolumeMount{
						{Name: "config", MountPath: "/etc/cassandra"},
					},
					Env: builder.envVars,
				},
			},
			TerminationGracePeriodSeconds: &terminateImmediately,
			Volumes: []v1.Volume{
				{
					Name: "config",
					VolumeSource: v1.VolumeSource{
						EmptyDir: &v1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "custom-config",
					VolumeSource: v1.VolumeSource{
						ConfigMap: &v1.ConfigMapVolumeSource{
							LocalObjectReference: v1.LocalObjectReference{
								Name: configMap.Name,
							},
						},
					},
				},
			},
		},
	})
}
