package discovery

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
	return oops.In("discovery").Wrapf(err, "%s", message)
}

func wrapErrorf(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	return oops.In("discovery").Wrapf(err, format, args...)
}
