package perception

import (
	"bytes"
	"encoding/json"
	"strings"
)

// visibleTaskOutput projects envelope responses onto their displayed answer for
// interaction audits and vocabulary learning. Native thinking blocks are already
// separate at the provider boundary. This also keeps retired control fields in
// historical records out of the critic's input.
//
// It reads an envelope the way articulation admits one
// (internal/articulation/admission.go): one JSON object, optionally inside one
// Markdown fence; member names by their decoded spelling, matched
// case-insensitively; a name that appears twice makes the object ambiguous and
// nothing is selected from it. It used to decode the whole input as JSON and
// match exact lowercase keys, so a fenced envelope projected to nothing, an
// uppercase surface key to nothing, and uppercase outer keys left the whole
// control envelope in the audit record (audit
// interaction-audit-surface-projection-parity).
func visibleTaskOutput(response string) string {
	body := unfenceEnvelope(response)
	fields, ambiguous, err := decodeEnvelopeFields(body)
	if err != nil {
		// A partial control envelope is not a visible answer. Do not recover its
		// control prose as fallback learning material.
		lower := strings.ToLower(response)
		for _, marker := range []string{`"control_packet"`, `"reasoning_trace"`, `"chain_of_thought"`} {
			if strings.Contains(lower, marker) {
				return ""
			}
		}
		return response
	}
	if ambiguous {
		return ""
	}
	if _, envelope := fields["control_packet"]; envelope {
		var surface string
		if err := json.Unmarshal(fields["surface_response"].value, &surface); err != nil {
			return ""
		}
		return surface
	}
	changed := false
	for _, key := range []string{"reasoning_trace", "chain_of_thought"} {
		if _, found := fields[key]; found {
			delete(fields, key)
			changed = true
		}
	}
	if changed {
		out := make(map[string]json.RawMessage, len(fields))
		for _, f := range fields {
			out[f.name] = f.value
		}
		data, err := json.Marshal(out)
		if err != nil {
			return ""
		}
		return string(data)
	}
	return response
}

// envelopeField is one top-level member: its decoded name and raw value.
type envelopeField struct {
	name  string
	value json.RawMessage
}

// unfenceEnvelope returns the body of a response wrapped in one Markdown code
// fence (```json ... ``` or ``` ... ```), or the response itself.
func unfenceEnvelope(response string) string {
	s := strings.TrimSpace(response)
	if !strings.HasPrefix(s, "```") || !strings.HasSuffix(s, "```") || len(s) < 6 {
		return response
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(s, "```"), "```")
	if nl := strings.IndexByte(inner, '\n'); nl >= 0 {
		// The info string ("json") ends at the first newline.
		if tag := strings.TrimSpace(inner[:nl]); tag == "" || !strings.ContainsAny(tag, "{}") {
			inner = inner[nl+1:]
		}
	}
	return strings.TrimSpace(inner)
}

// decodeEnvelopeFields decodes one JSON object into its top-level members,
// keyed by lowercased decoded name. ambiguous reports a name that appears
// twice under case folding. err is input that is not exactly one object.
func decodeEnvelopeFields(body string) (map[string]envelopeField, bool, error) {
	dec := json.NewDecoder(strings.NewReader(body))
	tok, err := dec.Token()
	if err != nil {
		return nil, false, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, false, errNotObject
	}
	fields := map[string]envelopeField{}
	ambiguous := false
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, false, err
		}
		name, ok := keyTok.(string)
		if !ok {
			return nil, false, errNotObject
		}
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, false, err
		}
		key := strings.ToLower(name)
		if _, dup := fields[key]; dup {
			ambiguous = true
		}
		fields[key] = envelopeField{name: name, value: value}
	}
	if _, err := dec.Token(); err != nil { // the closing brace
		return nil, false, err
	}
	if rest, _ := readRest(dec); len(bytes.TrimSpace(rest)) > 0 {
		return nil, false, errNotObject
	}
	return fields, ambiguous, nil
}

func readRest(dec *json.Decoder) ([]byte, error) {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(dec.Buffered())
	return buf.Bytes(), err
}

type envelopeError string

func (e envelopeError) Error() string { return string(e) }

const errNotObject = envelopeError("not exactly one JSON object")
