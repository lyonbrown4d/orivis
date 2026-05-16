package retention

import (
	"fmt"

	"github.com/samber/oops"
)

func newError(message string) error {
	return fmt.Errorf("%w", oops.New(message))
}

func wrapError(err error, message string) error {
	return oops.Wrapf(err, "%s", message)
}
