//go:build !unix

package main

import (
	"context"
	"io"
)

// Platforms without the Unix poll/read path keep the prior close-on-cancel
// behavior. The inherited-descriptor cancellation guarantee is currently
// verified on Unix; it still needs a platform-specific implementation and
// test before being claimed on Windows.
func newChatInputReader(_ context.Context, input io.ReadCloser) (io.Reader, bool) {
	return input, true
}
