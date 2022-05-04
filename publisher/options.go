package publisher

import (
	"time"
)

type Option func(*publisher)

const (
	defaultPongTimeout  = time.Second
	defaultCloseTimeout = 3 * time.Second
)

func WithPongTimeout(timeout time.Duration) Option {
	return func(p *publisher) {
		p.pongTimeout = timeout
	}
}

func WithCloseTimeout(timeout time.Duration) Option {
	return func(p *publisher) {
		p.closeTimeout = timeout
	}
}
