package browser

// Progressive action is adapted from BrowserNERD's Apache-2.0 browser-act
// contract. The operation vocabulary is deliberately closed; arbitrary
// JavaScript and live-kernel waits belong to later, separately gated surfaces.

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	browsersecurity "codenerd/internal/browser/security"
	"codenerd/internal/mangle"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

const (
	maxActionOperations = 25
	maxFillFields       = 50
	maxActionSleep      = 30 * time.Second
)

type FillField struct {
	Ref   string `json:"ref"`
	Value string `json:"value"`
}

type ActionOperation struct {
	Type            string      `json:"type"`
	URL             string      `json:"url,omitempty"`
	Ref             string      `json:"ref,omitempty"`
	Action          string      `json:"action,omitempty"`
	Value           string      `json:"value,omitempty"`
	Submit          bool        `json:"submit,omitempty"`
	Fields          []FillField `json:"fields,omitempty"`
	SubmitButton    string      `json:"submit_button,omitempty"`
	Key             string      `json:"key,omitempty"`
	DurationMS      int         `json:"duration_ms,omitempty"`
	BrowserID       string      `json:"browser_id,omitempty"`
	TargetID        string      `json:"target_id,omitempty"`
	SessionID       string      `json:"session_id,omitempty"`
	SourceSessionID string      `json:"source_session_id,omitempty"`
	Isolated        bool        `json:"isolated,omitempty"`
	NewInstance     *bool       `json:"new_instance,omitempty"`
}

type ActionStepResult struct {
	Index   int            `json:"index"`
	Type    string         `json:"type"`
	Success bool           `json:"success"`
	Error   string         `json:"error,omitempty"`
	Result  map[string]any `json:"result,omitempty"`
}

type ActionExecution struct {
	Success         bool               `json:"success"`
	Status          string             `json:"status"`
	SessionID       string             `json:"session_id,omitempty"`
	Summary         string             `json:"summary"`
	Counts          map[string]int     `json:"counts"`
	Results         []ActionStepResult `json:"results"`
	EvidenceHandles []string           `json:"evidence_handles"`
}

