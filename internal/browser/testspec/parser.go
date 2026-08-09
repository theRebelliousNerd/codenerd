package testspec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"codenerd/internal/browser"

	"gopkg.in/yaml.v3"
)

const (
	maxYAMLNodes  = 10000
	maxYAMLDepth  = 40
	maxURLBytes   = 4096
	maxValueBytes = 64 << 10
)

// ParseYAML strictly decodes and normalizes one bounded fixture document.
func ParseYAML(raw string) (Spec, error) {
	var spec Spec
	if len(raw) == 0 {
		return spec, fmt.Errorf("test_yaml is required")
	}
	if len(raw) > MaxFixtureBytes {
		return spec, fmt.Errorf("test_yaml exceeds %d bytes", MaxFixtureBytes)
	}
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(raw), &node); err != nil {
		return spec, fmt.Errorf("parse test_yaml: %w", err)
	}
	if err := validateYAMLNode(&node, 0, new(int)); err != nil {
		return spec, err
	}
	decoder := yaml.NewDecoder(strings.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(&spec); err != nil {
		return spec, fmt.Errorf("decode test_yaml: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return spec, fmt.Errorf("test_yaml must contain exactly one document")
		}
		return spec, fmt.Errorf("decode trailing test_yaml: %w", err)
	}
	if err := Normalize(&spec); err != nil {
		return spec, err
	}
	return spec, nil
}

// ParseValue strictly decodes a JSON-compatible tool argument.
func ParseValue(value any) (Spec, error) {
	var spec Spec
	encoded, err := json.Marshal(value)
	if err != nil {
		return spec, fmt.Errorf("encode test: %w", err)
	}
	if len(encoded) > MaxFixtureBytes {
		return spec, fmt.Errorf("test exceeds %d bytes", MaxFixtureBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&spec); err != nil {
		return spec, fmt.Errorf("decode test: %w", err)
	}
	if err := Normalize(&spec); err != nil {
		return spec, err
	}
	return spec, nil
}

// MarshalYAML encodes a normalized portable fixture.
func MarshalYAML(spec Spec) (string, error) {
	if err := Normalize(&spec); err != nil {
		return "", err
	}
	encoded, err := yaml.Marshal(spec)
	if err != nil {
		return "", fmt.Errorf("encode test_yaml: %w", err)
	}
	if len(encoded) > MaxFixtureBytes {
		return "", fmt.Errorf("encoded test_yaml exceeds %d bytes", MaxFixtureBytes)
	}
	return string(encoded), nil
}

// Normalize fills assertion defaults and rejects non-portable operations.
func Normalize(spec *Spec) error {
	if spec == nil {
		return fmt.Errorf("test is required")
	}
	spec.Name = strings.TrimSpace(spec.Name)
	if spec.Name == "" {
		spec.Name = "browser test"
	}
	if len(spec.Name) > MaxNameBytes {
		return fmt.Errorf("test name exceeds %d bytes", MaxNameBytes)
	}
	spec.SessionID = strings.TrimSpace(spec.SessionID)
	if len(spec.Actions) > MaxActions {
		return fmt.Errorf("actions exceeds limit of %d", MaxActions)
	}
	if len(spec.Assertions) == 0 {
		return fmt.Errorf("test has no assertions")
	}
	if len(spec.Assertions) > MaxAssertions {
		return fmt.Errorf("assertions exceeds limit of %d", MaxAssertions)
	}
	for index := range spec.Actions {
		if err := normalizeAction(&spec.Actions[index], index); err != nil {
			return err
		}
	}
	actionTest := len(spec.Actions) > 0
	for index := range spec.Assertions {
		assertion := &spec.Assertions[index]
		assertion.Name = strings.TrimSpace(assertion.Name)
		if assertion.Name == "" {
			assertion.Name = fmt.Sprintf("assertion_%d", index+1)
		}
		if len(assertion.Name) > MaxNameBytes {
			return fmt.Errorf("assertions[%d].name exceeds %d bytes", index, MaxNameBytes)
		}
		assertion.Query = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(assertion.Query), "."))
		if assertion.Query == "" || len(assertion.Query) > MaxQueryBytes {
			return fmt.Errorf("assertions[%d].query must contain at most %d bytes", index, MaxQueryBytes)
		}
		assertion.Expect = strings.ToLower(strings.TrimSpace(assertion.Expect))
		if assertion.Expect == "" {
			assertion.Expect = "present"
		}
		if assertion.Expect != "present" && assertion.Expect != "absent" {
			return fmt.Errorf("assertions[%d].expect must be present or absent", index)
		}
		assertion.Scope = strings.ToLower(strings.TrimSpace(assertion.Scope))
		if assertion.Scope == "" {
			if actionTest {
				assertion.Scope = "fresh"
			} else {
				assertion.Scope = "current"
			}
		}
		if assertion.Scope != "fresh" && assertion.Scope != "current" {
			return fmt.Errorf("assertions[%d].scope must be fresh or current", index)
		}
	}
	return nil
}

