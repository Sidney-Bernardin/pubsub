package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/Sidney-Bernardin/pubsub"
)

const (
	pongTimeout = time.Second
)

func main() {

	// Create a logger.
	logger := zerolog.New(os.Stdout)
	logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Create a router.
	r := mux.NewRouter()

	// Create a Pub/Sub handler.
	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan *pubsub.Event)
	r.Handle("/", pubsub.PubSub(ctx, &websocket.Upgrader{}, events, pongTimeout))

	// Get the optional port from an environment variable.
	port, ok := os.LookupEnv("PORT")
	if !ok {
		port = "8080"
	}

	// Run an HTTP server in another goroutine.
	go func() {

		logger.Info().Msgf("Listening on :%s ...", port)

		// Listen and serve.
		if err := http.ListenAndServe(":"+port, r); err != nil {
			logger.Fatal().Err(err).Msg("Cannot run server")
		}
	}()

	// Create a channel for OS signals.
	signals := make(chan os.Signal, 10)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	for {
		select {

		// When an OS signal is sent through the signals channel, return.
		case s := <-signals:
			cancel()
			logger.Info().Str("signal", s.String()).Msgf("Smooth shutdown")
			return

		// When an event is sent through the events channel, log the event.
		case e := <-events:

			var logEvent *zerolog.Event

			// Set logEvent based on it's event type.
			switch e.EventType {

			// Log with a serverity of warning, and give the log event the error.
			case pubsub.EventTypeInternalServerError:
				logEvent = logger.Warn().Err(e.Error)

			// Log with a serverity of info.
			default:
				logEvent = logger.Info()
			}

			logEvent.
				Str("id", e.ClientID).
				Str("topic", e.Topic).
				Str("client_type", e.ClientType).
				Msg(e.Msg())
		}
	}
}
