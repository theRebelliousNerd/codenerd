// Package security provides BrowserNERD-compatible browser evidence controls.
// The behavior is adapted from BrowserNERD under Apache-2.0; see
// THIRD_PARTY_NOTICES.md.
package security

import (
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"strings"
)

// Redacted is the stable replacement for secret-bearing values.
const Redacted = "[REDACTED]"

var (
	authorizationPattern = regexp.MustCompile(`(?i)\b(bearer|basic)\s+[A-Za-z0-9._~+/=-]+`)
	jwtPattern           = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\b`)
	assignmentPattern    = regexp.MustCompile(`(?i)(password|passwd|authorization|cookie|api[_-]?key|access[_-]?token|refresh[_-]?token|client[_-]?secret|token)(\s*[:=]\s*)([^&\s,;"'}]+)`)
	embeddedURLPattern   = regexp.MustCompile(`(?i)https?://[^\s<>"']+`)
)

// Redactor sanitizes browser evidence before it reaches logs, facts, or disk.
type Redactor struct {
	sensitive map[string]struct{}
}

// NewRedactor constructs the default credential redactor.
func NewRedactor(extraKeys []string) *Redactor {
	keys := []string{
		"authorization", "proxy-authorization", "cookie", "set-cookie",
		"x-api-key", "api-key", "apikey", "password", "passwd", "passphrase",
		"secret", "client-secret", "client_secret", "access-token", "access_token",
		"refresh-token", "refresh_token", "id-token", "id_token", "token",
		"private-key", "private_key", "card-number", "card_number", "cvv", "cvc",
	}
	result := &Redactor{sensitive: make(map[string]struct{}, len(keys)+len(extraKeys))}
	for _, key := range append(keys, extraKeys...) {
		result.sensitive[normalizeKey(key)] = struct{}{}
	}
	return result
}

// IsSensitiveKey reports whether a field name is secret-bearing.
func (r *Redactor) IsSensitiveKey(key string) bool {
	if r == nil {
		return false
	}
	normalized := normalizeKey(key)
	if _, ok := r.sensitive[normalized]; ok {
		return true
	}
	for candidate := range r.sensitive {
		if len(candidate) >= 5 && strings.Contains(normalized, candidate) {
			return true
		}
	}
	return false
}

// SanitizeString redacts URL parameters, authorization values, JWTs, and
// common textual secret assignments.
func (r *Redactor) SanitizeString(value string) string {
	if r == nil {
		return value
	}
	value = r.RedactURL(value)
	value = embeddedURLPattern.ReplaceAllStringFunc(value, r.RedactURL)
	value = authorizationPattern.ReplaceAllStringFunc(value, func(match string) string {
		fields := strings.Fields(match)
		if len(fields) == 0 {
			return Redacted
		}
		return fields[0] + " " + Redacted
	})
	value = jwtPattern.ReplaceAllString(value, Redacted)
	return assignmentPattern.ReplaceAllString(value, `${1}${2}`+Redacted)
}

// RedactURL removes values for sensitive query parameters.
func (r *Redactor) RedactURL(raw string) string {
	if r == nil {
		return raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.RawQuery == "" {
		return raw
	}
	query := parsed.Query()
	changed := false
	for key, values := range query {
		if !r.IsSensitiveKey(key) {
			continue
		}
		for i := range values {
			values[i] = Redacted
		}
		query[key] = values
		changed = true
	}
	if changed {
		parsed.RawQuery = query.Encode()
		return parsed.String()
	}
	return raw
}

// RedactHeader returns a safe HTTP header representation.
func (r *Redactor) RedactHeader(name, value string) string {
	if r == nil {
		return value
	}
	if !r.IsSensitiveKey(name) {
		return r.SanitizeString(value)
	}
	if strings.EqualFold(strings.TrimSpace(name), "authorization") || strings.EqualFold(strings.TrimSpace(name), "proxy-authorization") {
		fields := strings.Fields(value)
		if len(fields) > 1 {
			return fields[0] + " " + Redacted
		}
	}
	return Redacted
}

// RedactInputValue hides values from password, token, payment, and similarly
// described inputs.
func (r *Redactor) RedactInputValue(descriptor, value string) string {
	if r == nil {
		r = NewRedactor(nil)
	}
	if r.IsSensitiveKey(descriptor) || sensitiveInputDescriptor(descriptor) {
		return Redacted
	}
	return r.SanitizeString(value)
}

// Sanitize recursively copies maps, slices, and structs while redacting leaves.
func (r *Redactor) Sanitize(value any) any {
	if r == nil {
		return value
	}
	return r.sanitizeValue(reflect.ValueOf(value), "")
}

func (r *Redactor) sanitizeValue(value reflect.Value, fieldName string) any {
	if !value.IsValid() {
		return nil
	}
	for value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}
	if r.IsSensitiveKey(fieldName) {
		return Redacted
	}
	switch value.Kind() {
	case reflect.String:
		return r.SanitizeString(value.String())
	case reflect.Map:
		result := make(map[string]any, value.Len())
		sensitiveInput := r.mapDescribesSensitiveInput(value)
		iter := value.MapRange()
		for iter.Next() {
			key := fmt.Sprint(iter.Key().Interface())
			if sensitiveInput && inputValueKey(key) {
				result[key] = Redacted
			} else {
				result[key] = r.sanitizeValue(iter.Value(), key)
			}
		}
		return result
	case reflect.Slice, reflect.Array:
		result := make([]any, value.Len())
		for i := range result {
			result[i] = r.sanitizeValue(value.Index(i), fieldName)
		}
		return result
	case reflect.Struct:
		result := make(map[string]any, value.NumField())
		typeInfo := value.Type()
		for i := 0; i < value.NumField(); i++ {
			field := typeInfo.Field(i)
			if !field.IsExported() {
				continue
			}
			name := field.Name
			if tag := strings.Split(field.Tag.Get("json"), ",")[0]; tag != "" && tag != "-" {
				name = tag
			}
			result[name] = r.sanitizeValue(value.Field(i), name)
		}
		return result
	default:
		return value.Interface()
	}
}

func (r *Redactor) mapDescribesSensitiveInput(value reflect.Value) bool {
	iter := value.MapRange()
	for iter.Next() {
		key := normalizeKey(fmt.Sprint(iter.Key().Interface()))
		switch key {
		case "type", "name", "id", "selector", "autocomplete", "label":
		default:
			continue
		}
		raw := iter.Value()
		for raw.IsValid() && (raw.Kind() == reflect.Interface || raw.Kind() == reflect.Pointer) {
			if raw.IsNil() {
				break
			}
			raw = raw.Elem()
		}
		if raw.IsValid() && raw.Kind() == reflect.String && sensitiveInputDescriptor(raw.String()) {
			return true
		}
	}
	return false
}

func sensitiveInputDescriptor(value string) bool {
	normalized := normalizeKey(value)
	for _, marker := range []string{
		"password", "passwd", "current-password", "new-password", "one-time-code",
		"credit-card", "cc-number", "cc-csc", "card-number", "cvv", "cvc", "api-key", "token", "secret",
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func inputValueKey(key string) bool {
	switch normalizeKey(key) {
	case "value", "text", "content", "input":
		return true
	default:
		return false
	}
}

func normalizeKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.ReplaceAll(key, "_", "-")
	return strings.ReplaceAll(key, " ", "-")
}
