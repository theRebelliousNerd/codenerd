package broker

import (
	"context"

	"codenerd/internal/types"
)

// proxyStream forwards a streamed turn into w and settles its receipt when the
// stream actually ends.
//
// A streamed call returns immediately; the provider reports usage as the stream
// progresses. Settling at return time would record every streamed turn as
// costing nothing, which is most of the traffic in chat mode.
//
// The receipt is settled BEFORE the terminal result is published. A consumer
// that has observed the terminal therefore knows the call's accounting is
// complete; channel closure alone never meant that, and a process exiting the
// moment its last stream closed used to lose that stream's final receipt. The
// terminal's usage is response-carried evidence for settle, never a second
// charge.
//
// RelayStream drains the source even after cancellation, so the producer is
// never left blocked on a send and the usage it observed before stopping still
// reaches the receipt.
func (c *core) proxyStream(
	ctx context.Context,
	w *types.StreamWriter,
	receipt Receipt,
	req *Request,
	obs *callObserver,
	content, thoughts <-chan string,
	done <-chan types.StreamResult,
) {
	if obs.recording() {
		obs.stream = &streamText{}
		content = tee(content, obs.stream.addText)
		thoughts = tee(thoughts, obs.stream.addThought)
	}
	go func() {
		defer w.Close()
		res, _ := types.RelayStream(ctx, w, content, thoughts, done)
		obs.streamStop = res.StopReason
		var reported *types.UsageMetadata
		if responseReported(res.Usage) {
			u := res.Usage
			reported = &u
		}
		c.settle(receipt, req, obs, reported, res.Err)
		w.Finish(res)
	}()
}
