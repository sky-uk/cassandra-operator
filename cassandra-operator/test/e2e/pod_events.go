package e2e

import (
	"fmt"
	"github.com/onsi/gomega"
	"github.com/sky-uk/cassandra-operator/cassandra-operator/pkg/apis/cassandra/v1alpha1"
	coreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"time"
)

type simplifiedEvent struct {
	timestamp time.Time
	eventType watch.EventType
}

type eventLog interface {
	recordEvent(name string, event simplifiedEvent)
}

type PodEventLog struct {
	events map[string][]simplifiedEvent
}

func WatchPodEvents(namespace, clusterName string) (*PodEventLog, watch.Interface) {
	eventLog := &PodEventLog{make(map[string][]simplifiedEvent)}
	watcher, err := KubeClientset.CoreV1().Pods(namespace).Watch(metaV1.ListOptions{
		LabelSelector: labelSelectorForCluster(namespace, clusterName),
	})
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	go watchEvents(watcher, eventLog)
	return eventLog, watcher
}

func watchEvents(watcher watch.Interface, events eventLog) {
	for evt := range watcher.ResultChan() {

		switch watchedResource := evt.Object.(type) {
		case *coreV1.Pod:
			podName := watchedResource.Name

			var se simplifiedEvent
			switch evt.Type {
			case watch.Added:
				se = simplifiedEvent{timestamp: watchedResource.CreationTimestamp.Time, eventType: evt.Type}
			case watch.Deleted:
				se = simplifiedEvent{timestamp: watchedResource.DeletionTimestamp.Time, eventType: evt.Type}
			default:
				continue
			}
			events.recordEvent(podName, se)
		case *v1alpha1.Cassandra:
			events.recordEvent(watchedResource.Name, simplifiedEvent{timestamp: watchedResource.CreationTimestamp.Time, eventType: evt.Type})
		default:
			continue
		}
	}
}

// The basic idea is to validate no 2 pods were down at the same time.
// It doesn't matter which pod was first deleted.
// If a pod was deleted before the other pod, it should have been (re)created before the other pod is deleted.
// If that's not the case then 2 pods were down at one moment in time, which is the error case.
func (e *PodEventLog) PodsNotDownAtTheSameTime(pod, otherPod string) (bool, error) {
	lastCreationTimeForPod, err := e.lastCreationTimeForPod(pod)
	if err != nil {
		return false, err
	}
	deletionTimeForPod, err := e.deletionTimeForPod(pod)
	if err != nil {
		return false, err
	}
	lastCreationTimeForOtherPod, err := e.lastCreationTimeForPod(otherPod)
	if err != nil {
		return false, err
	}
	deletionTimeForOtherPod, err := e.deletionTimeForPod(otherPod)
	if err != nil {
		return false, err
	}

	if deletionTimeForPod.Before(deletionTimeForOtherPod) && !lastCreationTimeForPod.Before(deletionTimeForOtherPod) {
		return false, fmt.Errorf(
			"pod %s was deleted before pod %s was recreated. \nPod %s (re)started at %v and deleted at: %v.\nPod %s (re)started at %v and deleted at: %v",
			pod, otherPod,
			pod, lastCreationTimeForPod, deletionTimeForPod,
			otherPod, lastCreationTimeForOtherPod, deletionTimeForOtherPod,
		)
	}
	if deletionTimeForOtherPod.Before(deletionTimeForPod) && !lastCreationTimeForOtherPod.Before(deletionTimeForPod) {
		return false, fmt.Errorf(
			"pod %s was deleted before pod %s was recreated. \nPod %s (re)started at %v and deleted at: %v.\nPod %s (re)started at %v and deleted at: %v",
			otherPod, pod,
			otherPod, lastCreationTimeForOtherPod, deletionTimeForOtherPod,
			pod, lastCreationTimeForPod, deletionTimeForPod,
		)
	}
	return true, nil
}

func (e *PodEventLog) PodsStartedEventCount(pod string) int {
	return len(e.findEventsOfType(pod, watch.Added))
}

func (e *PodEventLog) recordEvent(podName string, event simplifiedEvent) {
	if _, ok := e.events[podName]; !ok {
		e.events[podName] = []simplifiedEvent{}
	}
	e.events[podName] = append(e.events[podName], event)
}

func (e *PodEventLog) deletionTimeForPod(podName string) (time.Time, error) {
	return e.findLastEventTime(podName, watch.Deleted)
}

func (e *PodEventLog) lastCreationTimeForPod(podName string) (time.Time, error) {
	return e.findLastEventTime(podName, watch.Added)
}

func (e *PodEventLog) findLastEventTime(podName string, eventType watch.EventType) (time.Time, error) {
	podEvent, err := e.findLastEventOfType(podName, eventType)
	if err != nil {
		return time.Time{}, err
	}
	return podEvent.timestamp, nil
}

func (e *PodEventLog) findLastEventOfType(podName string, eventType watch.EventType) (*simplifiedEvent, error) {
	podEvents := e.findEventsOfType(podName, eventType)
	if len(podEvents) == 0 {
		return nil, fmt.Errorf("no events found for pod: %s", podName)
	}

	for i := len(podEvents) - 1; i >= 0; i-- {
		if podEvents[i].eventType == eventType {
			return &podEvents[i], nil
		}
	}
	return nil, fmt.Errorf("no events of type %s found for pod: %s", eventType, podName)
}

func (e *PodEventLog) findEventsOfType(podName string, eventType watch.EventType) []simplifiedEvent {
	var podEvents []simplifiedEvent
	for _, event := range e.events[podName] {
		if event.eventType == eventType {
			podEvents = append(podEvents, event)
		}
	}
	return podEvents
}
