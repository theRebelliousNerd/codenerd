package synth

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
)

var (
	ErrEmptyResponse = errors.New("empty response")
	ErrMissingJSON   = errors.New("expected JSON object for mangle synth")
)

func DecodeSpec(raw string) (Spec, error) {
	payload, err := extractJSONPayload(raw)
	if err != nil {
		return Spec{}, err
	}

	decoder := json.NewDecoder(strings.NewReader(payload))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()

	var spec Spec
	if err := decoder.Decode(&spec); err != nil {
		return Spec{}, err
	}
	if err := ensureEOF(decoder); err != nil {
		return Spec{}, err
	}

	return spec, nil
}

func FromResponse(raw string, options Options) (Result, error) {
	spec, err := DecodeSpec(raw)
	if err != nil {
		return Result{}, err
	}
	return Compile(spec, options)
}

func extractJSONPayload(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ErrEmptyResponse
	}

	candidate := trimmed
	if strings.HasPrefix(trimmed, "```") {
		start := strings.Index(trimmed, "```")
		if start != -1 {
			end := strings.Index(trimmed[start+3:], "```")
			if end != -1 {
				content := trimmed[start+3 : start+3+end]
				if idx := strings.Index(content, "\n"); idx != -1 {
					content = content[idx+1:]
				}
				candidate = strings.TrimSpace(content)
			}
		}
	}

	if payload, ok := findJSONObject(candidate); ok {
		return payload, nil
	}
	if payload, ok := findJSONObject(trimmed); ok {
		return payload, nil
	}
	return "", ErrMissingJSON
}

func findJSONObject(input string) (string, bool) {
	start := -1
	depth := 0
	inString := false
	escaped := false
	for i := 0; i < len(input); i++ {
		ch := input[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' && inString {
			escaped = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if ch == '{' {
			if depth == 0 {
				start = i
			}
			depth++
			continue
		}
		if ch == '}' {
			if depth == 0 {
				continue
			}
			depth--
			if depth == 0 && start >= 0 {
				return input[start : i+1], true
			}
		}
	}
	return "", false
}

func ensureEOF(decoder *json.Decoder) error {
	var extra interface{}
	if err := decoder.Decode(&extra); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	return errors.New("unexpected trailing JSON content")
}
