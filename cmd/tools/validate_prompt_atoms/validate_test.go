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
