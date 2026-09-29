package main

import (
	"strings"
	"testing"
)

func errorMessages(issues []issue) string {
	var sb strings.Builder
	for _, i := range issues {
		if i.Severity == severityError {
			sb.WriteString(i.Message)
			sb.WriteString("; ")
		}
	}
	return sb.String()
}

func TestValidateAtomDef(t *testing.T) {
	cats := map[string]struct{}{"identity": {}}
	opts := validationOptions{}
	prio := 50
	mand := true
	valid := atomDefinition{ID: "a1", Category: "identity", Priority: &prio, IsMandatory: &mand, Content: "hello"}

	if msg := errorMessages(validateAtomDef("p.yaml", "p.yaml", valid, cats, opts)); msg != "" {
		t.Errorf("a valid atom produced errors: %s", msg)
	}

	checks := []struct {
		name   string
		mutate func(d *atomDefinition)
		want   string
	}{
		{"missing id", func(d *atomDefinition) { d.ID = "" }, "missing required field: id"},
		{"whitespace id", func(d *atomDefinition) { d.ID = "has space" }, "whitespace"},
		{"unknown category", func(d *atomDefinition) { d.Category = "bogus" }, "unknown category"},
		{"missing content", func(d *atomDefinition) { d.Content = "" }, "content or content_file"},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			d := valid
			c.mutate(&d)
			if msg := errorMessages(validateAtomDef("p", "p", d, cats, opts)); !strings.Contains(msg, c.want) {
				t.Errorf("expected error containing %q, got %q", c.want, msg)
			}
		})
	}

	// Priority out of range is flagged.
	bad := 999
	d := valid
	d.Priority = &bad
	if !strings.Contains(errorMessages(validateAtomDef("p", "p", d, cats, opts)), "out of range") {
		t.Error("priority out of range not flagged")
	}
}

func TestValidateAtomDef_RequiresToolsUnknown(t *testing.T) {
	cats := map[string]struct{}{"identity": {}}
	opts := validationOptions{}
	prio := 50
	mand := true
	base := atomDefinition{ID: "a1", Category: "identity", Priority: &prio, IsMandatory: &mand, Content: "hello"}

	bad := base
	bad.RequiresTools = []string{"begin_transaction", "read_file"}
	if msg := errorMessages(validateAtomDef("p.yaml", "p.yaml", bad, cats, opts)); !strings.Contains(msg, "unknown tool") || !strings.Contains(msg, "begin_transaction") {
		t.Errorf("validateAtomDef should report unknown tool begin_transaction, got %q", msg)
	}

	good := base
	good.RequiresTools = []string{"read_file", "find_symbol"}
	if msg := errorMessages(validateAtomDef("p.yaml", "p.yaml", good, cats, opts)); msg != "" {
		t.Errorf("validateAtomDef with known tools produced errors: %s", msg)
	}
}

func TestValidateRequiresToolsList_UnknownTool(t *testing.T) {
	cases := []struct {
		name       string
		tools      []string
		wantErr    bool
		wantSubstr string
	}{
		{name: "unknown single", tools: []string{"begin_transaction", "read_file"}, wantErr: true, wantSubstr: "begin_transaction"},
		{name: "all removed transaction tools", tools: []string{"begin_transaction", "add_edit", "prepare", "commit", "abort"}, wantErr: true, wantSubstr: "unknown tool"},
		{name: "known tools pass", tools: []string{"read_file", "find_symbol", "get_element"}, wantErr: false},
		{name: "empty passes", tools: nil, wantErr: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			issues := validateRequiresToolsList("test.yaml", "test-atom", tc.tools)
			if !tc.wantErr && len(issues) != 0 {
				t.Fatalf("expected no issues for %v, got %v", tc.tools, issues)
			}
			if tc.wantErr {
				if len(issues) == 0 {
					t.Fatalf("expected unknown-tool issue for %v, got none", tc.tools)
				}
				found := false
				for _, it := range issues {
					if it.Severity != severityError {
						t.Errorf("expected severity error, got %q for %v", it.Severity, it)
					}
					if tc.wantSubstr != "" && strings.Contains(it.Message, tc.wantSubstr) {
						found = true
					}
					if !strings.Contains(it.Message, "unknown tool") {
						t.Errorf("expected message to contain %q, got %q", "unknown tool", it.Message)
					}
				}
				if tc.wantSubstr != "" && !found {
					t.Errorf("expected message containing %q, got %v", tc.wantSubstr, errorMessages(issues))
				}
			}
		})
	}
}

