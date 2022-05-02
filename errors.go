package pubsub

import "github.com/pkg/errors"

var (
	errMustBePubOrSub = errors.New("client_type must be publisher or subscriber")
)
