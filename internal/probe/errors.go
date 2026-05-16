package probe

import (
	"fmt"

	"github.com/samber/oops"
)

func newError(message string) error {
	return fmt.Errorf("%w", oops.New(message))
}

func errorf(format string, args ...any) error {
	return newError(fmt.Sprintf(format, args...))
}

func wrapError(err error, message string) error {
	return oops.Wrapf(err, "%s", message)
}
