package store

import (
	"encoding/json"
	"fmt"
	"strings"

	"codenerd/internal/types"
)

type encodedFactArg struct {
	Type  string      `json:"type"`
	Value interface{} `json:"value,omitempty"`
}

func encodeFactArgs(args []interface{}) (string, error) {
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

func decodeFactArgs(data string) ([]interface{}, error) {
	if strings.TrimSpace(data) == "" {
		return nil, nil
	}

	var tagged []encodedFactArg
	if err := json.Unmarshal([]byte(data), &tagged); err == nil && isTaggedFactArgs(tagged) {
		args := make([]interface{}, 0, len(tagged))
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
				case float64:
					args = append(args, int64(v))
				case int64:
					args = append(args, v)
				default:
					args = append(args, int64(0))
				}
			case "float64":
				switch v := arg.Value.(type) {
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
