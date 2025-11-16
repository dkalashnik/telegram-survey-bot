package fsm

import (
	"errors"

	"github.com/looplab/fsm"
)

func isNoTransitionError(err error) bool {
	if err == nil {
		return false
	}
	var noTransitionError fsm.NoTransitionError
	return errors.As(err, &noTransitionError)
}
