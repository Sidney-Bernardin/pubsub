package pshandler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gofrs/uuid"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/Sidney-Bernardin/pshandler/pubsub"
)

var (
	errMustBePubOrSub = errors.New("action must be publish or subscribe")
)

type Client struct {
	id   string
	conn *websocket.Conn
}

func (c *Client) GetID() string {
	return c.id
}

type handler struct {
	upgrader *websocket.Upgrader
	pubsub   *pubsub.Pubsub
	logger   *zerolog.Logger
	mu       sync.Mutex
	clients  map[string]*Client
}

func NewHandler(upgrader *websocket.Upgrader, pubsub *pubsub.Pubsub, logger *zerolog.Logger) *handler {
	return &handler{upgrader, pubsub, logger, sync.Mutex{}, map[string]*Client{}}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	// Upgrade the HTTP connection to the WebSocket protocol.
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create and add a new wsClient to the clients map.
	client := Client{uuid.Must(uuid.NewV4()).String(), conn}
	h.mu.Lock()
	h.clients[client.id] = &client
	h.mu.Unlock()

	for {

		// Listen for inbound messages with the connection.
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

		// Convert the payload into an inbound struct.
		var in inbound
		if err := json.Unmarshal(p, &in); err != nil {

			// Send a client error.
			h.writeMsg(conn, client.id, nil, err, false)
			continue
		}

		switch in.Action {
		case "publish":
			fmt.Println("published")
		case "subscribe":
			fmt.Println("subscribe")
		default:

			// Send a client error.
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

	// Convert out into bytes.
	b, err := json.Marshal(out)
	if err != nil {

		// Delete the client and close the connection with an internal server error.
		e = errors.New(err.Error())
		h.closeConn(conn, id, websocket.CloseInternalServerErr, e)
		return
	}

	// Write out to the WebSocket connection.
	if err := conn.WriteMessage(websocket.BinaryMessage, b); err != nil {

		// If it's a close error, then delete the client and close the connection.
		if _, ok := err.(*websocket.CloseError); ok {
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

	defer conn.Close()

	// If the given client is subscribed to a topic, then delete it.
	h.pubsub.RemoveSubscriber(id)

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
	err := conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(closeCode, e.Error()), time.Now().Add(5*time.Second))
	if err != nil {

		// If it's a close error, then delete the client and close the connection.
		if _, ok := err.(*websocket.CloseError); ok {
			h.logger.Warn().Stack().Err(e).Msg("Server error")
			return
		}

		// Log as an internal server error.
		e = errors.New(err.Error())
		h.logger.Warn().Stack().Err(e).Msg("Server error")
	}
}