// ResolveEnvironment returns an execution-only copy with value_env expanded.
func ResolveEnvironment(spec Spec) (Spec, error) {
	resolved := cloneSpec(spec)
	for actionIndex := range resolved.Actions {
		action := &resolved.Actions[actionIndex]
		if action.ValueEnv != "" {
			value, err := environmentValue(action.ValueEnv, fmt.Sprintf("actions[%d]", actionIndex))
			if err != nil {
				return Spec{}, err
			}
			action.Value = value
			action.ValueEnv = ""
		}
		for fieldIndex := range action.Fields {
			field := &action.Fields[fieldIndex]
			if field.ValueEnv == "" {
				continue
			}
			value, err := environmentValue(field.ValueEnv, fmt.Sprintf("actions[%d].fields[%d]", actionIndex, fieldIndex))
			if err != nil {
				return Spec{}, err
			}
			field.Value = value
			field.ValueEnv = ""
		}
	}
	return resolved, nil
}

func normalizeAction(action *browser.ActionOperation, index int) error {
	action.Type = strings.ToLower(strings.TrimSpace(action.Type))
	switch action.Type {
	case "navigate":
		action.URL = strings.TrimSpace(action.URL)
		if action.URL == "" || len(action.URL) > maxURLBytes {
			return fmt.Errorf("actions[%d].url is required and capped at %d bytes", index, maxURLBytes)
		}
	case "interact":
		if action.Ref != "" || action.Target == nil {
			return fmt.Errorf("actions[%d] must use a semantic target, not an opaque ref", index)
		}
		if err := action.Target.Validate(); err != nil {
			return fmt.Errorf("actions[%d].target: %w", index, err)
		}
		action.Action = strings.ToLower(strings.TrimSpace(action.Action))
		if action.Action == "" {
			action.Action = "click"
		}
		if action.Action != "click" && action.Action != "type" && action.Action != "select" && action.Action != "toggle" && action.Action != "clear" {
			return fmt.Errorf("actions[%d].action is unsupported", index)
		}
		needsValue := action.Action == "type" || action.Action == "select"
		if !needsValue && (action.Value != "" || action.ValueEnv != "") {
			return fmt.Errorf("actions[%d].action %s does not accept a value", index, action.Action)
		}
		if err := normalizeValue(action.Target, &action.Value, &action.ValueEnv, fmt.Sprintf("actions[%d]", index), needsValue); err != nil {
			return err
		}
	case "fill":
		if len(action.Fields) == 0 || len(action.Fields) > 50 {
			return fmt.Errorf("actions[%d].fields must contain 1 to 50 entries", index)
		}
		for fieldIndex := range action.Fields {
			field := &action.Fields[fieldIndex]
			if field.Ref != "" || field.Target == nil {
				return fmt.Errorf("actions[%d].fields[%d] must use a semantic target", index, fieldIndex)
			}
			if err := field.Target.Validate(); err != nil {
				return fmt.Errorf("actions[%d].fields[%d].target: %w", index, fieldIndex, err)
			}
			if err := normalizeValue(field.Target, &field.Value, &field.ValueEnv, fmt.Sprintf("actions[%d].fields[%d]", index, fieldIndex), true); err != nil {
				return err
			}
		}
		if action.SubmitButton != "" {
			return fmt.Errorf("actions[%d] must use submit_target, not submit_button", index)
		}
		if action.SubmitTarget != nil {
			if err := action.SubmitTarget.Validate(); err != nil {
				return fmt.Errorf("actions[%d].submit_target: %w", index, err)
			}
		}
	case "key":
		if strings.TrimSpace(action.Key) == "" || len(action.Key) > 64 {
			return fmt.Errorf("actions[%d].key is required and capped at 64 bytes", index)
		}
	case "history":
		action.Action = strings.ToLower(strings.TrimSpace(action.Action))
		if action.Action != "back" && action.Action != "forward" && action.Action != "reload" {
			return fmt.Errorf("actions[%d].action must be back, forward, or reload", index)
		}
	case "sleep":
		if action.DurationMS < 0 || action.DurationMS > 30000 {
			return fmt.Errorf("actions[%d].duration_ms must be between 0 and 30000", index)
		}
	default:
		return fmt.Errorf("actions[%d].type %q is not portable", index, action.Type)
	}
	return nil
}

