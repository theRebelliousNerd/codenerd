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

// Err returns the first write failure and how many have occurred, so a caller
// can tell "no receipts because nothing spent" from "no receipts because the
// sink has been broken since boot".
func (s *FileSink) Err() (error, int) {
	if s == nil || s.log == nil {
		return nil, 0
	}
	return s.log.Err()
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
