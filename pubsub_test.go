package pubsub

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
)

func logEvents(t *testing.T, events chan Event) {

	t.Helper()

	l := zerolog.New(os.Stdout)
	for {
		e := <-events

		if e.EventType == EventTypeInternalServerError {
			l.Warn().Err(e.Error).
				Str("id", e.ClientID).
				Str("topic", e.Topic).
				Str("client_type", e.ClientType).
				Str("client_type", e.ClientType).
				Msg("Internal Server Error")
		}
	}
}
