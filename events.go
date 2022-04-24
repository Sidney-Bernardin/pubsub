package pubsub

const (
	EventTypeNewPublisher        = 0
	EventTypeNewSubscriber       = 1
	EventTypePublisherLeft       = 2
	EventTypeSubscriberLeft      = 3
	EventTypeInternalServerError = 4
)

type Event struct {
	ClientID   string
	Topic      string
	ClientType string
	Error      error
	EventType  int
}
