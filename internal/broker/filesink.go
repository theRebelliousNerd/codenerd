package broker

import (
	"codenerd/internal/jsonl"
)

// A receipt ring lives in one process. Gate A asks about the distribution of
// calls per epoch "across real sessions", which by definition spans processes,
// and a readout command is itself a different process from the agent that did
// the spending. So the ring is not enough: receipts have to reach disk.
//
// The append-and-rotate mechanics live in internal/jsonl because prompt-atom
// selections need exactly the same thing for exactly the same reason. One
// implementation, not two that drift.

// DefaultReceiptLogName is where receipts land under the workspace .nerd dir.
const DefaultReceiptLogName = "meter/receipts.jsonl"

// FileSink appends receipts to a rotating JSONL log.
type FileSink struct {
	log *jsonl.Appender
}

// NewFileSink opens (or creates) a receipt log at path.
func NewFileSink(path string) (*FileSink, error) {
	log, err := jsonl.Open(path)
	if err != nil {
		return nil, err
	}
	return &FileSink{log: log}, nil
}

// Record implements ReceiptSink.
func (s *FileSink) Record(r Receipt) {
	if s == nil || s.log == nil {
		return
	}
	s.log.Append(r)
}

// Failures returns how many receipt writes have failed and the first failure,
// so a caller can tell "no receipts because nothing spent" from "no receipts
// because the sink has been broken since boot".
func (s *FileSink) Failures() (count int, first error) {
	if s == nil || s.log == nil {
		return 0, nil
	}
	return s.log.Failures()
}

// Completeness says whether the log holds every receipt this sink was given.
// A cost export that reads the log must refuse to call a task cheap when it
// does not: a failed write or an overwritten rotation generation means a
// receipt existed that the log cannot return.
type Completeness struct {
	// Failures is how many receipt writes failed, FirstError the first.
	Failures   int
	FirstError error
	// DroppedGenerations is how many rotated generations were overwritten.
	DroppedGenerations int
}

// Complete reports whether nothing was lost.
func (c Completeness) Complete() bool { return c.Failures == 0 && c.DroppedGenerations == 0 }

// Completeness reports what this sink has lost so far.
func (s *FileSink) Completeness() Completeness {
	if s == nil || s.log == nil {
		return Completeness{}
	}
	n, first := s.log.Failures()
	return Completeness{Failures: n, FirstError: first, DroppedGenerations: s.log.Dropped()}
}

// SetMaxBytes overrides the rotation threshold.
func (s *FileSink) SetMaxBytes(n int64) {
	if s == nil || s.log == nil {
		return
	}
	s.log.SetMaxBytes(n)
}

// Close closes the log.
func (s *FileSink) Close() error {
	if s == nil || s.log == nil {
		return nil
	}
	return s.log.Close()
}

// ReadReceiptLog reads receipts from a JSONL log, oldest first, including the
// rotated generation when present. The second return is how many generations
// ended on a truncated line.
func ReadReceiptLog(path string) ([]Receipt, int, error) {
	return jsonl.Read[Receipt](path)
}
