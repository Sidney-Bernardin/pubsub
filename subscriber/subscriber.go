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

// Subscriber subscribes to the Pub/Sub server.
type Subscriber struct {
	wg   sync.WaitGroup
	conn *websocket.Conn

	// WebSocket messages and errors from the Pub/Sub server are sent through these channels.
	msgChan chan []byte
	errChan chan error

	// Context with a cancel function helps shutdown the subscriber's long
	// running goroutines.
	ctx    context.Context
	cancel context.CancelFunc

	// Timeouts.
	pongTimeout  time.Duration
	closeTimeout time.Duration
}

// NewSubscriber returns a pointer to a Subscriber.
func NewSubscriber(dialer *websocket.Dialer, addr, topic string, headers http.Header, options ...Option) (*Subscriber, *http.Response, error) {

	// Set the client_type and topic headers.
	headers.Set("client_type", "subscriber")
	headers.Set("topic", topic)

	// Connect to the Pub/Sub server.
	conn, httpRes, err := dialer.Dial(addr, headers)
	if err != nil {
		return nil, httpRes, errors.Wrap(err, "cannot dial Pub/Sub server")
	}

	// Create a cancel context for the subscriber.
	ctx, cancel := context.WithCancel(context.Background())

	// Create a subscriber.
	s := &Subscriber{
		conn:         conn,
		msgChan:      make(chan []byte),
		errChan:      make(chan error),
		ctx:          ctx,
		cancel:       cancel,
		wg:           sync.WaitGroup{},
		pongTimeout:  defaultPongTimeout,
		closeTimeout: defaultCloseTimeout,
	}

	// Add the options to the subscriber.
	for _, option := range options {
		option(s)
	}

	// Use the subscriber's wait group to keep track of the subscriber's long
	// running goroutines.
	s.wg.Add(1)

	// Start reading in another goroutine.
	go s.read()

	return s, httpRes, nil
}

// Close closes the subscriber's WebSocket connection and stops subscriber's long running goroutines.
func (p *Subscriber) Close() (err error) {

	// Cancel the subscriber's context.
	p.cancel()

	// Write a close WebSocket message to the Pub/Sub server.
	d := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
	writeErr := p.conn.WriteControl(websocket.CloseMessage, d, time.Now().Add(p.pongTimeout))
	if writeErr != nil && writeErr != websocket.ErrCloseSent {
		err = errors.Wrap(writeErr, "cannot write WebSocket close message")
	}

	// Create a timer to make sure that the rest of this function doesn't take
	// to long to finish.
	timer := time.NewTimer(p.closeTimeout)

	// Wait for the subscriber's long running goroutines to finish.
	go func() {
		p.wg.Wait()
		timer.Reset(0 * time.Second)
	}()

	// Wait for the timer to finish.
	<-timer.C

	// Close subscriber's WebSocket connection.
	if closeErr := p.conn.Close(); closeErr != nil && err == nil {
		err = errors.Wrap(closeErr, "cannot close WebSocket connection")
	}

	return
}

// Error return a channel for receving WebSocket errors from the Pub/Sub server.
func (p *Subscriber) Error() chan error {
	return p.errChan
}

// read is ment to run in it's own goroutine while it listens for incoming
// WebSocket messages from the Pub/Sub server until subscriber's context is canceled.
func (p *Subscriber) read() {

	// When this function finishes, cancel the subscriber's context. Then tell
	// the subscriber's wait group this function is finished.
	defer func() {
		p.cancel()
		p.wg.Done()
	}()

	for {
		select {

		// Wait for the subscriber's context to be done.
		case <-p.ctx.Done():
			return

		// Listen for incoming WebSocket messages form the Pub/Sub server.
		default:

			// Listen for incoming WebSocket messages form the Pub/Sub server.
			msgType, payload, err := p.conn.ReadMessage()
			if err != nil {
				p.errChan <- errors.Wrap(err, "cannot listen for WebSocket messages")
			}

			if msgType != websocket.BinaryMessage {

				// Send an error through the subscriber's error channel.
				p.errChan <- errors.Wrapf(ErrNotBinaryMessage, "got WebSocket message of type %v", msgType)
				continue
			}

			// Send the message through the subscriber's message channel.
			p.msgChan <- payload
		}
	}
}
