package pubsub

import (
	"sync"

	"github.com/pkg/errors"

	handler "github.com/Sidney-Bernardin/pubsub"
)

var (
	errAlreadySubscribed = errors.New("already subscribed to this topic")
)

type subscriber struct {
	client *handler.Client
	topic  string
}

type Pubsub struct {
	mu          sync.Mutex
	subscribers map[string]*subscriber
}

func NewPubsub() *Pubsub {
	return &Pubsub{sync.Mutex{}, map[string]*subscriber{}}
}

// addSubscriber adds a subscriber to the Pub/Sub service. The given ID should
// be unique to all added other subscribers.
func (ps *Pubsub) addSubscriber(client *handler.Client, topic string) error {

	// Make sure the given ID is unique to the other subscribers.
	if _, ok := ps.subscribers[client.ID]; !ok {
		return errAlreadySubscribed
	}

	// Create and add a new subscriber to the subscribers map.
	ps.mu.Lock()
	ps.subscribers[client.ID] = &subscriber{client, topic}
	ps.mu.Unlock()

	return nil
}

// hasSubscriber returns true if the given id matches any subscribers in the
// Pub/Sub service.
func (ps *Pubsub) hasSubscriber(id string) bool {
	_, ok := ps.subscribers[id]
	return ok
}