// ExecuteActions runs a bounded sequence and stops on the first failure when
// requested. Every operation consumes the current active session unless it
// carries an explicit session_id.
func (m *SessionManager) ExecuteActions(ctx context.Context, sessionID string, operations []ActionOperation, stopOnError bool) (ActionExecution, error) {
	if len(operations) == 0 {
		return ActionExecution{}, fmt.Errorf("operations must be a non-empty array")
	}
	if len(operations) > maxActionOperations {
		return ActionExecution{}, fmt.Errorf("operations exceeds limit of %d", maxActionOperations)
	}

	activeSession := sessionID
	results := make([]ActionStepResult, 0, len(operations))
	succeeded, failed := 0, 0
	for index, operation := range operations {
		if err := ctx.Err(); err != nil {
			return ActionExecution{}, err
		}
		opType := strings.ToLower(strings.TrimSpace(operation.Type))
		stepSession := activeSession
		if operation.SessionID != "" {
			stepSession = operation.SessionID
		}
		step := ActionStepResult{Index: index, Type: opType}
		var (
			result map[string]any
			err    error
		)

		switch opType {
		case "navigate":
			err = requireSession(stepSession)
			if err == nil && strings.TrimSpace(operation.URL) == "" {
				err = fmt.Errorf("url is required")
			}
			if err == nil {
				err = m.Navigate(ctx, stepSession, operation.URL)
				result = map[string]any{"url": m.SanitizeForEvidence(operation.URL)}
			}
		case "interact":
			err = requireSession(stepSession)
			if err == nil {
				result, err = m.InteractRef(ctx, stepSession, operation.Ref, operation.Action, operation.Value, operation.Submit)
			}
		case "fill":
			err = requireSession(stepSession)
			if err == nil {
				result, err = m.FillRefs(ctx, stepSession, operation.Fields, operation.Submit, operation.SubmitButton)
			}
		case "key":
			err = requireSession(stepSession)
			if err == nil {
				err = m.PressKey(ctx, stepSession, operation.Key)
				result = map[string]any{"key": operation.Key}
			}
		case "history":
			err = requireSession(stepSession)
			if err == nil {
				err = m.History(ctx, stepSession, operation.Action)
				result = map[string]any{"action": strings.ToLower(operation.Action)}
			}
		case "sleep":
			duration := time.Duration(operation.DurationMS) * time.Millisecond
			if duration < 0 || duration > maxActionSleep {
				err = fmt.Errorf("duration_ms must be between 0 and %d", maxActionSleep.Milliseconds())
			} else {
				err = sleepWithContext(ctx, duration)
				result = map[string]any{"slept_ms": operation.DurationMS}
			}
		case "session_create":
			created, createErr := m.CreateTab(ctx, operation.BrowserID, operation.URL, operation.Isolated)
			err = createErr
			if created != nil {
				activeSession = created.ID
				result = map[string]any{"session": sanitizeSession(m, *created)}
			}
		case "session_attach":
			attached, attachErr := m.AttachToBrowser(ctx, operation.BrowserID, operation.TargetID)
			err = attachErr
			if attached != nil {
				activeSession = attached.ID
				result = map[string]any{"session": sanitizeSession(m, *attached)}
			}
		case "session_fork":
			source := operation.SourceSessionID
			if source == "" {
				source = stepSession
			}
			forked, forkErr := m.ForkSession(ctx, source, operation.URL)
			err = forkErr
			if forked != nil {
				activeSession = forked.ID
				result = map[string]any{"session": sanitizeSession(m, *forked)}
			}
		case "session_focus":
			err = requireSession(stepSession)
			if err == nil {
				err = m.FocusSession(ctx, stepSession)
				result = map[string]any{"session_id": stepSession}
			}
		case "session_close":
			err = requireSession(stepSession)
			if err == nil {
				err = m.CloseSession(ctx, stepSession)
				result = map[string]any{"session_id": stepSession}
				if stepSession == activeSession {
					activeSession = ""
				}
			}
		case "browser_launch":
			var launched *BrowserInstance
			newInstance := operation.NewInstance == nil || *operation.NewInstance
			if newInstance {
				launched, err = m.LaunchAdditional(ctx)
			} else {
				err = m.Start(ctx)
				if err == nil {
					instances := m.ListBrowsers()
					for i := range instances {
						if instances[i].Default {
							launched = &instances[i]
							break
						}
					}
				}
			}
			if launched != nil {
				copy := *launched
				copy.ControlURL = m.SanitizeForEvidence(copy.ControlURL)
				result = map[string]any{"browser": copy}
			}
		case "browser_close":
			if strings.TrimSpace(operation.BrowserID) == "" {
				err = fmt.Errorf("browser_id is required")
			} else {
				err = m.CloseBrowser(ctx, operation.BrowserID)
				result = map[string]any{"browser_id": operation.BrowserID}
			}
		default:
			err = fmt.Errorf("unknown operation type %q", operation.Type)
		}

		step.Success = err == nil
		step.Result = result
		if err != nil {
			step.Error = m.SanitizeForEvidence(err.Error())
			failed++
		} else {
			succeeded++
		}
		results = append(results, step)
		if err != nil && stopOnError {
			break
		}
	}

	status := "ok"
	if failed > 0 {
		status = "error"
	}
	handle := fmt.Sprintf("browser:%s:act:%d", activeSession, time.Now().UnixMilli())
	return ActionExecution{
		Success: failed == 0, Status: status, SessionID: activeSession,
		Summary: fmt.Sprintf("executed %d operation(s): %d succeeded, %d failed", len(results), succeeded, failed),
		Counts:  map[string]int{"total": len(results), "succeeded": succeeded, "failed": failed},
		Results: results, EvidenceHandles: []string{handle},
	}, nil
}

