package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"

	"github.com/Sidney-Bernardin/pubsub/publisher"
)

func publish(pub *publisher.Publisher, msg []byte) {

	defer func() {
		if err := pub.Close(); err != nil {
			log.Fatalf("Cannot close publisher properly: %v", err)
		}
	}()

	if err := pub.Publish([]byte("Hello, World!")); err != nil {
		log.Fatalf("Cannot publish: %v", err)
	}

	for {
		select {
		case err := <-pub.Error():

			c := errors.Cause(err)
			if websocket.IsCloseError(c, websocket.CloseNormalClosure) {
				log.Println("Normal closure")
				return
			}

			log.Fatalf("Unexpected error from the publisher: %v", err)
		}
	}
}

func main() {

	// Get the URL from an environment variable.
	url, ok := os.LookupEnv("URL")
	if !ok {
		log.Fatalf("URL must be set as an environment variable")
	}

	// Create a publisher.
	pub, _, err := publisher.NewPublisher(
		websocket.DefaultDialer,
		url,
		"example",
		http.Header{},
		publisher.WithPongTimeout(time.Second),
		publisher.WithCloseTimeout(2*time.Second),
	)

	// Handle the error.
	if err != nil {
		log.Fatalf("Cannot create a publisher: %v", err) // With github.com/pkg/errors, you can use errors.Cause(err)
		return
	}

	go publish(pub, []byte("Hello, World"))

	// When an OS signal is sent through the signals channel, return.
	signals := make(chan os.Signal, 10)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	<-signals
}
