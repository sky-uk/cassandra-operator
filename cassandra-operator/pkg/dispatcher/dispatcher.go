package dispatcher

import (
	log "github.com/sirupsen/logrus"
	"sync"
)

// Event describes an event which can happen to a particular entity. Events have a kind (e.g. "created", "modified",
// "deleted"), a key which uniquely identifies the entity the event applies to, and event-specific data.
type Event struct {
	ID   uint64
	Kind string
	Key  string
	Data interface{}
}

// Dispatcher is a component which takes events bound for a particular entity, and routes them to a worker which
// will handle those events and apply changes to the entity sequentially.
//
// The goal of this approach is to ensure that changes made to Cassandra cluster definitions are applied consistently,
// for example, a cluster deletion cannot be initiated while the cluster is still being created.
type Dispatcher interface {
	// Dispatch the supplied Event to an appropriate worker
	Dispatch(e *Event)
}

// New Dispatcher. The handlerFunc will be invoked to handle a single Event after dispatch,
// once stopCh is closed, no more events would be handled
func New(handlerFunc func(*Event), stopCh <-chan struct{}) Dispatcher {
	dispatcher := &dispatcher{
		handlerFunc:             handlerFunc,
		eventProcessingChannels: make(map[string]chan Event),
		stopCh:                  stopCh,
		dispatchLock:            sync.Mutex{},
	}
	go dispatcher.watchStopChannel()

	return dispatcher
}

type dispatcher struct {
	eventProcessingChannels map[string]chan Event
	handlerFunc             func(*Event)
	stopCh                  <-chan struct{}
	stopped                 bool
	dispatchLock            sync.Mutex
	eventCounter            uint64
}

const clusterEventBufferSize = 100

func (d *dispatcher) Dispatch(e *Event) {
	if !d.stopped {
		d.dispatchLock.Lock()
		defer d.dispatchLock.Unlock()
		eventProcessingChannel, ok := d.eventProcessingChannels[e.Key]
		if !ok {
			eventProcessingChannel = make(chan Event, clusterEventBufferSize)
			d.eventProcessingChannels[e.Key] = eventProcessingChannel
			log.Infof("Starting event worker for key: %s", e.Key)
			go d.start(eventProcessingChannel, d.handlerFunc, d.stopCh)
		}
		d.eventCounter++
		e.ID = d.eventCounter
		log.Debugf("Event dispatched id=%d key=%s type=%s", e.ID, e.Key, e.Kind)
		eventProcessingChannel <- *e
	} else {
		log.Warnf("Ignoring event with kind: %s and key: %s, as the event dispatching was stopped", e.Kind, e.Key)
	}
}

func (d *dispatcher) watchStopChannel() {
	for {
		select {
		case <-d.stopCh:
			d.stopped = true
			return
		}
	}
}

func (d *dispatcher) start(eventsIn <-chan Event, handler func(*Event), stopCh <-chan struct{}) {
	for {
		select {
		case e := <-eventsIn:
			handler(&e)
		case <-stopCh:
			return
		}
	}
}
