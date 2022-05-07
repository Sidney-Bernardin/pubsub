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

type wsClient struct {
	id          string
	conn        *websocket.Conn
	mu          *sync.RWMutex
	msgChan     chan *websocket.PreparedMessage
	errChan     chan error
	ctx         context.Context
	cancel      context.CancelFunc
	pongTimeout time.Duration
}

func newWSClient(ctx context.Context, name string, conn *websocket.Conn, mu *sync.RWMutex, pongTimeout time.Duration) *wsClient {

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

func (c *wsClient) closeConn() {

	d := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
	err := c.conn.WriteControl(websocket.CloseMessage, d, time.Now().Add(c.pongTimeout))
	if err != nil && err != websocket.ErrCloseSent {
		c.errChan <- errors.Wrap(err, "cannot write WebSocket close message")
	}

	if err := c.conn.Close(); err != nil {
		c.errChan <- errors.Wrap(err, "cannot close WebSocket connection")
	}
}

func (c *wsClient) read(ignoreMsg bool) {

	defer func() {
		c.closeConn()
		c.cancel()
	}()

	for {

		msgType, payload, err := c.conn.ReadMessage()
		if err != nil {

			if _, ok := err.(*websocket.CloseError); ok {
				return
			}

			if err, ok := err.(net.Error); ok && err.Timeout() {
				return
			}

			c.errChan <- errors.Wrap(err, "cannot listen for WebSocket messages")
			return
		}

		select {

		case <-c.ctx.Done():
			return

		default:

			if !ignoreMsg {

				msg, err := websocket.NewPreparedMessage(msgType, payload)
				if err != nil {

					c.errChan <- errors.Wrap(err, "cannot prepare WebSocket message")
					return
				}

				c.msgChan <- msg
			}
		}
	}
}

func (c *wsClient) ping(timeout, frequency time.Duration) {

	defer func() {
		c.closeConn()
		c.cancel()
	}()

	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(timeout))
	})

	ticker := time.NewTicker(frequency)

	for {
		select {

		case <-c.ctx.Done():
			return

		case <-ticker.C:

			c.mu.RLock()

			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {

				c.mu.RUnlock()

				if err == websocket.ErrCloseSent {
					return
				}

				c.errChan <- errors.Wrap(err, "cannot write WebSocket ping message")
				return
			}

			c.mu.RUnlock()
		}
	}
}
