package pubsub

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gofrs/uuid"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

var (
	errMustBePubOrSub    = errors.New("action must be publish or subscribe")
	errAlreadySubscribed = errors.New("already subscribed to this topic")
)

type subscription struct {
	c     *client
	topic string
}

type client struct {
	id   string
	conn *websocket.Conn
}

func newClient(conn *websocket.Conn, name string) *client {
	return &client{
		id:   name + "-" + uuid.Must(uuid.NewV4()).String(),
		conn: conn,
	}
}

type handler struct {
	upgrader      *websocket.Upgrader
	logger        *zerolog.Logger
	mu            sync.Mutex
	clients       map[string]*client
	subscriptions map[string]*subscription
}

func NewHandler(upgrader *websocket.Upgrader, logger *zerolog.Logger) *handler {
	return &handler{
		upgrader:      upgrader,
		logger:        logger,
		mu:            sync.Mutex{},
		clients:       map[string]*client{},
		subscriptions: map[string]*subscription{},
	}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	// Upgrade the HTTP connection to the WebSocket protocol.
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Create a new client, then add it to the clients map.
	client := newClient(conn, r.Header.Get("name"))
	h.mu.Lock()
	h.clients[client.id] = client
	h.mu.Unlock()

	for {

		// Listen for inbound messages.
		_, p, err := conn.ReadMessage()
		if err != nil {

			// If it's a close error, then delete the client and close the connection.
			if _, ok := err.(*websocket.CloseError); ok {
				h.closeConn(conn, client.id, websocket.CloseNormalClosure, nil)
				return
			}

			// Send an internal server error.
			e := errors.New(err.Error())
			h.writeMsg(conn, client.id, nil, e, true)
			continue
		}

		// Decode the payload.
		var in inbound
		if err := json.Unmarshal(p, &in); err != nil {

			// Send the error.
			h.writeMsg(conn, client.id, nil, err, false)
			continue
		}

		switch in.Action {
		case "publish":
			h.logger.Info().Str("id", client.id).Str("topic", in.Topic).Msgf("New subscriber")
		case "subscribe":

			// Make sure the given ID is unique to the other subscribers.
			if _, ok := h.subscriptions[client.id]; ok {

				// Send an error.
				h.writeMsg(conn, client.id, nil, errAlreadySubscribed, false)
				continue
			}

			// Create and add a new subscriber to the subscribers map.
			h.mu.Lock()
			h.subscriptions[client.id] = &subscription{client, in.Topic}
			h.mu.Unlock()

			h.logger.Info().Str("id", client.id).Str("topic", in.Topic).Msgf("New subscriber")

		default:

			// Send an error.
			h.writeMsg(conn, client.id, nil, errMustBePubOrSub, false)
			continue
		}
	}
}

// send writes a binary message to the given WebSocket connection.
func (h *handler) writeMsg(conn *websocket.Conn, id string, msg []byte, e error, srvErr bool) {

	out := outbound{msg, outboundError{e.Error(), srvErr}}

	// If srvErr is true then the error and message should be logged and hidden
	// from the client.
	if e != nil && srvErr {
		h.logger.Warn().Stack().Err(e).Msg("Server error")
		out.Error.Message = "Internal server error"
		out.Message = nil
	}

	// Encode out.
	b, err := json.Marshal(out)
	if err != nil {

		// Send an internal server error.
		e = errors.New(err.Error())
		h.writeMsg(conn, id, nil, e, true)
		return
	}

	// Write to the WebSocket connection.
	if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {

		// If it's a close sent error, then delete the client and close the connection.
		if err == websocket.ErrCloseSent {
			h.closeConn(conn, id, websocket.CloseNormalClosure, nil)
			return
		}

		// Log as an internal server error.
		e = errors.New(err.Error())
		h.logger.Warn().Stack().Err(e).Msg("Server error")
	}
}

// closeConn closes the given WebSocket connection after sending a close
// message containing an optional error.
func (h *handler) closeConn(conn *websocket.Conn, id string, closeCode int, e error) {

	// If the client has a subscription, then remove it from the map.
	if _, ok := h.subscriptions[id]; ok {
		delete(h.subscriptions, id)
	}

	if e == nil {
		e = errors.New("")
	}

	// If an InternalServerErr is being sent then the error should be logged
	// and hidden from the client.
	if closeCode == websocket.CloseInternalServerErr {
		h.logger.Warn().Stack().Err(e).Msg("Server error")
		e = errors.New("Internal server error")
	}

	// Write a close message to the WebSocket connection.
	_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(closeCode, e.Error()), time.Now().Add(5*time.Second))
}
