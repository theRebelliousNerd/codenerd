package store

import (
	"encoding/json"
	"fmt"
	"strings"

	"codenerd/internal/types"
)

type encodedFactArg struct {
	Type  string `json:"type"`
	Value any    `json:"value,omitempty"`
}

func encodeFactArgs(args []any) (string, error) {
	encoded := make([]encodedFactArg, 0, len(args))
	for _, arg := range args {
		switch v := arg.(type) {
		case nil:
			encoded = append(encoded, encodedFactArg{Type: "nil"})
		case types.MangleAtom:
			encoded = append(encoded, encodedFactArg{Type: "atom", Value: string(v)})
		case string:
			encoded = append(encoded, encodedFactArg{Type: "string", Value: v})
		case int:
			encoded = append(encoded, encodedFactArg{Type: "int64", Value: int64(v)})
		case int64:
			encoded = append(encoded, encodedFactArg{Type: "int64", Value: v})
		case float64:
			encoded = append(encoded, encodedFactArg{Type: "float64", Value: v})
		case bool:
			encoded = append(encoded, encodedFactArg{Type: "bool", Value: v})
		default:
			encoded = append(encoded, encodedFactArg{Type: "string", Value: fmt.Sprintf("%v", v)})
		}
	}

	data, err := json.Marshal(encoded)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func decodeFactArgs(data string) ([]any, error) {
	if strings.TrimSpace(data) == "" {
		return nil, nil
	}

	// UseNumber, not plain Unmarshal.
	//
	// encoding/json decodes every JSON number into an `any` as float64, and
	// float64 holds integers exactly only up to 2^53 (~9.0e15). A Unix
	// nanosecond timestamp is ~1.79e18, so the int64 case below was rounding
	// every one of them: file_topology's ModTime went in as
	// 1786773933859876776 and came back as 1786773933859876864.
	//
	// That is not a cosmetic drift. These rows are read back to build RETRACTION
	// facts for changed and deleted files, and a retraction only removes a fact
	// that matches argument-for-argument. A rounded timestamp produces a fact
	// that matches nothing, so the retraction silently does nothing and the
	// superseded row stays in the kernel forever — which looks exactly like "the
	// scanner does not retract", with no error anywhere to say otherwise.
	//
	// json.Number keeps the literal digits, so Int64() returns the value that
	// was written. Old rows decode identically; nothing needs migrating.
	dec := json.NewDecoder(strings.NewReader(data))
	dec.UseNumber()

	var tagged []encodedFactArg
	if err := dec.Decode(&tagged); err == nil && isTaggedFactArgs(tagged) {
		args := make([]any, 0, len(tagged))
		for _, arg := range tagged {
			switch arg.Type {
			case "nil":
				args = append(args, nil)
			case "atom":
				if s, ok := arg.Value.(string); ok {
					args = append(args, types.MangleAtom(s))
				} else {
					args = append(args, nil)
				}
			case "string":
				if s, ok := arg.Value.(string); ok {
					args = append(args, s)
				} else {
					args = append(args, fmt.Sprintf("%v", arg.Value))
				}
			case "int64":
				switch v := arg.Value.(type) {
				case json.Number:
					if n, err := v.Int64(); err == nil {
						args = append(args, n)
					} else if f, ferr := v.Float64(); ferr == nil {
						// A value that will not fit an int64 is corrupt data
						// rather than a number; keep the old lossy behavior
						// rather than dropping the argument entirely.
						args = append(args, int64(f))
					} else {
						args = append(args, int64(0))
					}
				case float64:
					args = append(args, int64(v))
				case int64:
					args = append(args, v)
				default:
					args = append(args, int64(0))
				}
			case "float64":
				switch v := arg.Value.(type) {
				case json.Number:
					if f, err := v.Float64(); err == nil {
						args = append(args, f)
					} else {
						args = append(args, 0.0)
					}
				case float64:
					args = append(args, v)
				case int64:
					args = append(args, float64(v))
				default:
					args = append(args, 0.0)
				}
			case "bool":
				if b, ok := arg.Value.(bool); ok {
					args = append(args, b)
				} else {
					args = append(args, false)
				}
			default:
				args = append(args, arg.Value)
			}
		}
		return args, nil
	}

	return nil, fmt.Errorf("invalid or non-tagged fact arguments data")
}

func isTaggedFactArgs(args []encodedFactArg) bool {
	for _, arg := range args {
		if arg.Type == "" {
			return false
		}
	}
	return true
}
