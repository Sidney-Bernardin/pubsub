package subscriber

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

var (
	ErrNotBinaryMessage = errors.New("message type is not binary")
)

type Subscriber struct {
	conn         *websocket.Conn
	msgChan      chan []byte
	errChan      chan error
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	pongTimeout  time.Duration
	closeTimeout time.Duration
}

func NewSubscriber(dialer *websocket.Dialer, addr, topic string, headers http.Header, options ...Option) (*Subscriber, *http.Response, error) {

	headers.Set("client_type", "subscriber")
	headers.Set("topic", topic)

	conn, httpRes, err := dialer.Dial(addr, headers)
	if err != nil {
		return nil, httpRes, errors.Wrap(err, "cannot dial Pub/Sub server")
	}

	ctx, cancel := context.WithCancel(context.Background())
	p := &Subscriber{
		conn:         conn,
		msgChan:      make(chan []byte),
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

func (p *Subscriber) Close() (err error) {

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

func (p *Subscriber) Error() chan error {
	return p.errChan
}

func (p *Subscriber) read() {

	defer func() {
		p.cancel()
		p.wg.Done()
	}()

	for {
		select {

		case <-p.ctx.Done():
			return

		default:

			msgType, payload, err := p.conn.ReadMessage()
			if err != nil {
				p.errChan <- errors.Wrap(err, "cannot listen for WebSocket messages")
			}

			if msgType != websocket.BinaryMessage {
				p.errChan <- errors.Wrapf(ErrNotBinaryMessage, "got WebSocket message of type %v", msgType)
				continue
			}

			p.msgChan <- payload
		}
	}
}
