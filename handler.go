package pubsub

import (
	"net/http"
	"time"

	"github.com/gofrs/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

// topic is simply a wraper for it's broadcast channel giving broadcastChan a
// way to count it's listeners.
type topic struct {
	listenerCount int
	broadcastChan chan *websocket.PreparedMessage
}

// closeConn closes the given WebSocket connection.
func closeConn(conn *websocket.Conn, closeCode int) {
	d := websocket.FormatCloseMessage(closeCode, "")
	_ = conn.WriteControl(websocket.CloseMessage, d, time.Now().Add(1*time.Second))
}

func Handler(upgrader *websocket.Upgrader, eventChan chan Event) http.HandlerFunc {

	// Keep track of all topics.
	topics := map[string]*topic{}

	return func(w http.ResponseWriter, r *http.Request) {

		// Get the topic and the client type from the URL.
		topic_name := mux.Vars(r)["topic_name"]
		clientType := mux.Vars(r)["client_type"]

		// Make sure that the client type is publisher or subscriber.
		if clientType != "publisher" && clientType != "subscriber" {
			http.Error(w, "client_type must be publisher or subscriber", http.StatusBadRequest)
			return
		}

		// Upgrade the HTTP connection to the WebSocket protocol.
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// If topic_name dose not have a topic mapped to it, then map on to it.
		if _, ok := topics[topic_name]; !ok {
			topics[topic_name] = &topic{0, make(chan *websocket.PreparedMessage)}
		}

		// Generate an ID for the WebSocket connection and get the mapped topic for topic_name.
		id := r.Header.Get("servie_name") + "-" + uuid.Must(uuid.NewV4()).String()
		mappedTopic := topics[topic_name]

		switch clientType {
		case "publisher":

			eventChan <- Event{id, topic_name, clientType, nil, EventTypeNewPublisher}

			// Forever listen for inbound WebSocket messages, then send them
			// through mappedTopic's broadcast channel.
			for {

				// Listen for inbound WebSocket messages.
				msgType, p, err := conn.ReadMessage()
				if err != nil {

					// If the error is a close error, then return.
					if _, ok := err.(*websocket.CloseError); ok {
						return
					}

					// Close the connection and send the internal server error
					// through the event channel.
					e := errors.Wrap(err, "Cannot listen for inbound WebSocket messages")
					eventChan <- Event{id, topic_name, clientType, e, EventTypeInternalServerError}
					closeConn(conn, websocket.CloseInternalServerErr)
					return
				}

				// Turn the payload into a prepared message.
				msg, err := websocket.NewPreparedMessage(msgType, p)
				if err != nil {

					// Close the connection and send the internal server error
					// through the event channel.
					e := errors.Wrap(err, "Cannot listen for inbound WebSocket messages")
					eventChan <- Event{id, topic_name, clientType, e, EventTypeInternalServerError}
					closeConn(conn, websocket.CloseInternalServerErr)
					return
				}

				// Send the message to each listener of mappedTopic's broadcast channel.
				for i := 0; i < mappedTopic.listenerCount; i++ {
					mappedTopic.broadcastChan <- msg
				}
			}

		case "subscriber":

			// Add 1 to mappedTopic's listenerCount and remove 1 from it before returning.
			mappedTopic.listenerCount++
			defer func() { mappedTopic.listenerCount-- }()

			eventChan <- Event{id, topic_name, clientType, nil, EventTypeNewSubscriber}

			// Forever listen for inbound messages from mappedTopic's broadcast
			// channel, then write them to the WebSocket connection.
			for {

				msg := <-mappedTopic.broadcastChan

				// Write the message to the WebSocket connection.
				if err := conn.WritePreparedMessage(msg); err != nil {

					// If the error is a close sent error, then return.
					if err == websocket.ErrCloseSent {
						return
					}

					// Close the connection and send the internal server error
					// through the event channel.
					e := errors.Wrap(err, "Cannot write message to WebSocket connection")
					eventChan <- Event{id, topic_name, clientType, e, EventTypeInternalServerError}
					closeConn(conn, websocket.CloseInternalServerErr)
					return
				}
			}
		}
	}
}
