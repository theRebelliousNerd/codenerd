//go:build integration

package e2e_test

import (
	"errors"

	"codenerd/internal/types"
)

// rejection is a post-action validation's rejection as an error, and nil for
// every other state. ValidateInteractiveToolResult returns a typed state
// (verified, rejected, incomplete, unvalidated, not_run); what these gate tests
// assert is whether the gate rejected the result, which is the one state the
// session surfaces as a tool error.
func rejection(v types.ToolValidation) error {
	if v.State == types.ToolValidationRejected {
		return errors.New("validation failed: " + v.Reason)
	}
	return nil
}
