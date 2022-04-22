package pubsub

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func startTestServer(t *testing.T, h http.HandlerFunc, topic, clientType string) (*httptest.Server, *http.Response, *websocket.Conn) {

	t.Helper()

	srv := httptest.NewServer(h)

	url := fmt.Sprintf(
		"ws%s/%s/%s",
		strings.TrimPrefix(srv.URL, "http"),
		topic,
		clientType)

	conn, httpRes, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("Cannot create new test server: %v", err)
	}

	return srv, httpRes, conn
}

func TestHandler(t *testing.T) {

	tt := []struct {
		name string

		topic      string
		clientType string

		expectedWSCode   int
		expectedWSErr    string
		expectedHTTPCode int
		expectedHTTPErr  string
	}{}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			h := Handler(&websocket.Upgrader, l)

			startTestServer(t, h, tc.topic, tc.clientType)
		})
	}
}
