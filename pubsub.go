package pubsub

import (
	"sync"

	"github.com/pkg/errors"
)

var (
	errAlreadySubscribed = errors.New("already subscribed to this topic")
)

type subscription struct {
	c     *client
	topic string
}

type pubsub struct {
	mu            *sync.Mutex
	subscriptions map[string]*subscription
}

// AddSubscriber lets the Pub/Sub service know that the given client is
// subscribed. The given ID should be unique to all other added subscribers.
func (ps *pubsub) AddSubscriber(c *client, topic string) error {

	// Make sure the given ID is unique to the other subscribers.
	if _, ok := ps.subscriptions[c.id]; !ok {
		return errAlreadySubscribed
	}

	// Create and add a new subscriber to the subscribers map.
	ps.mu.Lock()
	ps.subscriptions[c.id] = &subscription{c, topic}
	ps.mu.Unlock()

	return nil
}

// RemoveSubscriber removes all memory of the given subscriber from the Pub/Sub service.
func (ps *pubsub) RemoveSubscriber(id string) {
	if _, ok := ps.subscriptions[id]; ok {
		delete(ps.subscriptions, id)
	}
}
