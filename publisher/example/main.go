package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"

	"github.com/Sidney-Bernardin/pubsub/publisher"
)

func main() {

	// Get the URL from an environment variable.
	url, ok := os.LookupEnv("URL")
	if !ok {
		log.Fatalf("URL must be set as an environment variable")
	}

	// Create a publisher.
	pub, _, err := publisher.NewPublisher(websocket.DefaultDialer,
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

	// Before returning, close the publisher.
	defer func() {
		if err := pub.Close(); err != nil {
			log.Fatalf("Cannot close publisher properly: %v", err) // With github.com/pkg/errors, you can use errors.Cause(err)
		}
	}()

	// Publish a message with the publisher.
	if err := pub.Publish([]byte("Hello, World!")); err != nil {
		log.Fatalf("Cannot publish: %v", err) // With github.com/pkg/errors, you can use errors.Cause(err)
	}

	// Forever read any errors that the publisher recives and log them.
	for {
		err := <-pub.Error()
		log.Fatalf("The publisher got an error: %v", err) // With github.com/pkg/errors, you can use errors.Cause(err)
	}
}