// InteractRef resolves an opaque ref with fingerprint fallback and performs one
// closed interaction.
func (m *SessionManager) InteractRef(ctx context.Context, sessionID, ref, action, value string, submit bool) (map[string]any, error) {
	if strings.TrimSpace(ref) == "" {
		return nil, fmt.Errorf("ref is required")
	}
	action = strings.ToLower(strings.TrimSpace(action))
	if action == "" {
		action = "click"
	}
	element, fingerprint, err := m.resolveElementRef(ctx, sessionID, ref)
	if err != nil {
		return nil, err
	}
	visible, err := element.Visible()
	if err != nil || !visible {
		return nil, fmt.Errorf("element %s is not visible", ref)
	}

	descriptor := strings.Join([]string{
		fingerprint.TagName, fingerprint.ID, fingerprint.Name, fingerprint.AriaLabel,
		fingerprint.DataTestID, fingerprint.Role, fingerprint.InputType, ref,
	}, " ")
	safeValue := m.redactor.RedactInputValue(descriptor, value)
	sensitive := safeValue == browsersecurity.Redacted
	result := map[string]any{"ref": ref, "action": action}

	switch action {
	case "click":
		err = element.Click(proto.InputMouseButtonLeft, 1)
	case "type":
		if selectErr := element.SelectAllText(); selectErr == nil {
			_ = element.Input("")
		}
		err = element.Input(value)
		if err == nil && submit {
			err = pressAndRelease(element.Page().Keyboard, input.Enter)
		}
		result["characters"] = utf8.RuneCountInString(value)
		if sensitive {
			result["redacted"] = true
		}
	case "select":
		err = element.Select([]string{value}, true, rod.SelectorTypeText)
		if sensitive {
			result["redacted"] = true
		}
	case "toggle":
		err = element.Click(proto.InputMouseButtonLeft, 1)
		if err == nil {
			if checked, propertyErr := element.Property("checked"); propertyErr == nil {
				result["checked"] = checked.Bool()
			}
		}
	case "clear":
		if selectErr := element.SelectAllText(); selectErr != nil {
			err = selectErr
		} else {
			err = element.Input("")
		}
	default:
		return nil, fmt.Errorf("unsupported interaction action %q", action)
	}
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", action, ref, err)
	}

	now := time.Now()
	qualified := qualifiedRef(sessionID, ref)
	var fact mangle.Fact
	if action == "type" || action == "select" || action == "clear" {
		fact = mangle.Fact{Predicate: "input_event", Args: []any{qualified, safeValue, now.UnixMilli()}, Timestamp: now}
	} else {
		fact = mangle.Fact{Predicate: "click_event", Args: []any{qualified, now.UnixMilli()}, Timestamp: now}
	}
	if err := m.addFacts([]mangle.Fact{fact}); err != nil {
		return nil, fmt.Errorf("assert browser action fact: %w", err)
	}
	return result, nil
}

func (m *SessionManager) FillRefs(ctx context.Context, sessionID string, fields []FillField, submit bool, submitButton string) (map[string]any, error) {
	if len(fields) == 0 {
		return nil, fmt.Errorf("fields must be a non-empty array")
	}
	if len(fields) > maxFillFields {
		return nil, fmt.Errorf("fields exceeds limit of %d", maxFillFields)
	}
	filled := make([]string, 0, len(fields))
	for _, field := range fields {
		if _, err := m.InteractRef(ctx, sessionID, field.Ref, "type", field.Value, false); err != nil {
			return nil, fmt.Errorf("fill %s: %w", field.Ref, err)
		}
		filled = append(filled, field.Ref)
	}
	if submitButton != "" {
		if _, err := m.InteractRef(ctx, sessionID, submitButton, "click", "", false); err != nil {
			return nil, fmt.Errorf("submit form: %w", err)
		}
	} else if submit {
		if err := m.PressKey(ctx, sessionID, "Enter"); err != nil {
			return nil, fmt.Errorf("submit form: %w", err)
		}
	}
	return map[string]any{"filled_refs": filled, "submitted": submit || submitButton != ""}, nil
}

