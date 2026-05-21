package cache

import (
	"errors"
	"fmt"

	"github.com/samber/oops"
)

func newError(message string) error {
	return wrapError(errors.New(message), message)
}

func newErrorf(format string, args ...any) error {
	return newError(fmt.Sprintf(format, args...))
}

func wrapError(err error, message string) error {
	if err == nil {
		return nil
	}
	return oops.In("cache").Wrapf(err, "%s", message)
}
