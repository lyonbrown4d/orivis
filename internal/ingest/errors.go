package ingest

import (
	"errors"

	"github.com/samber/oops"
)

var (
	ErrQueueFull = newError("ingest: result queue is full")
	ErrClosed    = newError("ingest: result ingestor is closed")
)

func newError(message string) error {
	return wrapError(errors.New(message), message)
}

func wrapError(err error, message string) error {
	return oops.In("ingest").Wrapf(err, "%s", message)
}

func joinErrors(errs ...error) error {
	err := oops.Join(errs...)
	if err == nil {
		return nil
	}
	return wrapError(err, "join ingest errors")
}
