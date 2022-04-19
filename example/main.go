package main

import (
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"

	"github.com/Sidney-Bernardin/pubsub"
)

var port = flag.String("port", "8080", "What port to serve on.")

func main() {

	flag.Parse()

	// Create and setup a new logger.
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	logger := zerolog.New(os.Stdout)
	logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Create and setup a new pubsub handler.
	h := pubsub.NewHandler(&websocket.Upgrader{}, &logger)
	http.Handle("/", h)

	// Create and setup a new channel to interrupt and terination signals.
	signals := make(chan os.Signal, 10)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	// Run the server in a separate go routine.
	go func() {
		logger.Info().Msgf("Listening on :%s ...", *port)
		if err := http.ListenAndServe(":"+*port, nil); err != nil {
			logger.Fatal().Err(err).Msg("Server stoped listening")
		}
	}()

	// Wait for a signal to come through the signals channel before returning.
	<-signals
	logger.Info().Msg("Closed properly")
	return
}
