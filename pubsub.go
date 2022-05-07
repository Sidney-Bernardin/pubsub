package pubsub

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

func PubSub(ctx context.Context, upgrader *websocket.Upgrader, eventChan chan *Event, pongTimeout time.Duration) http.HandlerFunc {

	mu := sync.RWMutex{}

	topics := map[string]*topic{}

	return func(w http.ResponseWriter, r *http.Request) {

		topicName := r.Header.Get("topic")
		clientType := r.Header.Get("client_type")

		if clientType != "publisher" && clientType != "subscriber" {

			http.Error(w, "client_type must be publisher or subscriber", http.StatusBadRequest)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {

			e := errors.Wrap(err, "cannot upgrade HTTP connection to use WebSocket protocol")
			http.Error(w, e.Error(), http.StatusBadRequest)
			return
		}

		client := newWSClient(ctx, r.Header.Get("service_name"), conn, &mu, pongTimeout)

		if _, ok := topics[topicName]; !ok {
			topics[topicName] = newTopic(&mu)
		}

		mappedTopic := topics[topicName]

		switch clientType {
		case "publisher":

			defer func() {
				eventChan <- &Event{client.id, topicName, clientType, nil, EventTypePublisherLeft}
			}()

			eventChan <- &Event{client.id, topicName, clientType, nil, EventTypeNewPublisher}

			go client.read(false)
			go client.ping(5*time.Second, 2*time.Second)

			for {
				select {

				case <-client.ctx.Done():
					return

				case err := <-client.errChan:
					eventChan <- &Event{client.id, topicName, clientType, err, EventTypeInternalServerError}

				case msg := <-client.msgChan:

					for i := 0; i < mappedTopic.listenerCount(); i++ {
						mappedTopic.msgChan <- msg
					}
				}
			}

		case "subscriber":

			defer func() {

				mappedTopic.removeListener()

				if mappedTopic.listenerCount() == 0 {
					if _, ok := topics[topicName]; ok {
						delete(topics, topicName)
					}
				}

				eventChan <- &Event{client.id, topicName, clientType, nil, EventTypeSubscriberLeft}
			}()

			mappedTopic.addListener()

			eventChan <- &Event{client.id, topicName, clientType, nil, EventTypeNewSubscriber}

			go client.read(true)
			go client.ping(5*time.Second, 2*time.Second)

			for {
				select {

				case <-client.ctx.Done():
					return

				case err := <-client.errChan:
					eventChan <- &Event{client.id, topicName, clientType, err, EventTypeInternalServerError}

				case msg := <-mappedTopic.msgChan:

					mu.Lock()

					if err := conn.WritePreparedMessage(msg); err != nil {

						mu.Unlock()

						if err == websocket.ErrCloseSent {
							return
						}

						conn.Close()
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
