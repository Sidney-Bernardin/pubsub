package pubsub

import (
	"sync"

	"github.com/pkg/errors"
)

var (
	errAlreadySubscribed = errors.New("already subscribed to this topic")
)

type client interface {
	GetID() string
}

type subscription struct {
	client
	topic string
}

type Pubsub struct {
	mu            sync.Mutex
	subscriptions map[string]*subscription
}

func NewPubsub() *Pubsub {
	return &Pubsub{sync.Mutex{}, map[string]*subscription{}}
}

// AddSubscriber lets the Pub/Sub service know that the given client is
// subscribed. The given ID should be unique to all added other subscribers.
func (ps *Pubsub) AddSubscriber(c client, topic string) error {

	// Make sure the given ID is unique to the other subscribers.
	if _, ok := ps.subscriptions[c.GetID()]; !ok {
		return errAlreadySubscribed
	}

	// Create and add a new subscriber to the subscribers map.
	ps.mu.Lock()
	ps.subscriptions[c.GetID()] = &subscription{c, topic}
	ps.mu.Unlock()

	return nil
}

// RemoveSubscriber removes all memory of the given subscriber from the Pub/Sub service.
func (ps *Pubsub) RemoveSubscriber(id string) {
	if _, ok := ps.subscriptions[id]; ok {
		delete(ps.subscriptions, id)
	}
}
