package turnjudge

// inBatches runs fn over items in consecutive batches of at most size, in
// parallel, and returns the batch results in item order. Any failed batch
// fails the whole: a partial verdict never reorders context, flags a review or
// splits a problem list. A size below one is one item per batch.
func inBatches[T, R any](items []T, size int, fn func(batch []T) (R, error)) ([]R, error) {
	size = max(size, 1)
	type part struct {
		out R
		err error
	}
	var parts []chan part
	for start := 0; start < len(items); start += size {
		batch := items[start:min(start+size, len(items))]
		ch := make(chan part, 1)
		parts = append(parts, ch)
		go func() {
			out, err := fn(batch)
			ch <- part{out, err}
		}()
	}
	outs := make([]R, 0, len(parts))
	var firstErr error
	for _, ch := range parts {
		p := <-ch
		if p.err != nil && firstErr == nil {
			firstErr = p.err
		}
		outs = append(outs, p.out)
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return outs, nil
}
