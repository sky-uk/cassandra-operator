package cluster

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	typedV1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
)

const (
	// InvalidClusterEvent describes an event for an invalid cluster
	InvalidClusterEvent = "InvalidCluster"
	// InvalidChangeEvent describes an event for an invalid change
	InvalidChangeEvent = "InvalidChange"
	// ClusterUpdateEvent describes an event for a cluster update
	ClusterUpdateEvent = "ClusterUpdate"
	// WaitingForStatefulSetChange is an event created when waiting for a stateful set change to complete
	WaitingForStatefulSetChange = "WaitingForStatefulSetChange"
	// StatefulSetChangeComplete is an event created when a stateful set change iscomplete
	StatefulSetChangeComplete = "StatefulSetChangeComplete"
	// ClusterSnapshotCreationScheduleEvent is an event triggered on creation of a scheduled snapshot
	ClusterSnapshotCreationScheduleEvent = "ClusterSnapshotCreationScheduleEvent"
	// ClusterSnapshotCreationUnscheduleEvent is an event triggered on removal of a scheduled snapshot
	ClusterSnapshotCreationUnscheduleEvent = "ClusterSnapshotCreationUnscheduleEvent"
	// ClusterSnapshotCreationModificationEvent is an event triggered when the scheduled snapshot is modified
	ClusterSnapshotCreationModificationEvent = "ClusterSnapshotCreationModificationEvent"
	// ClusterSnapshotCleanupScheduleEvent is an event triggered when scheduling a snapshot cleanup
	ClusterSnapshotCleanupScheduleEvent = "ClusterSnapshotCleanupScheduleEvent"
	// ClusterSnapshotCleanupUnscheduleEvent is an event triggered when scheduling a snapshot cleanup
	ClusterSnapshotCleanupUnscheduleEvent = "ClusterSnapshotCleanupUnscheduleEvent"
	// ClusterSnapshotCleanupModificationEvent is an event triggered when the snapshot cleanup job is modified
	ClusterSnapshotCleanupModificationEvent = "ClusterSnapshotCleanupModificationEvent"

	operatorNamespace = ""
)

// NewEventRecorder creates an EventRecorder which can be used to record events reflecting the state of operator
// managed clusters. It correctly does aggregation of repeated events into a count, first timestamp and last timestamp.
func NewEventRecorder(kubeClientset *kubernetes.Clientset, s *runtime.Scheme) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&typedV1.EventSinkImpl{Interface: kubeClientset.CoreV1().Events(operatorNamespace)})
	return eventBroadcaster.NewRecorder(s, v1.EventSource{Component: "cassandra-operator"})
}
