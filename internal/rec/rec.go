// Package rec provides utilities for recovering from panics and wrapping errors.
package rec

import (
	"fmt"
	"runtime/debug"
)

func rec() error {
	if r := recover(); r != nil {
		switch t := r.(type) {
		case error:
			return fmt.Errorf("recovered panic: %w\n%s", t, debug.Stack())
		default:
			return fmt.Errorf("recovered panic: %v\n%s", r, debug.Stack())
		}
	}
	return nil
}

// Error recovers a panic and assigns it to the provided error.
func Error(err *error) {
	if r := rec(); r != nil {
		*err = r
	}
}

// Wrap recovers a panic with the provided format and arguments
// and assigns it to the provided error.
// The recovered panic is appended to the end of the arguments.
// If no panic was recovered, but the error is not nil, it is wrapped
// with the provided format and arguments as well.
func Wrap(err *error, format string, a ...any) {
	if r := rec(); r != nil {
		*err = fmt.Errorf(format, append(a, r)...)
	} else if *err != nil {
		*err = fmt.Errorf(format, append(a, *err)...)
	}
}