func TestValidateRequiresToolsList_Branches(t *testing.T) {
	t.Run("empty value", func(t *testing.T) {
		issues := validateRequiresToolsList("test.yaml", "test-atom", []string{""})
		if len(issues) != 1 {
			t.Fatalf("expected 1 issue for empty value, got %v", errorMessages(issues))
		}
		if issues[0].Severity != severityError {
			t.Errorf("expected severity error, got %q", issues[0].Severity)
		}
		if got := issues[0].Message; got != "requires_tools contains an empty value" {
			t.Errorf("unexpected message: %q", got)
		}
	})

	t.Run("surrounding whitespace", func(t *testing.T) {
		for _, v := range []string{" read_file", "read_file ", "  read_file  "} {
			issues := validateRequiresToolsList("test.yaml", "test-atom", []string{v})
			if len(issues) != 1 {
				t.Fatalf("expected 1 issue for %q, got %v", v, errorMessages(issues))
			}
			if issues[0].Severity != severityError {
				t.Errorf("expected severity error for %q, got %q", v, issues[0].Severity)
			}
			if got := issues[0].Message; !strings.Contains(got, "surrounding whitespace") || !strings.Contains(got, v) {
				t.Errorf("expected surrounding-whitespace message naming %q, got %q", v, got)
			}
		}
	})

	t.Run("slash prefixed", func(t *testing.T) {
		issues := validateRequiresToolsList("test.yaml", "test-atom", []string{"/read_file"})
		if len(issues) != 1 {
			t.Fatalf("expected 1 issue for slash-prefixed value, got %v", errorMessages(issues))
		}
		if issues[0].Severity != severityError {
			t.Errorf("expected severity error, got %q", issues[0].Severity)
		}
		if got := issues[0].Message; !strings.Contains(got, "must not be slash-prefixed") || !strings.Contains(got, "/read_file") {
			t.Errorf("unexpected message: %q", got)
		}
	})

	t.Run("invalid characters", func(t *testing.T) {
		for _, v := range []string{"Read_File", "read-file", "read.file", "read file", "read!file"} {
			issues := validateRequiresToolsList("test.yaml", "test-atom", []string{v})
			if len(issues) != 1 {
				t.Fatalf("expected exactly 1 issue for %q (invalid skipped, no unknown), got %v", v, errorMessages(issues))
			}
			if issues[0].Severity != severityError {
				t.Errorf("expected severity error for %q, got %q", v, issues[0].Severity)
			}
			if got := issues[0].Message; !strings.Contains(got, "must match [a-z0-9_]+") || !strings.Contains(got, v) {
				t.Errorf("expected format message naming %q, got %q", v, got)
			}
			if got := issues[0].Message; strings.Contains(got, "unknown tool") {
				t.Errorf("invalid value %q must report format, not unknown tool, got %q", v, got)
			}
		}
	})

	t.Run("duplicate value", func(t *testing.T) {
		issues := validateRequiresToolsList("test.yaml", "test-atom", []string{"read_file", "read_file"})
		if len(issues) != 1 {
			t.Fatalf("expected 1 issue for duplicate value, got %v", errorMessages(issues))
		}
		if issues[0].Severity != severityError {
			t.Errorf("expected severity error, got %q", issues[0].Severity)
		}
		if got := issues[0].Message; !strings.Contains(got, "duplicate value") || !strings.Contains(got, "read_file") {
			t.Errorf("unexpected message: %q", got)
		}
	})

	t.Run("duplicate of unknown reports duplicate once", func(t *testing.T) {
		issues := validateRequiresToolsList("test.yaml", "test-atom", []string{"begin_transaction", "begin_transaction"})
		if len(issues) != 2 {
			t.Fatalf("expected 2 issues (unknown + duplicate), got %v", errorMessages(issues))
		}
		if got := issues[0].Message; !strings.Contains(got, "unknown tool") || !strings.Contains(got, "begin_transaction") {
			t.Errorf("first issue should be unknown tool, got %q", got)
		}
		if got := issues[1].Message; !strings.Contains(got, "duplicate value") {
			t.Errorf("second issue should be duplicate value, got %q", got)
		}
	})
}
