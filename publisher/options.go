package publisher

import (
	"time"
)

type Option func(*Publisher)

const (
	defaultPongTimeout  = time.Second
	defaultCloseTimeout = 3 * time.Second
)

func WithPongTimeout(timeout time.Duration) Option {
	return func(p *Publisher) {
		p.pongTimeout = timeout
	}
}

func WithCloseTimeout(timeout time.Duration) Option {
	return func(p *Publisher) {
		p.closeTimeout = timeout
	}
}
