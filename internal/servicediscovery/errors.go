package servicediscovery

import (
	"errors"

	"github.com/samber/oops"
)

func newError(message string) error {
	return wrapError(errors.New(message), message)
}

func wrapError(err error, message string) error {
	if err == nil {
		return nil
	}
	return oops.In("servicediscovery").Wrapf(err, "%s", message)
}
