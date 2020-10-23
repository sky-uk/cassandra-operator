package stub

import (
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	policy "k8s.io/api/policy/v1beta1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	clientV1 "k8s.io/client-go/kubernetes/typed/core/v1"
	restclient "k8s.io/client-go/rest"
)

// NewStubbedPodsGetter creates a PodsGetter that returns a stubbed list of pods with the IP addresses supplied.
// Most other properties of the pods are not set.
func NewStubbedPodsGetter(labels map[string]string, podAddresses ...string) clientV1.PodsGetter {
	podList := make([]v1.Pod, len(podAddresses))
	for i, podAddress := range podAddresses {
		podList[i] = v1.Pod{
			ObjectMeta: metaV1.ObjectMeta{
				Labels: labels,
			},
			Status: v1.PodStatus{
				PodIP: podAddress,
			}}
	}

	return &stubbedPodsGetter{&stubbedPodInterface{podList, nil}}
}

// NewFailingStubbedPodsGetter creates a PodsGetter that returns an error rather than a list of pods.
func NewFailingStubbedPodsGetter() clientV1.PodsGetter {
	return &stubbedPodsGetter{&stubbedPodInterface{nil, fmt.Errorf("failure")}}
}

type stubbedPodsGetter struct {
	podInterface *stubbedPodInterface
}

func (s *stubbedPodsGetter) Pods(namespace string) clientV1.PodInterface {
	return s.podInterface
}

type stubbedPodInterface struct {
	podsResult  []v1.Pod
	errorResult error
}

func (s *stubbedPodInterface) Create(*v1.Pod) (*v1.Pod, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *stubbedPodInterface) Update(*v1.Pod) (*v1.Pod, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *stubbedPodInterface) UpdateStatus(*v1.Pod) (*v1.Pod, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *stubbedPodInterface) Delete(name string, options *metaV1.DeleteOptions) error {
	return fmt.Errorf("not implemented")
}

func (s *stubbedPodInterface) DeleteCollection(options *metaV1.DeleteOptions, listOptions metaV1.ListOptions) error {
	return fmt.Errorf("not implemented")
}

func (s *stubbedPodInterface) Get(name string, options metaV1.GetOptions) (*v1.Pod, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *stubbedPodInterface) List(opts metaV1.ListOptions) (*v1.PodList, error) {
	if s.errorResult != nil {
		return nil, s.errorResult
	}

	if len(opts.LabelSelector) > 0 {
		selector := strings.Split(opts.LabelSelector, "=")
		matchedPods := make([]v1.Pod, 0)

		for _, pod := range s.podsResult {
			if pod.Labels[selector[0]] == selector[1] {
				matchedPods = append(matchedPods, pod)
			}
		}
		return &v1.PodList{Items: matchedPods}, nil
	}

	return &v1.PodList{Items: s.podsResult}, nil
}

func (s *stubbedPodInterface) Watch(opts metaV1.ListOptions) (watch.Interface, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *stubbedPodInterface) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Pod, err error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *stubbedPodInterface) Bind(binding *v1.Binding) error {
	return fmt.Errorf("not implemented")
}

func (s *stubbedPodInterface) Evict(eviction *policy.Eviction) error {
	return fmt.Errorf("not implemented")
}

func (s *stubbedPodInterface) GetLogs(name string, opts *v1.PodLogOptions) *restclient.Request {
	return nil
}
