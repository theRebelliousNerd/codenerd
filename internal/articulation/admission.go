package articulation

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// The envelope grammar the processor admits, and the one the live stream
// parser follows:
//
//   - A response is one JSON object, optionally inside one Markdown fence, with
//     nothing after it. Text around an envelope is a declared legacy framing
//     (the "json_extracted" method) and is reported, never silently dropped.
//   - A member name is its decoded spelling: "surface_response" is
//     "surface_response". Protocol names also match case-insensitively, the
//     tolerance encoding/json has always given the final decoder, so the
//     stream parser and the final parser read the same key as the same key.
//   - A name appears once per object. Go's decoder keeps the last of two
//     duplicates and the stream parser keeps the first, so a repeat means the
//     two surfaces of one reply would disagree; the reply is ambiguous and
//     nothing is selected from it. Inside tool_args the model writes names
//     for a tool, so the check there is on the exact decoded name.
//   - Two candidate envelopes in one reply is ambiguity too. Choosing the first
//     or the last was a guess that a quoted example could win.

// maxEnvelopeDepth bounds nesting during admission so a hostile reply cannot
// spend unbounded memory on frames.
const maxEnvelopeDepth = maxJSONDepth

// ambiguousEnvelopeError reports a reply with more than one reading of its
// own envelope. It is a classification, not a malformed-JSON error: the
// processor stops on it instead of trying the next, more tolerant, reader.
type ambiguousEnvelopeError struct{ reason string }

func (e *ambiguousEnvelopeError) Error() string { return "ambiguous envelope: " + e.reason }

func isAmbiguousEnvelope(err error) (*ambiguousEnvelopeError, bool) {
	var amb *ambiguousEnvelopeError
	if errors.As(err, &amb) {
		return amb, true
	}
	return nil, false
}

// scanFrame is one open container during the admission scan.
type scanFrame struct {
	object    bool
	expectKey bool
	exact     bool // names are arbitrary tool arguments: compare exactly
	keys      map[string]struct{}
}

// scanEnvelope checks that s is exactly one JSON object whose member names are
// each unique after decoding. A syntax error is returned as an ordinary error
// (the next reader may still make sense of the text); a repeated name is an
// *ambiguousEnvelopeError.
func scanEnvelope(s string) error {
	dec := json.NewDecoder(strings.NewReader(s))
	var stack []scanFrame
	pendingExact := false
	started := false

	closeValue := func() {
		if n := len(stack); n > 0 && stack[n-1].object {
			stack[n-1].expectKey = true
		}
	}

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			if !started || len(stack) != 0 {
				return io.ErrUnexpectedEOF
			}
			return nil
		}
		if err != nil {
			return err
		}
		if len(stack) == 0 && started {
			return fmt.Errorf("unexpected content after the envelope object")
		}

		switch v := tok.(type) {
		case json.Delim:
			switch v {
			case '{', '[':
				if !started && v != '{' {
					return fmt.Errorf("an envelope is a JSON object, not an array")
				}
				started = true
				if len(stack) >= maxEnvelopeDepth {
					return fmt.Errorf("envelope nesting exceeds %d levels", maxEnvelopeDepth)
				}
				exact := pendingExact
				pendingExact = false
				if n := len(stack); n > 0 && stack[n-1].exact {
					exact = true
				}
				f := scanFrame{object: v == '{', exact: exact}
				if f.object {
					f.expectKey = true
					f.keys = map[string]struct{}{}
				}
				stack = append(stack, f)
			case '}', ']':
				stack = stack[:len(stack)-1]
				closeValue()
				if len(stack) == 0 {
					// The root closed. Anything further is caught on the next
					// token (or EOF ends the scan).
					continue
				}
			}
		case string:
			if !started {
				return fmt.Errorf("an envelope is a JSON object, not a string")
			}
			top := &stack[len(stack)-1]
			if top.object && top.expectKey {
				top.expectKey = false
				name := v
				if !top.exact {
					name = foldKey(v)
				}
				if _, dup := top.keys[name]; dup {
					return &ambiguousEnvelopeError{reason: fmt.Sprintf("the key %q appears more than once", v)}
				}
				top.keys[name] = struct{}{}
				pendingExact = !top.exact && name == "tool_args"
				continue
			}
			pendingExact = false
			closeValue()
		default:
			if !started {
				return fmt.Errorf("an envelope is a JSON object")
			}
			pendingExact = false
			closeValue()
		}
	}
}

// foldKey is the identity of a protocol member name: the case folding
// encoding/json applies when it matches a name to a struct field.
func foldKey(name string) string { return strings.ToLower(strings.ToUpper(name)) }

// admissionNotice is the host-owned surface for a reply that could not be
// admitted. It is what the person and the history keep; any text shown while
// the reply streamed was provisional and is not kept.
func admissionNotice(reason string) string {
	return "_The model's reply could not be accepted: " + reason +
		". No part of it was applied, and any text shown while it arrived is not part of the answer. Please ask again._"
}
