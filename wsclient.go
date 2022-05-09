package pubsub

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/gofrs/uuid"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

// wsClient helps a *websocket.Conn read and ping.
type wsClient struct {
	mu   *sync.RWMutex
	id   string
	conn *websocket.Conn

	// WebSocket messages and errors from the WebSocket client server are sent
	// through these channels.
	msgChan chan *websocket.PreparedMessage
	errChan chan error

	// Context with a cancel function helps shutdown wsClient's long
	// running goroutines.
	ctx    context.Context
	cancel context.CancelFunc

	pongTimeout time.Duration
}

// newWSClient return a pointer to a wsClient.
func newWSClient(ctx context.Context, name string, conn *websocket.Conn, mu *sync.RWMutex, pongTimeout time.Duration) *wsClient {

	// Wrap the given context.
	newCTX, cancel := context.WithCancel(ctx)

	return &wsClient{
		id:          name + "-" + uuid.Must(uuid.NewV4()).String(),
		conn:        conn,
		mu:          mu,
		msgChan:     make(chan *websocket.PreparedMessage),
		errChan:     make(chan error),
		ctx:         newCTX,
		cancel:      cancel,
		pongTimeout: pongTimeout,
	}
}

// closeConn closes wsClient's WebSocket connection.
func (c *wsClient) closeConn() {

	// Write a close WebSocket message to the WebSocket client.
	d := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
	err := c.conn.WriteControl(websocket.CloseMessage, d, time.Now().Add(c.pongTimeout))
	if err != nil && err != websocket.ErrCloseSent {

		// Send the error through wsClient's error channel.
		c.errChan <- errors.Wrap(err, "cannot write WebSocket close message")
	}

	// Close wsClient's WebSocket connection.
	if err := c.conn.Close(); err != nil {

		// Send the error through wsClient's error channel.
		c.errChan <- errors.Wrap(err, "cannot close WebSocket connection")
	}
}

// read is ment to run in it's own goroutine while it listens for incoming
// WebSocket messages from the WebSocket client until wsClient's context is canceled.
func (c *wsClient) read(ignoreMsg bool) {

	// Close wsClient's WebSocket connection, then cancel wsClient's context.
	defer func() {
		c.closeConn()
		c.cancel()
	}()

	for {

		// Listen for incoming WebSocket messages form the WebSocket client.
		msgType, payload, err := c.conn.ReadMessage()
		if err != nil {

			// If the error is a close error, return.
			if _, ok := err.(*websocket.CloseError); ok {
				return
			}

			// If the error is a timeout error, return.
			if err, ok := err.(net.Error); ok && err.Timeout() {
				return
			}

			// Send the error through wsClient's error channel.
			c.errChan <- errors.Wrap(err, "cannot listen for WebSocket messages")
			return
		}

		select {

		// Wait for wsClient's context to be done.
		case <-c.ctx.Done():
			return

		// Convert the payload into a prepared WebSocket message, then send the
		// message through wsClient's message channel.
		default:

			if !ignoreMsg {

				// Convert the payload into a prepared WebSocket message.
				msg, err := websocket.NewPreparedMessage(msgType, payload)
				if err != nil {

					// Send the error through wsClient's error channel.
					c.errChan <- errors.Wrap(err, "cannot prepare WebSocket message")
					return
				}

				// Send the prepared WebSocket message through wsClient's message channel.
				c.msgChan <- msg
			}
		}
	}
}

// ping is ment to run in it's own goroutine while it pings the WebSocket
// client until wsClient's context is canceled.
func (c *wsClient) ping(timeout, frequency time.Duration) {

	// Close wsClient's WebSocket connection, then cancel wsClient's context.
	defer func() {
		c.closeConn()
		c.cancel()
	}()

	// Set the handler for pong WebSocket messages from the WebSocket client.
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(timeout))
	})

	// Create a ticker for pinging.
	ticker := time.NewTicker(frequency)

	for {
		select {

		// Wait for wsClient's context to be done.
		case <-c.ctx.Done():
			return

		// When a tick is sent through the ticker's channel, write a ping
		// WebSocket message to the WebSocket client.
		case <-ticker.C:

			c.mu.RLock()

			// Write a ping WebSocket message to the WebSocket client.
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {

				c.mu.RUnlock()

				// If the error is a close error, return.
				if err == websocket.ErrCloseSent {
					return
				}

				// Send the error through wsClient's error channel.
				c.errChan <- errors.Wrap(err, "cannot write WebSocket ping message")
				return
			}

			c.mu.RUnlock()
		}
	}
}