func normalizeValue(target *browser.ElementMatcher, value, valueEnv *string, location string, required bool) error {
	*valueEnv = strings.TrimSpace(*valueEnv)
	if *value != "" && *valueEnv != "" {
		return fmt.Errorf("%s cannot set both value and value_env", location)
	}
	if len(*value) > maxValueBytes {
		return fmt.Errorf("%s.value exceeds %d bytes", location, maxValueBytes)
	}
	if *valueEnv != "" && !validEnvironmentName(*valueEnv) {
		return fmt.Errorf("%s has invalid value_env name %q", location, *valueEnv)
	}
	if target != nil && target.IsSensitive() && *value != "" {
		return fmt.Errorf("%s targets a sensitive field and must use value_env", location)
	}
	if required && *value == "" && *valueEnv == "" {
		return fmt.Errorf("%s requires value or value_env", location)
	}
	return nil
}

func validEnvironmentName(name string) bool {
	for index, char := range name {
		if char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' || char == '_' || index > 0 && char >= '0' && char <= '9' {
			continue
		}
		return false
	}
	return name != ""
}

func environmentValue(name, location string) (string, error) {
	if !validEnvironmentName(name) {
		return "", fmt.Errorf("%s has invalid value_env name %q", location, name)
	}
	value, ok := os.LookupEnv(name)
	if !ok {
		return "", fmt.Errorf("%s requires environment variable %s", location, name)
	}
	if value == "" {
		return "", fmt.Errorf("%s requires non-empty environment variable %s", location, name)
	}
	if len(value) > maxValueBytes {
		return "", fmt.Errorf("%s environment variable %s exceeds %d bytes", location, name, maxValueBytes)
	}
	return value, nil
}

func cloneSpec(spec Spec) Spec {
	copySpec := spec
	copySpec.Actions = append([]browser.ActionOperation(nil), spec.Actions...)
	for index := range copySpec.Actions {
		action := &copySpec.Actions[index]
		action.Fields = append([]browser.FillField(nil), action.Fields...)
		if action.Target != nil {
			copy := *action.Target
			action.Target = &copy
		}
		if action.SubmitTarget != nil {
			copy := *action.SubmitTarget
			action.SubmitTarget = &copy
		}
		for fieldIndex := range action.Fields {
			if action.Fields[fieldIndex].Target != nil {
				copy := *action.Fields[fieldIndex].Target
				action.Fields[fieldIndex].Target = &copy
			}
		}
	}
	copySpec.Assertions = append([]Assertion(nil), spec.Assertions...)
	return copySpec
}

func validateYAMLNode(node *yaml.Node, depth int, count *int) error {
	if node == nil {
		return nil
	}
	*count++
	if *count > maxYAMLNodes {
		return fmt.Errorf("test_yaml exceeds YAML node limit of %d", maxYAMLNodes)
	}
	if depth > maxYAMLDepth {
		return fmt.Errorf("test_yaml exceeds YAML depth limit of %d", maxYAMLDepth)
	}
	if node.Kind == yaml.AliasNode || node.Alias != nil {
		return fmt.Errorf("test_yaml aliases are not allowed")
	}
	for _, child := range node.Content {
		if err := validateYAMLNode(child, depth+1, count); err != nil {
			return err
		}
	}
	return nil
}
