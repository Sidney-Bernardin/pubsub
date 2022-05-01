package pubsub

import (
	"context"

	"github.com/gofrs/uuid"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

// wsClient handles reading and pinging for a given WebSocket connection.
type wsClient struct {
	id      string
	conn    *websocket.Conn
	msgChan chan *websocket.PreparedMessage
	errChan chan error
	cancel  context.CancelFunc
}

// newWSClient returns a pointer to a wsClient.
func newWSClient(conn *websocket.Conn, name string, cancel context.CancelFunc) *wsClient {
	return &wsClient{
		id:      name + "-" + uuid.Must(uuid.NewV4()).String(),
		conn:    conn,
		msgChan: make(chan *websocket.PreparedMessage),
		errChan: make(chan error),
		cancel:  cancel,
	}
}

// read forever listens for WebSocket message from the client. If an error
// occurs during the process, the function will return and c.cancel wll be called.
func (c *wsClient) read(ignoreMsg bool) {

	// Before returning, call c.cancel.
	defer c.cancel()

	for {

		// Listen for WebSocket message from the client.
		msgType, payload, err := c.conn.ReadMessage()
		if err != nil {

			// If it's a close error, return.
			if _, ok := err.(*websocket.CloseError); ok {
				return
			}

			// Send the error through c.errChan and close the WebSocket connection.
			c.errChan <- errors.Wrap(err, "cannot listen for WebSocket messages")
			c.conn.Close()
			return
		}

		if !ignoreMsg {

			// Turn the payload into a new prepared WebSocket message.
			msg, err := websocket.NewPreparedMessage(msgType, payload)
			if err != nil {

				// Send the error through c.errChan and close the WebSocket connection.
				c.errChan <- errors.Wrap(err, "cannot prepare WebSocket message")
				c.conn.Close()
				return
			}

			// Send the message through c.msgChan.
			c.msgChan <- msg
		}
	}
}
