# pubsub

A WebSocket based Pub/Sub library for Go.

## How to use

#### Create a Pub/Sub server
By using the pubsub.PubSub handler, you can create a Pub/Sub service thit will store the publishers, subscribers, topics, etc in-memory. When errors accer, they will be sent through an event channel so that you have control over how you log any errors.

Here is a snippet of the pubsub example code:
``` go
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
  
  ...
}
```

#### Create a subscriber
To create a subscriber just use the subscriber.NewSubscriber constructor. To get the messages that the subscriber recives you just listen to a message channel that you give to the subscriber.

Here is a snippet of the subscriber example code:
``` go
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

  ...
}
```

#### Create a pulisher
To create a publisher just use the publisher.NewPublisher constructor. Publish a message by calling the Publish method on the the publisher.

Here is a snippet of the publisher example code:
``` go
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
	s := <-signals
  
  ...
}
```
