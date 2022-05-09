package publisher

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

// Publisher publishes to the Pub/Sub server.
type Publisher struct {
	wg   sync.WaitGroup
	conn *websocket.Conn

	// WebSocket errors from the Pub/Sub server are sent through errChan.
	errChan chan error

	// Context with a cancel function helps shutdown the publisher's long
	// running goroutines.
	ctx    context.Context
	cancel context.CancelFunc

	// Timeouts.
	pongTimeout  time.Duration
	closeTimeout time.Duration
}

// NewPublisher returns a pointer to a Publisher.
func NewPublisher(dialer *websocket.Dialer, addr, topic string, headers http.Header, options ...Option) (*Publisher, *http.Response, error) {

	// Set the client_type and topic headers.
	headers.Set("client_type", "publisher")
	headers.Set("topic", topic)

	// Connect to the Pub/Sub server.
	conn, httpRes, err := dialer.Dial(addr, headers)
	if err != nil {
		return nil, httpRes, errors.Wrap(err, "cannot dial Pub/Sub server")
	}

	// Create a cancel context for the publisher.
	ctx, cancel := context.WithCancel(context.Background())

	// Create a publisher.
	p := &Publisher{
		conn:         conn,
		errChan:      make(chan error),
		ctx:          ctx,
		cancel:       cancel,
		wg:           sync.WaitGroup{},
		pongTimeout:  defaultPongTimeout,
		closeTimeout: defaultCloseTimeout,
	}

	// Add the options to the publisher.
	for _, option := range options {
		option(p)
	}

	// Use the publisher's wait group to keep track of the publisher's long
	// running goroutines.
	p.wg.Add(1)

	// Start reading in another goroutine.
	go p.read()

	return p, httpRes, nil
}

// Close closes the publisher's WebSocket connection and stops publisher's long running goroutines.
func (p *Publisher) Close() (err error) {

	// Cancel the publisher's context.
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

	// Wait for the publisher's long running goroutines to finish.
	go func() {
		p.wg.Wait()
		timer.Reset(0 * time.Second)
	}()

	// Wait for the timer to finish.
	<-timer.C

	// Close the publisher's WebSocket connection.
	if closeErr := p.conn.Close(); closeErr != nil && err == nil {
		err = errors.Wrap(closeErr, "cannot close WebSocket connection")
	}

	return
}

// Error return a channel for receving WebSocket errors from the Pub/Sub server.
func (p *Publisher) Error() chan error {
	return p.errChan
}

// Publish publishes the given message to the Pub/Sub server.
func (p *Publisher) Publish(msg []byte) error {
	return errors.Wrap(p.conn.WriteMessage(websocket.BinaryMessage, msg), "cannot write WebSocket message")
}

// read is ment to run in it's own goroutine while it listens for incoming
// WebSocket messages from the Pub/Sub server until publisher's context is canceled.
func (p *Publisher) read() {

	// When this function finishes, cancel the publisher's context. Then tell
	// the publisher's wait group this function is finished.
	defer func() {
		p.cancel()
		p.wg.Done()
	}()

	for {
		select {

		// Wait for the publisher's context to be done.
		case <-p.ctx.Done():
			return

		// Listen for incoming WebSocket messages form the Pub/Sub server.
		default:

			// Listen for incoming WebSocket messages form the Pub/Sub server.
			_, _, err := p.conn.ReadMessage()
			if err != nil {

				// Send the error through the publisher's error channel.
				p.errChan <- errors.Wrap(err, "cannot listen for WebSocket messages")
			}
		}
	}
}
