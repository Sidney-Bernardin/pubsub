package publisher

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

type Publisher struct {
	conn         *websocket.Conn
	errChan      chan error
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	pongTimeout  time.Duration
	closeTimeout time.Duration
}

func NewPublisher(dialer *websocket.Dialer, addr, topic string, headers http.Header, options ...Option) (*Publisher, *http.Response, error) {

	headers.Set("client_type", "publisher")
	headers.Set("topic", topic)

	conn, httpRes, err := dialer.Dial(addr, headers)
	if err != nil {
		return nil, httpRes, errors.Wrap(err, "cannot dial Pub/Sub server")
	}

	ctx, cancel := context.WithCancel(context.Background())
	p := &Publisher{
		conn:         conn,
		errChan:      make(chan error),
		ctx:          ctx,
		cancel:       cancel,
		wg:           sync.WaitGroup{},
		pongTimeout:  defaultPongTimeout,
		closeTimeout: defaultCloseTimeout,
	}

	for _, option := range options {
		option(p)
	}

	p.wg.Add(1)

	go p.read()

	return p, httpRes, nil
}

func (p *Publisher) Close() (err error) {

	p.cancel()

	d := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
	writeErr := p.conn.WriteControl(websocket.CloseMessage, d, time.Now().Add(p.pongTimeout))
	if writeErr != nil && writeErr != websocket.ErrCloseSent {
		err = errors.Wrap(writeErr, "cannot write WebSocket close message")
	}

	timer := time.NewTimer(p.closeTimeout)

	go func() {
		p.wg.Wait()
		timer.Reset(0 * time.Second)
	}()

	<-timer.C

	if closeErr := p.conn.Close(); closeErr != nil && err == nil {
		err = errors.Wrap(closeErr, "cannot close WebSocket connection")
	}

	return
}

func (p *Publisher) Error() chan error {
	return p.errChan
}

func (p *Publisher) Publish(msg []byte) error {
	return errors.Wrap(p.conn.WriteMessage(websocket.BinaryMessage, msg), "cannot write WebSocket message")
}

func (p *Publisher) read() {

	defer func() {
		p.cancel()
		p.wg.Done()
	}()

	for {
		select {

		case <-p.ctx.Done():
			return

		default:

			_, _, err := p.conn.ReadMessage()
			if err != nil {

				p.errChan <- errors.Wrap(err, "cannot listen for WebSocket messages")
			}
		}
	}
}
