package main

import (
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/Sidney-Bernardin/pubsub"
)

var port = flag.String("port", "8080", "What port to serve on.")

func main() {

	flag.Parse()

	// Setup a new logger.
	logger := zerolog.New(os.Stdout)
	logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Create a new mux router.
	r := mux.NewRouter()

	// Use the router to setup a new route for the pubsub handler.
	events := make(chan pubsub.Event)
	r.Handle("/{topic_name}/{client_type}", pubsub.Handler(&websocket.Upgrader{}, events))

	// Run the server in a separate go routine.
	go func() {
		logger.Info().Msgf("Listening on :%s ...", *port)
		if err := http.ListenAndServe(":"+*port, r); err != nil {
			logger.Fatal().Err(err).Msg("Cannot run server")
		}
	}()

	// Create and setup a new channel to interrupt and terination signals.
	signals := make(chan os.Signal, 10)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	// Forever listen for inbound events from the events channel until a signal is recived.
	for {
		select {
		case <-signals:
			return
		case e := <-events:

			var logEvent *zerolog.Event
			var msg string

			// Configure log based on the type of the event.
			switch e.EventType {
			case pubsub.EventTypeNewPublisher:
				logEvent = logger.Info()
				msg = "New publisher"
			case pubsub.EventTypeNewSubscriber:
				logEvent = logger.Info()
				msg = "New subscriber"
			case pubsub.EventTypePublisherLeft:
				logEvent = logger.Info()
				msg = "Publisher left"
			case pubsub.EventTypeSubscriberLeft:
				logEvent = logger.Info()
				msg = "Subscriber left"
			case pubsub.EventTypeInternalServerError:
				logEvent = logger.Warn().Err(e.Error)
				msg = "Internal Server Error"
			}

			logEvent.
				Str("id", e.ClientID).
				Str("topic", e.Topic).
				Str("client_type", e.ClientType).
				Str("client_type", e.ClientType).
				Msg(msg)
		}
	}
}
