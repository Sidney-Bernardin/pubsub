package pubsub

const (
	EventTypeNewPublisher        = 0
	EventTypeNewSubscriber       = 1
	EventTypePublisherLeft       = 2
	EventTypeSubscriberLeft      = 3
	EventTypeInternalServerError = 4
)

var eventTypeMessages = map[int]string{
	0: "new publisher",
	1: "new subscriber",
	2: "publisher left",
	3: "subscriber left",
	4: "internal server error",
}

type Event struct {
	ClientID   string
	Topic      string
	ClientType string
	Error      error
	EventType  int
}

func (e *Event) Msg() (ret string) {
	return eventTypeMessages[e.EventType]
}
