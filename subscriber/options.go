package subscriber

import (
	"time"
)

type Option func(*Subscriber)

const (
	defaultPongTimeout  = time.Second
	defaultCloseTimeout = 3 * time.Second
)

func WithMessageChan(msgChan chan []byte) Option {
	return func(p *Subscriber) {
		p.msgChan = msgChan
	}
}

func WithPongTimeout(timeout time.Duration) Option {
	return func(p *Subscriber) {
		p.pongTimeout = timeout
	}
}

func WithCloseTimeout(timeout time.Duration) Option {
	return func(p *Subscriber) {
		p.closeTimeout = timeout
	}
}
