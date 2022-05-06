package publisher

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

// Publisher handles publishing to and reading from a WebSocket based Pub/Sub server.
type Publisher struct {
	conn         *websocket.Conn
	errChan      chan error
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	pongTimeout  time.Duration
	closeTimeout time.Duration
}

// NewPublisher return a pointer to a publisher.
func NewPublisher(dialer *websocket.Dialer, addr, topic string, headers http.Header, options ...Option) (*Publisher, *http.Response, error) {

	// Set the client_type header to publisher.
	headers.Set("client_type", "publisher")
	headers.Set("topic", topic)

	// Dial the Pub/Sub server with the given address and topic and headers.
	conn, httpRes, err := dialer.Dial(addr, headers)
	if err != nil {
		return nil, httpRes, errors.Wrap(err, "cannot dial Pub/Sub server")
	}

	// Create a publisher, and a cancel context to go with it.
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

	// Load the options into the server.
	for _, option := range options {
		option(p)
	}

	// Keep track of all goroutines with the publisher's wait group.
	p.wg.Add(1)

	// Start reading from the Pub/Sub server in another goroutine.
	go p.read()

	return p, httpRes, nil
}

// Close stops all of the publisher's long running goroutines, then closes the
// publisher's WebSocket connection.
func (p *Publisher) Close() (err error) {

	// Cancel the publisher's context.
	p.cancel()

	// Write a WebSocket close message to the Pub/Sub server.
	d := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
	writeErr := p.conn.WriteControl(websocket.CloseMessage, d, time.Now().Add(p.pongTimeout))
	if writeErr != nil && writeErr != websocket.ErrCloseSent {
		err = errors.Wrap(writeErr, "cannot write WebSocket close message")
	}

	// Start a timer for p.closeTimeout to make sure the publisher's goroutines
	// arent taking to long to finish.
	timer := time.NewTimer(p.closeTimeout)

	// Wait for the goroutines to finish is another goroutine. If the
	// goroutines finish before the timer finishes, make the timer to have 0
	// seconds left.
	go func() {
		p.wg.Wait()
		timer.Reset(0 * time.Second)
	}()

	// Wait for the timer to finish.
	<-timer.C

	// Close the WebSocket connection.
	if closeErr := p.conn.Close(); closeErr != nil && err == nil {
		err = errors.Wrap(closeErr, "cannot close WebSocket connection")
	}

	return
}

// Error returns the publisher's error channel.
func (p *Publisher) Error() chan error {
	return p.errChan
}

// Publish publishes the given message to the Pub/Sub server.
func (p *Publisher) Publish(msg []byte) error {
	return errors.Wrap(p.conn.WriteMessage(websocket.BinaryMessage, msg), "cannot write WebSocket message")
}

// read forever listens for WebSocket messages from the Pub/Sub server.
func (p *Publisher) read() {

	// Before returning, tell the publisher's wait group that this goroutine is finished.
	defer func() {
		p.cancel()
		p.wg.Done()
	}()

	for {
		select {

		// When the publisher's context is done, return.
		case <-p.ctx.Done():
			return

		// Forever listen for WebSocket messages from the Pub/Sub server.
		default:
			for {

				// Listen for WebSocket messages from the client.
				_, _, err := p.conn.ReadMessage()
				if err != nil {

					// Send the error through the publisher's error channel.
					p.errChan <- errors.Wrap(err, "cannot listen for WebSocket messages")
				}
			}
		}
	}
}
