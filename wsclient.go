package pubsub

import (
	"context"
	"time"

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

// read forever listens for WebSocket message from the client. If ignoreMsg is
// false, the message will be send through wsClient.msgChan. If an error
// occurs, the error will be sent through wsClient.errChan, the WebSocket
// connection will close, and all wsClient functions will be canceled.
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

// ping writes WebSocket ping messages to the client. Frequency is how
// often a ping should be written, and timeout is the max time it should take
// for a pong message to be recived.
func (c *wsClient) ping(timeout, frequency time.Duration) {

	// Before returning, call c.cancel.
	defer c.cancel()

	// lastPong is the time when the last pong message was recived.
	lastPong := time.Now()

	// Set the pong handler to a function that resets lastPong to the current time.
	c.conn.SetPongHandler(func(_ string) error {
		lastPong = time.Now()
		return nil
	})

	// Create a ticker.
	ticker := time.NewTicker(frequency)

	for {
		select {

		// When c.ctx is done, return.
		case <-c.ctx.Done():
			return

		// When a tick is sent through ticker.C, ping the client.
		case <-ticker.C:

			// If the pong didn't come soon enough, return.
			if time.Now().Sub(lastPong) > timeout {
				c.conn.Close()
				return
			}

			// Write a WebSocket ping message to the client.
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.errChan <- errors.Wrap(err, "cannot write WebSocket ping message")
				c.conn.Close()
				return
			}
		}
	}
}
