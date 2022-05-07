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

	ticker := time.NewTicker(2 * time.Second)

	for {
		select {
		case err := <-pub.Error():

			c := errors.Cause(err)
			if websocket.IsCloseError(c, websocket.CloseNormalClosure) {
				log.Println("Normal closure")
				return
			}

			log.Fatalf("Unexpected error from the publisher: %v", err)
		case <-ticker.C:
			if err := pub.Publish([]byte("Hello, World!")); err != nil {
				log.Fatalf("Cannot publish: %v", err)
				return
			}
		}
	}
}

func main() {

	url, ok := os.LookupEnv("URL")
	if !ok {
		log.Fatalf("URL must be set as an environment variable")
	}

	pub, _, err := publisher.NewPublisher(
		websocket.DefaultDialer,
		url,
		"example",
		http.Header{},
		publisher.WithPongTimeout(time.Second),
		publisher.WithCloseTimeout(2*time.Second),
	)

	if err != nil {
		log.Fatalf("Cannot create a publisher: %v", err)
		return
	}

	go publish(pub, []byte("Hello, World"))

	signals := make(chan os.Signal, 10)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	<-signals
}
