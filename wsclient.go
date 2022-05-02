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
	ctx     context.Context
	cancel  context.CancelFunc
}

// newWSClient returns a pointer to a wsClient.
func newWSClient(ctx context.Context, conn *websocket.Conn, name string) *wsClient {

	// Give the given context a cancel function.
	newCTX, cancel := context.WithCancel(ctx)

	return &wsClient{
		id:      name + "-" + uuid.Must(uuid.NewV4()).String(),
		conn:    conn,
		msgChan: make(chan *websocket.PreparedMessage),
		errChan: make(chan error),
		ctx:     newCTX,
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

		select {

		// When c.ctx is done, return.
		case <-c.ctx.Done():
			return

		// Default to sending the message through c.msgChan.
		default:

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
}