func (m *SessionManager) PressKey(ctx context.Context, sessionID, keySpec string) error {
	page, ok := m.Page(sessionID)
	if !ok || page == nil {
		return fmt.Errorf("unknown session: %s", sessionID)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	parts := strings.Split(keySpec, "+")
	if len(parts) == 0 || strings.TrimSpace(parts[len(parts)-1]) == "" {
		return fmt.Errorf("key is required")
	}
	modifiers := make([]input.Key, 0, len(parts)-1)
	for _, raw := range parts[:len(parts)-1] {
		modifier, ok := modifierKey(strings.TrimSpace(raw))
		if !ok {
			return fmt.Errorf("unsupported modifier %q", raw)
		}
		modifiers = append(modifiers, modifier)
	}
	key, ok := namedKey(strings.TrimSpace(parts[len(parts)-1]))
	if !ok {
		return fmt.Errorf("unsupported key %q", parts[len(parts)-1])
	}
	keyboard := page.Context(ctx).Keyboard
	for _, modifier := range modifiers {
		if err := keyboard.Press(modifier); err != nil {
			releaseKeys(keyboard, modifiers)
			return err
		}
	}
	if err := keyboard.Type(key); err != nil {
		releaseKeys(keyboard, modifiers)
		return err
	}
	return releaseKeys(keyboard, modifiers)
}

func (m *SessionManager) History(ctx context.Context, sessionID, action string) error {
	page, ok := m.Page(sessionID)
	if !ok || page == nil {
		return fmt.Errorf("unknown session: %s", sessionID)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	m.invalidateElementReferences(sessionID)
	var err error
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "back":
		err = page.Context(ctx).NavigateBack()
	case "forward":
		err = page.Context(ctx).NavigateForward()
	case "reload":
		err = page.Context(ctx).Reload()
	default:
		return fmt.Errorf("history action must be back, forward, or reload")
	}
	if err != nil {
		return err
	}
	if info, infoErr := page.Context(ctx).Info(); infoErr == nil && info != nil {
		m.UpdateMetadata(sessionID, func(session Session) Session {
			session.URL = m.redactor.SanitizeString(info.URL)
			session.Title = m.redactor.SanitizeString(info.Title)
			return session
		})
	}
	return nil
}

func (m *SessionManager) resolveElementRef(ctx context.Context, sessionID, ref string) (*rod.Element, ElementFingerprint, error) {
	page, ok := m.Page(sessionID)
	if !ok || page == nil {
		return nil, ElementFingerprint{}, fmt.Errorf("unknown session: %s", sessionID)
	}
	registry := m.Registry(sessionID)
	fingerprint, ok := registry.Get(ref)
	if !ok || fingerprint.Generation != registry.Generation() {
		return nil, ElementFingerprint{}, fmt.Errorf("stale or unknown element ref %s; observe the page again", ref)
	}

	selectors := append([]string{fingerprint.Selector}, fingerprint.AltSelectors...)
	seen := make(map[string]struct{}, len(selectors))
	for _, selector := range selectors {
		selector = strings.TrimSpace(selector)
		if selector == "" {
			continue
		}
		if _, duplicate := seen[selector]; duplicate {
			continue
		}
		seen[selector] = struct{}{}
		elements, err := page.Context(ctx).Elements(selector)
		if err != nil {
			continue
		}
		if element := bestFingerprintMatch(elements, fingerprint); element != nil {
			return element, fingerprint, nil
		}
	}
	if fingerprint.TagName != "" {
		elements, err := page.Context(ctx).Elements(fingerprint.TagName)
		if err == nil {
			if len(elements) > 200 {
				elements = elements[:200]
			}
			if element := bestFingerprintMatch(elements, fingerprint); element != nil {
				return element, fingerprint, nil
			}
		}
	}
	return nil, ElementFingerprint{}, fmt.Errorf("element ref %s no longer matches the page; observe again", ref)
}

func bestFingerprintMatch(elements rod.Elements, fingerprint ElementFingerprint) *rod.Element {
	bestScore := -1
	var best *rod.Element
	for _, element := range elements {
		score := fingerprintScore(element, fingerprint)
		if score > bestScore {
			bestScore = score
			best = element
		}
	}
	if bestScore < 1 {
		return nil
	}
	return best
}

func fingerprintScore(element *rod.Element, fingerprint ElementFingerprint) int {
	if element == nil {
		return -1
	}
	tagValue, err := element.Property("tagName")
	if err != nil {
		return -1
	}
	tag := strings.ToLower(tagValue.Str())
	if fingerprint.TagName != "" && tag != strings.ToLower(fingerprint.TagName) {
		return -1
	}
	score := 1
	checks := []struct {
		name, expected string
		weight         int
		strong         bool
	}{
		{"id", fingerprint.ID, 100, true}, {"data-testid", fingerprint.DataTestID, 90, true},
		{"name", fingerprint.Name, 60, true}, {"aria-label", fingerprint.AriaLabel, 50, true},
		{"role", fingerprint.Role, 15, false}, {"type", fingerprint.InputType, 15, false},
	}
	for _, check := range checks {
		if check.expected == "" {
			continue
		}
		actual, attrErr := element.Attribute(check.name)
		if check.name == "data-testid" && (attrErr != nil || actual == nil) {
			actual, attrErr = element.Attribute("data-test-id")
		}
		if attrErr != nil || actual == nil || *actual != check.expected {
			if check.strong {
				return -1
			}
			continue
		}
		score += check.weight
	}
	if fingerprint.TextContent != "" {
		if text, textErr := element.Text(); textErr == nil && strings.Contains(strings.TrimSpace(text), strings.TrimSpace(fingerprint.TextContent)) {
			score += 20
		}
	}
	return score
}

func namedKey(raw string) (input.Key, bool) {
	switch strings.ToLower(raw) {
	case "enter", "return":
		return input.Enter, true
	case "tab":
		return input.Tab, true
	case "escape", "esc":
		return input.Escape, true
	case "space":
		return input.Space, true
	case "backspace":
		return input.Backspace, true
	case "delete":
		return input.Delete, true
	case "home":
		return input.Home, true
	case "end":
		return input.End, true
	case "pageup":
		return input.PageUp, true
	case "pagedown":
		return input.PageDown, true
	case "arrowleft", "left":
		return input.ArrowLeft, true
	case "arrowright", "right":
		return input.ArrowRight, true
	case "arrowup", "up":
		return input.ArrowUp, true
	case "arrowdown", "down":
		return input.ArrowDown, true
	}
	runeValue, size := utf8.DecodeRuneInString(raw)
	if runeValue != utf8.RuneError && size == len(raw) {
		return input.Key(runeValue), true
	}
	return 0, false
}

func modifierKey(raw string) (input.Key, bool) {
	switch strings.ToLower(raw) {
	case "control", "ctrl":
		return input.ControlLeft, true
	case "shift":
		return input.ShiftLeft, true
	case "alt", "option":
		return input.AltLeft, true
	case "meta", "command", "cmd":
		return input.MetaLeft, true
	default:
		return 0, false
	}
}

func pressAndRelease(keyboard *rod.Keyboard, key input.Key) error {
	if err := keyboard.Press(key); err != nil {
		return err
	}
	return keyboard.Release(key)
}

func releaseKeys(keyboard *rod.Keyboard, keys []input.Key) error {
	var first error
	for i := len(keys) - 1; i >= 0; i-- {
		if err := keyboard.Release(keys[i]); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func sleepWithContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func requireSession(sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return fmt.Errorf("session_id is required")
	}
	return nil
}

func sanitizeSession(m *SessionManager, session Session) Session {
	session.URL = m.SanitizeForEvidence(session.URL)
	session.Title = m.SanitizeForEvidence(session.Title)
	return session
}
