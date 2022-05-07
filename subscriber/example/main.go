package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Sidney-Bernardin/pubsub/subscriber"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

func listen(sub *subscriber.Subscriber, msgChan chan []byte) {

	defer func() {
		if err := sub.Close(); err != nil {
			log.Fatalf("Cannot close subscriber properly: %v", err)
		}
	}()

	for {
		select {
		case msg := <-msgChan:
			log.Printf("Got message: %s\n", msg)
		case err := <-sub.Error():

			c := errors.Cause(err)
			if websocket.IsCloseError(c, websocket.CloseNormalClosure) {
				log.Printf("Subscriber closed normally")
				return
			}

			log.Fatalf("Error from the subscriber: %v", err)
		}
	}
}

func main() {

	url, ok := os.LookupEnv("URL")
	if !ok {
		log.Fatalf("URL must be set as an environment variable")
	}

	dialer := websocket.DefaultDialer
	topic := "example"
	h := http.Header{}
	msgChan := make(chan []byte)

	sub, _, err := subscriber.NewSubscriber(dialer, url, topic, h,
		subscriber.WithMessageChan(msgChan),
		subscriber.WithPongTimeout(time.Second),
		subscriber.WithCloseTimeout(2*time.Second))

	if err != nil {
		log.Fatalf("Cannot create a subscriber: %v", err)
		return
	}

	go listen(sub, msgChan)

	signals := make(chan os.Signal, 10)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	s := <-signals

	log.Printf("Smooth shutdown: %s", s.String())
}
