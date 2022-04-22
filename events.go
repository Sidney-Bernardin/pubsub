package pubsub

const (
	EventTypeNewPublisher        = 0
	EventTypeNewSubscriber       = 1
	EventTypeInternalServerError = 2
)

type Event struct {
	ClientID   string
	Topic      string
	ClientType string
	Error      error
	EventType  int
}
