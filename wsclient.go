package pubsub

import (
	"context"

	"github.com/gofrs/uuid"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

type wsClient struct {
	id      string
	conn    *websocket.Conn
	msgChan chan *websocket.PreparedMessage
	errChan chan error
	cancel  context.CancelFunc
}

func newWSClient(conn *websocket.Conn, name string, cancel context.CancelFunc) *wsClient {
	return &wsClient{
		id:      name + "-" + uuid.Must(uuid.NewV4()).String(),
		conn:    conn,
		msgChan: make(chan *websocket.PreparedMessage),
		errChan: make(chan error),
		cancel:  cancel,
	}
}

func (c *wsClient) read(ignoreMsg bool) {
	for {
		msgType, payload, err := c.conn.ReadMessage()
		if err != nil {

			if _, ok := err.(*websocket.CloseError); ok {
				c.cancel()
				return
			}

			c.errChan <- errors.Wrap(err, "cannot listen for WebSocket messages")
			c.conn.Close()
			c.cancel()
			return
		}

		if !ignoreMsg {
			msg, err := websocket.NewPreparedMessage(msgType, payload)
			if err != nil {
				c.errChan <- errors.Wrap(err, "cannot prepare WebSocket message")
				c.conn.Close()
				c.cancel()
				return
			}

			c.msgChan <- msg
		}
	}
}

func ping() {

}
