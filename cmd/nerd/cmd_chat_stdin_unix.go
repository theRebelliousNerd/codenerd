//go:build unix

package main

import (
	"context"
	"errors"
	"io"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

const chatInputPollInterval = 25 * time.Millisecond

// newChatInputReader makes inherited file-descriptor input cancellable without
// closing it or changing its shared open-file flags. Non-file readers retain
// the close-on-cancel behavior used by the chat lifecycle's owned readers.
func newChatInputReader(ctx context.Context, input io.ReadCloser) (io.Reader, bool) {
	file, ok := input.(*os.File)
	if !ok || file == nil {
		return input, true
	}
	return &pollingChatFileReader{ctx: ctx, file: file}, false
}

// pollingChatFileReader waits for descriptor readability before reading. A
// blocking read on an inherited pipe cannot be interrupted by closing another
// os.File handle for the same descriptor, so polling keeps the read on the
// command goroutine and lets context cancellation return without a stranded
// worker or descriptor-flag changes.
type pollingChatFileReader struct {
	ctx  context.Context
	file *os.File
}

func (r *pollingChatFileReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for {
		if err := r.ctx.Err(); err != nil {
			return 0, err
		}

		rawFD := r.file.Fd()
		if rawFD == ^uintptr(0) {
			return 0, os.ErrClosed
		}
		fd := int(rawFD)
		pollFDs := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		_, err := unix.Poll(pollFDs, int(chatInputPollInterval/time.Millisecond))
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return 0, err
		}
		revents := pollFDs[0].Revents
		if revents == 0 {
			continue
		}
		if revents&unix.POLLNVAL != 0 {
			return 0, unix.EBADF
		}
		if revents&(unix.POLLIN|unix.POLLHUP|unix.POLLERR) == 0 {
			continue
		}

		n, err := unix.Read(fd, p)
		if errors.Is(err, unix.EINTR) || errors.Is(err, unix.EAGAIN) {
			continue
		}
		if n == 0 && err == nil {
			return 0, io.EOF
		}
		return n, err
	}
}
