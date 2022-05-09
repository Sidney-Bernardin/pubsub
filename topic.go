package pubsub

import (
	"sync"

	"github.com/gorilla/websocket"
)

type topic struct {
	mu *sync.RWMutex

	// The amount of listeners of the topic's message channel.
	listeners int

	// Published messages go through this channel.
	msgChan chan *websocket.PreparedMessage
}

// newTopic returns a pointer to a topic.
func newTopic(mu *sync.RWMutex) *topic {
	return &topic{mu, 0, make(chan *websocket.PreparedMessage)}
}

// listenerCount returns the current amount of listeners of the topic message channel.
func (t *topic) listenerCount() int {
	defer t.mu.RUnlock()
	t.mu.RLock()
	return t.listeners
}

// addListener increments the topics listener count by one.
func (t *topic) addListener() {
	t.mu.Lock()
	t.listeners++
	t.mu.Unlock()
}

// addListener decrements the topics listener count by one.
func (t *topic) removeListener() {
	t.mu.Lock()
	t.listeners--
	t.mu.Unlock()
}
