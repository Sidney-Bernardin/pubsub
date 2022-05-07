package pubsub

import (
	"sync"

	"github.com/gorilla/websocket"
)

// topic is a wrapper for it's message channel, giving it a listener count and
// methods to change the listener count.
type topic struct {
	mu        *sync.RWMutex
	listeners int
	msgChan   chan *websocket.PreparedMessage
}

// newTopic return a pointer to a topic.
func newTopic(mu *sync.RWMutex) *topic {
	return &topic{mu, 0, make(chan *websocket.PreparedMessage)}
}

func (t *topic) listenerCount() int {
	defer t.mu.RUnlock()
	t.mu.RLock()
	return t.listeners
}

// addListener increments the topic's listenerCount by 1.
func (t *topic) addListener() {
	t.mu.Lock()
	t.listeners++
	t.mu.Unlock()
}

// addListener decrements the topic's listenerCount by 1.
func (t *topic) removeListener() {
	t.mu.Lock()
	t.listeners--
	t.mu.Unlock()
}
