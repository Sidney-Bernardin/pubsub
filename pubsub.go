package pubsub

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

var (
	ErrMustBePubOrSub = errors.New("client_type must be publisher or subscriber")
)

func PubSub(ctx context.Context, upgrader *websocket.Upgrader, eventChan chan *Event, pongTimeout time.Duration) http.HandlerFunc {

	mu := sync.RWMutex{}

	// This map of topics lets publishers send messages to the correct topics,
	// and lets subscribers listen for messages from the correct topics.
	topics := map[string]*topic{}

	return func(w http.ResponseWriter, r *http.Request) {

		// Get the topic and client_type headers.
		topicName := r.Header.Get("topic")
		clientType := r.Header.Get("client_type")

		// Make sure the client_type is either publisher or subscriber.
		if clientType != "publisher" && clientType != "subscriber" {

			// Send an HTTP error.
			http.Error(w, ErrMustBePubOrSub.Error(), http.StatusBadRequest)
			return
		}

		// Upgrade the HTTP connection to a WebSocket connection.
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {

			// Send an HTTP error.
			e := errors.Wrap(err, "cannot upgrade HTTP connection to use WebSocket protocol")
			http.Error(w, e.Error(), http.StatusBadRequest)
			return
		}

		// Create a new wsClient.
		client := newWSClient(ctx, r.Header.Get("service_name"), conn, &mu, pongTimeout)

		// If the topic doesn't exist, create it.
		if _, ok := topics[topicName]; !ok {
			topics[topicName] = newTopic(&mu)
		}

		// Get the topic from the topics map.
		mappedTopic := topics[topicName]

		switch clientType {
		case "publisher":

			// When this function finishes, send an event through the event channel.
			defer func() {
				eventChan <- &Event{client.id, topicName, clientType, nil, EventTypePublisherLeft}
			}()

			// Send an event through the event channel.
			eventChan <- &Event{client.id, topicName, clientType, nil, EventTypeNewPublisher}

			// Read and ping in seperate goroutines.
			go client.read(false)
			go client.ping(5*time.Second, 2*time.Second)

			for {
				select {

				// Wait for client's context to be done.
				case <-client.ctx.Done():
					return

				// When an error is sent through client's error channel, send
				// an event through the event channel.
				case err := <-client.errChan:
					eventChan <- &Event{client.id, topicName, clientType, err, EventTypeInternalServerError}

				// When a message is sent through client's message channel,
				// sent it through mapped topic message channel.
				case msg := <-client.msgChan:

					// For each listener of the mapped topic send the message
					// through the mapped topic's message channel.
					for i := 0; i < mappedTopic.listenerCount(); i++ {
						mappedTopic.msgChan <- msg
					}
				}
			}

		case "subscriber":

			// When this function finishes, clean up the mapped topic's listener count.
			defer func() {

				// Tell the mapped topic that a subscriber left.
				mappedTopic.removeListener()

				// If there are no more listeners of the mapped topic, delete
				// the mapped topic from the topics map.
				if mappedTopic.listenerCount() == 0 {
					if _, ok := topics[topicName]; ok {
						delete(topics, topicName)
					}
				}

				// Send an event through the event channel.
				eventChan <- &Event{client.id, topicName, clientType, nil, EventTypeSubscriberLeft}
			}()

			// Tell the mapped topic that it has a new listener.
			mappedTopic.addListener()

			// Send an event through the event channel.
			eventChan <- &Event{client.id, topicName, clientType, nil, EventTypeNewSubscriber}

			// Read and ping in seperate goroutines.
			go client.read(true)
			go client.ping(5*time.Second, 2*time.Second)

			for {
				select {

				// Wait for client's context to be done.
				case <-client.ctx.Done():
					return

				// When an error is sent through client's error channel, send
				// an event through the event channel.
				case err := <-client.errChan:
					eventChan <- &Event{client.id, topicName, clientType, err, EventTypeInternalServerError}

				// When a message is sent through the mapped topics's message
				// channel, write it to the WebSocket client.
				case msg := <-mappedTopic.msgChan:

					mu.Lock()

					// Write the message to the WebSocket client.
					if err := conn.WritePreparedMessage(msg); err != nil {

						mu.Unlock()

						// If the error is a close error, return.
						if err == websocket.ErrCloseSent {
							return
						}

						// Close the WebSocket connection.
						conn.Close()

						// Send an event through the event channel.
						e := errors.Wrap(err, "cannot write WebSocket message")
						eventChan <- &Event{client.id, topicName, clientType, e, EventTypeInternalServerError}
						return
					}

					mu.Unlock()
				}
			}
		}
	}
}
