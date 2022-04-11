package main

import (
	"flag"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"

	handler "github.com/Sidney-Bernardin/pubsub"
)

var port = flag.String("port", "8080", "What port to serve on.")

func main() {

	flag.Parse()

	// Setup a new logger.
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	l := zerolog.New(os.Stdout)
	l = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Setup a new pubsub handler.
	pubsubHandler := handler.NewHandler(
		&websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		&l,
	)

	// Create a new router.
	mux := http.NewServeMux()

	// Make a new route with pubsubHandler.
	mux.Handle("/pubsub", pubsubHandler)

	// Start the server.
	l.Info().Msgf("Serving on :%s ...", *port)
	if err := http.ListenAndServe(":"+*port, mux); err != nil {
		l.Err(err).Msg("Server stoped serving")
	}
}
