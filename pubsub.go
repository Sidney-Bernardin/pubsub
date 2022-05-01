package pubsub

import (
	"context"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

// PubSub returns an HTTP handler function that acts as a WebSocket based Pub/Sub service.
func PubSub(upgrader *websocket.Upgrader, eventChan chan *Event) http.HandlerFunc {

	mu := sync.Mutex{}

	// All topics are stored in this map, topic names have a corresponding topic.
	topics := map[string]*topic{}

	return func(w http.ResponseWriter, r *http.Request) {

		// Get the topic name from the URL and the client type from the headers.
		topicName := r.URL.EscapedPath()
		clientType := r.Header.Get("client_type")

		// Make sure the client type is publisher or a subscriber.
		if clientType != "publisher" && clientType != "subscriber" {

			// Respond with an HTTP error.
			http.Error(w, "client_type must be publisher or subscriber", http.StatusBadRequest)
			return
		}

		// Upgrade the HTTP connection to use WebSocket protocol.
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {

			// Respond with an HTTP error.
			e := errors.Wrap(err, "cannot upgrade HTTP connection to use WebSocket protocol")
			http.Error(w, e.Error(), http.StatusBadRequest)
			return
		}

		// Create a wsClient. It helps handle the WebSocket connection.
		ctx, cancel := context.WithCancel(context.Background())
		client := newWSClient(conn, r.Header.Get("service_name"), cancel)

		// Check if topicName has a corresponding topic, if it dosn't, create one.
		if _, ok := topics[topicName]; !ok {
			topics[topicName] = newTopic(&mu)
		}

		// Get the corresponding topic for topicName.
		mappedTopic := topics[topicName]

		switch clientType {
		case "publisher":

			// Before returning, send an event.
			defer func() {
				eventChan <- &Event{client.id, topicName, clientType, nil, EventTypePublisherLeft}
			}()

			// Send an event.
			eventChan <- &Event{client.id, topicName, clientType, nil, EventTypeNewPublisher}

			// Read WebSocket messages from the client in another goroutine and
			// send the through client.msgChan.
			go client.read(false)

			for {
				select {

				// When the context is done, return.
				case <-ctx.Done():
					return

				// When an error is sent through client.errChan, send an
				// internal-server-error event.
				case err := <-client.errChan:
					eventChan <- &Event{client.id, topicName, clientType, err, EventTypeInternalServerError}

				// When a message is sent through client.msgChan, send the
				// message to all listeners of mappedTopic.msgChan.
				case msg := <-client.msgChan:

					// For each listener of mappedTopic.msgChan, send
					// the message to the channel.
					for i := 0; i < mappedTopic.listenerCount; i++ {
						mappedTopic.msgChan <- msg
					}
				}
			}

		case "subscriber":

			// Before returning, manage mappedTopic.listenerCount and send an event.
			defer func() {

				// Remove a listener from mappedTopic.
				mappedTopic.removeListener()

				// If there are no goroutines listening to mappedTopic.msgChan,
				// delete the topic from the map of topics.
				if mappedTopic.listenerCount == 0 {
					if _, ok := topics[topicName]; ok {
						delete(topics, topicName)
					}
				}

				// Send an event.
				eventChan <- &Event{client.id, topicName, clientType, nil, EventTypeSubscriberLeft}
			}()

			// Add a listener to mappedTopic.
			mappedTopic.addListener()

			// Send an event.
			eventChan <- &Event{client.id, topicName, clientType, nil, EventTypeNewSubscriber}

			// Read WebSocket messages from the client in another goroutine,
			// but don't sent them through client.msgChan.
			go client.read(true)

			for {
				select {

				// When the context is done, return.
				case <-ctx.Done():
					return

				// When an error is sent through client.errChan, send an
				// internal-server-error event.
				case err := <-client.errChan:
					eventChan <- &Event{client.id, topicName, clientType, err, EventTypeInternalServerError}

				// When a message is sent through mappedTopic.msgChan, write
				// the message with the WebSocket connection.
				case msg := <-mappedTopic.msgChan:

					// Write the message to the WebSocket connection.
					if err := conn.WritePreparedMessage(msg); err != nil {

						// Close the WebSocket connection and send an internal-server-error event.
						conn.Close()
						e := errors.Wrap(err, "cannot write WebSocket message")
						eventChan <- &Event{client.id, topicName, clientType, e, EventTypeInternalServerError}
						return
					}
				}
			}
		}
	}
}
