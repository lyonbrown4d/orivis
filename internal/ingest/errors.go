package ingest

import (
	"fmt"

	"github.com/samber/oops"
)

var (
	ErrQueueFull = newError("ingest: result queue is full")
	ErrClosed    = newError("ingest: result ingestor is closed")
)

func newError(message string) error {
	return fmt.Errorf("%w", oops.New(message))
}

func wrapError(err error, message string) error {
	return oops.Wrapf(err, "%s", message)
}

func joinErrors(errs ...error) error {
	err := oops.Join(errs...)
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w", err)
}
