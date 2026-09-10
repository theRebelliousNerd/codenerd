package broker

import (
	"context"
	"sync"
)

const (
	streamContentBuffer = 64
	streamErrBuffer     = 4
)

func closedStringChan() <-chan string {
	ch := make(chan string)
	close(ch)
	return ch
}

func errChanWith(err error) <-chan error {
	ch := make(chan error, 1)
	ch <- err
	close(ch)
	return ch
}

// proxyStream forwards a streamed turn and settles its receipt when the stream
// actually ends.
//
// A streamed call returns immediately; the provider reports usage as the stream
// progresses. Settling at return time would record every streamed turn as
// costing nothing, which is most of the traffic in chat mode.
//
// Both forwarders select on ctx.Done and then drain their input. Draining looks
// redundant — a cancelled context usually stops the producer too — but "usually"
// is how goroutine leaks are written. A consumer that abandons a stream mid-turn
// (the user hits Ctrl+C, a timeout fires) would otherwise block the forwarder on
// a send forever, holding the observer, the receipt, and the underlying client's
// goroutine alive for the life of the process.
func (c *core) proxyStream(
	ctx context.Context,
	receipt Receipt,
	req *Request,
	obs *callObserver,
	content <-chan string,
	errs <-chan error,
) (<-chan string, <-chan error) {
	outContent := make(chan string, streamContentBuffer)
	outErrs := make(chan error, streamErrBuffer)

	var (
		mu        sync.Mutex
		streamErr error
		wg        sync.WaitGroup
	)

	recordErr := func(err error) {
		if err == nil {
			return
		}
		mu.Lock()
		if streamErr == nil {
			streamErr = err
		}
		mu.Unlock()
	}

	wg.Add(2)

	go func() {
		defer wg.Done()
		defer close(outContent)
		for {
			select {
			case chunk, ok := <-content:
				if !ok {
					return
				}
				select {
				case outContent <- chunk:
				case <-ctx.Done():
					drainStrings(content)
					return
				}
			case <-ctx.Done():
				drainStrings(content)
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		defer close(outErrs)
		for {
			select {
			case err, ok := <-errs:
				if !ok {
					return
				}
				recordErr(err)
				select {
				case outErrs <- err:
				case <-ctx.Done():
					drainErrors(errs, recordErr)
					return
				}
			case <-ctx.Done():
				drainErrors(errs, recordErr)
				return
			}
		}
	}()

	// Settlement is deliberately after both forwarders have returned, and each
	// forwarder closes its output channel before returning. A consumer therefore
	// observes the stream close BEFORE the receipt exists.
	//
	// That ordering is the right one -- the receipt records the true end of the
	// turn, including any error that arrived last -- but it has a consequence
	// worth stating: nothing may assume a receipt is present the instant a
	// stream closes. A reader that needs one has to wait for it, and a process
	// exiting the moment its last stream closes can lose that final receipt.
	go func() {
		wg.Wait()
		mu.Lock()
		final := streamErr
		mu.Unlock()
		c.settle(receipt, req, obs, nil, final)
	}()

	return outContent, outErrs
}

// drainStrings consumes the remainder of ch so its producer is never blocked on
// a send that nobody will receive.
func drainStrings(ch <-chan string) {
	for range ch {
	}
}

// drainErrors consumes the remainder of ch, keeping the first error seen so a
// failure that arrives after cancellation still reaches the receipt.
func drainErrors(ch <-chan error, record func(error)) {
	for err := range ch {
		record(err)
	}
}
