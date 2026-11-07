package tools

import (
	"errors"
	"fmt"
	"io/fs"
)

// RefusalError is a tool's deliberate refusal of what a call proposed: a guard
// that keeps the workspace whole rejected the content (an edit that would not
// parse, one that drops a delimiter, a delete whose uses remain). The reason
// goes back to the model, which revises the call; the guard worked.
type RefusalError struct{ err error }

func (e *RefusalError) Error() string { return e.err.Error() }
func (e *RefusalError) Unwrap() error { return e.err }

// Refuse formats a refusal as fmt.Errorf does.
func Refuse(format string, args ...any) error {
	return &RefusalError{fmt.Errorf(format, args...)}
}

// ModelFacing reports whether a tool call failed on what the call itself
// asked for: a refusal of its content, a path it named that does not exist,
// an argument missing or of the wrong type, a tool or path it may not use.
// The model reads the error and re-plans; nothing in codeNERD is broken, so
// such a failure is no defect to surface (debug-self reads ERROR as one).
func ModelFacing(err error) bool {
	var refusal *RefusalError
	return errors.As(err, &refusal) ||
		errors.Is(err, fs.ErrNotExist) ||
		errors.Is(err, ErrMissingRequiredArg) ||
		errors.Is(err, ErrInvalidArgType) ||
		errors.Is(err, ErrToolNotAllowed) ||
		errors.Is(err, ErrPathOutsideWorkspace) ||
		errors.Is(err, ErrToolNotFound)
}
