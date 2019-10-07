package event

const (
	// AddCluster is a kind of event which the receiver is able to handle
	AddCluster = "ADD_CLUSTER"
	// DeleteCluster is a kind of event which the receiver is able to handle
	DeleteCluster = "DELETE_CLUSTER"
	// UpdateCluster is a kind of event which the receiver is able to handle
	UpdateCluster = "UPDATE_CLUSTER"
	// GatherMetrics is a kind of event which the receiver is able to handle
	GatherMetrics = "GATHER_METRICS"
	// AddCustomConfig is a kind of event which the receiver is able to handle
	AddCustomConfig = "ADD_CUSTOM_CONFIG"
	// UpdateCustomConfig is a kind of event which the receiver is able to handle
	UpdateCustomConfig = "UPDATE_CUSTOM_CONFIG"
	// DeleteCustomConfig is a kind of event which the receiver is able to handle
	DeleteCustomConfig = "DELETE_CUSTOM_CONFIG"
	// AddService is a kind of event which the receiver is able to handle
	AddService = "ADD_SERVICE"
)

// Event describes an event which can happen to a particular entity. Events have a kind (e.g. "created", "modified",
// "deleted"), a key which uniquely identifies the entity the event applies to, and event-specific data.
type Event struct {
	Kind string
	Key  string
	Data interface{}
}
