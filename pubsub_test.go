package pubsub

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

// connect returns a WebSocket connection to the Pub/Sub server.
func connect(t *testing.T, url, clientType, topicName string) (*websocket.Conn, *http.Response, string) {

	t.Helper()

	// Add the client_type and topic headers.
	header := http.Header{}
	header.Add("client_type", clientType)
	header.Add("topic", topicName)

	// Dial the server.
	conn, httpRes, err := websocket.DefaultDialer.Dial(url, header)
	if err != nil {

		// If the error is a bad handshake error, return the HTTP response body.
		if err == websocket.ErrBadHandshake {

			// Read the HTTP response body.
			b, err := ioutil.ReadAll(httpRes.Body)
			if err != nil {
				t.Fatalf("Cannot read HTTP response body: %v", err)
			}

			return nil, httpRes, strings.Trim(string(b), "\n")
		}

		t.Fatalf("Cannot dial Pub/Sub server: %v", err)
	}

	return conn, httpRes, ""
}

func logEvents(t *testing.T, events chan *Event) {

	t.Helper()

	l := zerolog.New(os.Stdout)
	for {
		e := <-events

		if e.EventType == EventTypeInternalServerError {
			l.Warn().Err(e.Error).
				Str("id", e.ClientID).
				Str("topic", e.Topic).
				Str("client_type", e.ClientType).
				Str("client_type", e.ClientType).
				Msg("Internal Server Error")
		}
	}
}

func TestPubSub(t *testing.T) {

	tt := []struct {
		name string

		pubClientType         string
		pubTopic              string
		pubMsg                []byte
		pubExpectedStatusCode int
		pubExpectedBody       string

		subClientType         string
		subTopic              string
		subExpectedMsg        []byte
		subExpectedStatusCode int
		subExpectedBody       string
	}{
		{
			name:                  "test1",
			pubClientType:         "publisher",
			pubTopic:              "test",
			pubMsg:                []byte("Hello, World!"),
			pubExpectedStatusCode: http.StatusSwitchingProtocols,
			subClientType:         "subscriber",
			subTopic:              "test",
			subExpectedMsg:        []byte("Hello, World!"),
			subExpectedStatusCode: http.StatusSwitchingProtocols,
		},
		{
			name:                  "test2",
			pubClientType:         "publisher",
			pubTopic:              "test",
			pubMsg:                []byte("Hello, World!"),
			pubExpectedStatusCode: http.StatusSwitchingProtocols,
			subClientType:         "invalid",
			subTopic:              "test",
			subExpectedMsg:        []byte("Hello, World!"),
			subExpectedStatusCode: http.StatusBadRequest,
			subExpectedBody:       ErrMustBePubOrSub.Error(),
		},
		{
			name:                  "test3",
			pubClientType:         "invalid",
			pubTopic:              "test",
			pubMsg:                []byte("Hello, World!"),
			pubExpectedStatusCode: http.StatusBadRequest,
			pubExpectedBody:       ErrMustBePubOrSub.Error(),
			subClientType:         "subscriber",
			subTopic:              "test",
			subExpectedMsg:        []byte("Hello, World!"),
			subExpectedStatusCode: http.StatusSwitchingProtocols,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			// Create a Pub/Sub handler.
			eventChan := make(chan *Event)
			h := PubSub(context.Background(), &websocket.Upgrader{}, eventChan, time.Second)
			go logEvents(t, eventChan)

			// Create a test server for the Pub/Sub handler.
			srv := httptest.NewServer(h)
			defer srv.Close()
			wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

			// Connect to the Pub/Sub server as a subscriber.
			subConn, res, body := connect(t, wsURL, tc.subClientType, tc.subTopic)

			// Check the status code.
			if res.StatusCode != tc.subExpectedStatusCode {
				t.Fatalf("Subscriber got wrong status code. Expected '%v' got '%v'.", tc.subExpectedStatusCode, res.StatusCode)
			}

			// Check the body.
			if body != tc.subExpectedBody {
				t.Fatalf("Subscriber got wrong body. Expected '%v' got '%v'.", tc.subExpectedBody, body)
			}

			// Connect to the Pub/Sub server as a publisher.
			pubConn, res, body := connect(t, wsURL, tc.pubClientType, tc.pubTopic)

			// Check the status code.
			if res.StatusCode != tc.pubExpectedStatusCode {
				t.Fatalf("Publisher got wrong status code. Expected '%v' got '%v'.", tc.pubExpectedStatusCode, res.StatusCode)
			}

			// Check the body.
			if body != tc.pubExpectedBody {
				t.Fatalf("Publisher got wrong body. Expected '%v' got '%v'.", tc.subExpectedBody, body)
			}

			// If any of the connections are nil, return.
			if pubConn == nil || subConn == nil {
				return
			}

			// Tell the publisher to write a WebSocket message to the Pub/Sub sever.
			if err := pubConn.WriteMessage(websocket.BinaryMessage, tc.pubMsg); err != nil {
				t.Fatalf("Cannot write WebSocket message: %v", err)
			}

			// Tell the subscriber to listen for messages from the Pub/Sub server.
			subConn.SetReadDeadline(time.Now().Add(3 * time.Second))
			_, payload, err := subConn.ReadMessage()
			if err != nil {
				t.Fatalf("Subscriber cannot listen for WebSocket messages: %v", err)
			}

			// Check the messages.
			if string(payload) != string(tc.subExpectedMsg) {
				t.Fatalf("Subscriber got wrong message. Expected '%s' got '%s'.", tc.subExpectedMsg, payload)
			}
		})
	}
}
