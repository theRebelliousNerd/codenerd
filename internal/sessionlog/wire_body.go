package sessionlog

import (
	"encoding/json"
	"fmt"
)

// WireBody restores the body of one captured provider round trip from an
// exchange file's bytes: the request body, or the response body when
// response is set. attempt selects the round trip by its Wire.Attempt
// number; 0 is the last one, the attempt whose answer the call kept.
//
// An exchange stores a JSON body as ordered JSON with every multi-line string
// split into its lines and listed in lines_paths (SplitLines); RestoreTree
// joins them back, so the body comes out with the provider's key order and
// exact strings, ready to diff against another call or send again. A text
// body is stored as its lines and is joined. A body the capture did not keep
// (binary, a streamed response) is an error that carries the file's own note.
func WireBody(exchange []byte, attempt int, response bool) ([]byte, error) {
	tree, err := DecodeOrdered(exchange)
	if err != nil {
		return nil, fmt.Errorf("exchange file does not parse: %w", err)
	}
	root, ok := tree.(*Object)
	if !ok {
		return nil, fmt.Errorf("exchange file is not a JSON object")
	}
	wires, _ := memberArray(root, "wire")
	if len(wires) == 0 {
		return nil, fmt.Errorf("the exchange holds no wire capture (the client sent nothing through a captured transport; see logical_request)")
	}
	round, err := pickAttempt(wires, attempt)
	if err != nil {
		return nil, err
	}
	side := "request"
	if response {
		side = "response"
	}
	sideVal, ok := round.Get(side)
	sideObj, isObj := sideVal.(*Object)
	if !ok || !isObj {
		return nil, fmt.Errorf("round trip has no %s recorded", side)
	}
	body, _ := sideObj.Get("body")
	note, _ := sideObj.Get("body_note")
	noteText, _ := note.(string)
	if body == nil {
		if noteText != "" {
			return nil, fmt.Errorf("the %s body was not recorded: %s", side, noteText)
		}
		return nil, fmt.Errorf("the %s had no body", side)
	}
	paths := stringList(sideObj, "lines_paths")
	if noteText != "" {
		// A text body: its lines, joined.
		lines, ok := body.([]any)
		if !ok {
			return nil, fmt.Errorf("the %s text body is not stored as lines", side)
		}
		out := make([]string, 0, len(lines))
		for i, l := range lines {
			s, ok := l.(string)
			if !ok {
				return nil, fmt.Errorf("the %s text body line %d is not a string", side, i)
			}
			out = append(out, s)
		}
		return []byte(Join(out)), nil
	}
	restored, err := RestoreTree(body, paths)
	if err != nil {
		return nil, err
	}
	return json.Marshal(restored)
}

func memberArray(o *Object, key string) ([]any, bool) {
	v, ok := o.Get(key)
	if !ok {
		return nil, false
	}
	arr, ok := v.([]any)
	return arr, ok
}

func stringList(o *Object, key string) []string {
	arr, _ := memberArray(o, key)
	out := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func pickAttempt(wires []any, attempt int) (*Object, error) {
	if attempt == 0 {
		last, ok := wires[len(wires)-1].(*Object)
		if !ok {
			return nil, fmt.Errorf("round trip %d is not an object", len(wires))
		}
		return last, nil
	}
	for _, w := range wires {
		o, ok := w.(*Object)
		if !ok {
			continue
		}
		if n, ok := o.Get("attempt"); ok {
			if num, ok := n.(json.Number); ok && num.String() == fmt.Sprint(attempt) {
				return o, nil
			}
		}
	}
	return nil, fmt.Errorf("the exchange has no round trip with attempt %d (it has %d)", attempt, len(wires))
}
