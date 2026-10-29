package browser

// Declarative element matching is adapted from BrowserNERD's Apache-2.0
// generated-test contract. Portable fixtures retain semantic identity only;
// private CSS selectors and generation-bound refs never become test data.

import (
	"context"
	"fmt"
	"strings"
)

const maxMatcherFieldBytes = 512

// ElementMatcher is a portable, selector-free description of one element.
// Every populated field is matched conjunctively and resolution must be unique.
type ElementMatcher struct {
	DataTestID string `json:"data_testid,omitempty" yaml:"data_testid,omitempty"`
	ID         string `json:"id,omitempty" yaml:"id,omitempty"`
	Name       string `json:"name,omitempty" yaml:"name,omitempty"`
	AriaLabel  string `json:"aria_label,omitempty" yaml:"aria_label,omitempty"`
	Role       string `json:"role,omitempty" yaml:"role,omitempty"`
	Text       string `json:"text,omitempty" yaml:"text,omitempty"`
	TagName    string `json:"tag_name,omitempty" yaml:"tag_name,omitempty"`
	InputType  string `json:"input_type,omitempty" yaml:"input_type,omitempty"`
}

// Validate rejects empty or oversized matchers before page access.
func (m ElementMatcher) Validate() error {
	fields := []string{m.DataTestID, m.ID, m.Name, m.AriaLabel, m.Role, m.Text, m.TagName, m.InputType}
	populated := 0
	for _, field := range fields {
		if strings.TrimSpace(field) == "" {
			continue
		}
		populated++
		if len(field) > maxMatcherFieldBytes {
			return fmt.Errorf("element matcher field exceeds %d bytes", maxMatcherFieldBytes)
		}
	}
	if populated == 0 {
		return fmt.Errorf("element matcher requires at least one semantic field")
	}
	return nil
}

// IsSensitive reports whether fixture values for this element must use an
// environment variable instead of a literal.
func (m ElementMatcher) IsSensitive() bool {
	descriptor := strings.ToLower(strings.Join([]string{
		m.DataTestID, m.ID, m.Name, m.AriaLabel, m.Role, m.Text, m.TagName, m.InputType,
	}, " "))
	for _, marker := range []string{
		"password", "passwd", "current-password", "new-password", "one-time-code",
		"credit-card", "cc-number", "cc-csc", "card-number", "cvv", "cvc", "api-key", "token", "secret",
	} {
		if strings.Contains(descriptor, marker) {
			return true
		}
	}
	return false
}

// MatcherForRef returns a redacted portable matcher for a current opaque ref.
func (m *SessionManager) MatcherForRef(sessionID, ref string) (ElementMatcher, error) {
	registry := m.Registry(sessionID)
	if registry == nil {
		return ElementMatcher{}, fmt.Errorf("unknown session: %s", sessionID)
	}
	fingerprint, ok := registry.Get(strings.TrimSpace(ref))
	if !ok {
		return ElementMatcher{}, fmt.Errorf("stale or unknown element ref %s; observe the page again", ref)
	}
	matcher := ElementMatcher{
		DataTestID: m.SanitizeForEvidence(fingerprint.DataTestID),
		ID:         m.SanitizeForEvidence(fingerprint.ID),
		Name:       m.SanitizeForEvidence(fingerprint.Name),
		AriaLabel:  m.SanitizeForEvidence(fingerprint.AriaLabel),
		Role:       strings.ToLower(strings.TrimSpace(fingerprint.Role)),
		Text:       m.SanitizeForEvidence(strings.TrimSpace(fingerprint.TextContent)),
		TagName:    strings.ToLower(strings.TrimSpace(fingerprint.TagName)),
		InputType:  strings.ToLower(strings.TrimSpace(fingerprint.InputType)),
	}
	if err := matcher.Validate(); err != nil {
		return ElementMatcher{}, fmt.Errorf("element ref %s is not portable: %w", ref, err)
	}
	return matcher, nil
}

// ResolveElementMatcher observes the current page, selects one exact semantic
// match, and returns its current generation-bound ref. Truncated discovery and
// ambiguous matches fail closed.
func (m *SessionManager) ResolveElementMatcher(ctx context.Context, sessionID string, matcher ElementMatcher) (string, error) {
	if err := matcher.Validate(); err != nil {
		return "", err
	}
	observation, err := m.Observe(ctx, sessionID, ObserveOptions{
		Mode: "interactive", View: "full", MaxItems: maxObservationItems, VisibleOnly: true,
	})
	if err != nil {
		return "", fmt.Errorf("observe matcher candidates: %w", err)
	}
	if observation.Truncated {
		return "", fmt.Errorf("element matcher resolution is incomplete because interactive discovery was truncated")
	}
	raw, _ := observation.Data["interactive"].([]InteractiveElement)
	matches := make([]InteractiveElement, 0, 2)
	for _, candidate := range raw {
		if candidate.Fingerprint != nil && matcherMatches(matcher, *candidate.Fingerprint) {
			matches = append(matches, candidate)
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("element matcher did not match a visible interactive element")
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("element matcher is ambiguous across %d visible interactive elements", len(matches))
	}
	return matches[0].Ref, nil
}

func matcherMatches(matcher ElementMatcher, candidate PublicElementFingerprint) bool {
	checks := []struct {
		expected string
		actual   string
		fold     bool
	}{
		{matcher.DataTestID, candidate.DataTestID, false}, {matcher.ID, candidate.ID, false},
		{matcher.Name, candidate.Name, false}, {matcher.AriaLabel, candidate.AriaLabel, false},
		{matcher.Role, candidate.Role, true}, {matcher.Text, strings.TrimSpace(candidate.TextContent), false},
		{matcher.TagName, candidate.TagName, true}, {matcher.InputType, candidate.InputType, true},
	}
	for _, check := range checks {
		if strings.TrimSpace(check.expected) == "" {
			continue
		}
		if check.fold {
			if !strings.EqualFold(strings.TrimSpace(check.expected), strings.TrimSpace(check.actual)) {
				return false
			}
		} else if strings.TrimSpace(check.expected) != strings.TrimSpace(check.actual) {
			return false
		}
	}
	return true
}
