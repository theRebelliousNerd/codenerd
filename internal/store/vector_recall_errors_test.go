package store

import "testing"

// A semantic recall whose SQL fails while it is being stepped is a failed
// search, not an empty one. vectorRecallVec checked the Query error, then
// looped over rows and returned what it had with nil error, never reading
// rows.Err(): an execution failure -- here a query vector the index's
// distance function refuses -- read as "no similar content" (audit
// knn-recall-limit-error-contract).
func TestVectorRecallVec_AnExecutionFailureIsAnError(t *testing.T) {
	s := newVecStore(t)
	storeProbes(t, s, 3)
	s.mu.RLock()
	space := s.currentSpaceLocked()
	s.mu.RUnlock()

	// The index holds 4-dimension vectors; a 3-dimension query fails when
	// the statement steps, after Query has returned.
	got, err := s.vectorRecallVec("probe", semanticQuery{vec: []float32{1, 0, 0}, space: space}, 5, nil, "", nil)
	if err == nil {
		t.Fatalf("a recall the index refused returned %d result(s) and no error", len(got))
	}

	// A healthy recall still answers.
	if ok, err := s.vectorRecallVec("probe", semanticQuery{vec: []float32{1, 0, 0, 0}, space: space}, 5, nil, "", nil); err != nil || len(ok) == 0 {
		t.Fatalf("healthy recall = %d result(s), %v", len(ok), err)
	}
}
