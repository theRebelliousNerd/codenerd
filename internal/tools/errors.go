package tools

import "errors"

// Tool registry errors.
var (
	// ErrToolNotFound is returned when a tool is not registered.
	ErrToolNotFound = errors.New("tool not found")

	// ErrToolNameEmpty is returned when a tool has no name.
	ErrToolNameEmpty = errors.New("tool name cannot be empty")

	// ErrToolExecuteNil is returned when a tool has no execute function.
	ErrToolExecuteNil = errors.New("tool execute function cannot be nil")

	// ErrToolAlreadyRegistered is returned when registering a duplicate.
	ErrToolAlreadyRegistered = errors.New("tool already registered")

	// ErrMissingRequiredArg is returned when a required argument is missing.
	ErrMissingRequiredArg = errors.New("missing required argument")

	// ErrInvalidArgType is returned when an argument has the wrong type.
	ErrInvalidArgType = errors.New("invalid argument type")
)
