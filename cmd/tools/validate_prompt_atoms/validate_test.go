package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/tools/catalog"
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

	// TODO: Test Null/Undefined/Empty Fields - test pointers being nil versus explicitly pointing to zero values
	// TODO: Test Null/Undefined/Empty Fields - test what happens when Priority or IsMandatory are nil pointers
	// TODO: Test Type Coercion - simulate YAML parser injecting string instead of int for Priority
	// TODO: Test Type Coercion - simulate YAML parser injecting integer instead of string for ID
	// TODO: Test User Request Extremes - test Content string payload of 50MB to evaluate OOM safety
	// TODO: Test State Conflicts - test DependsOn and ConflictsWith simultaneously referencing the same atom
	// TODO: Test State Conflicts - test ContentFile pointing to a directory or broken symlink instead of regular file
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
	// TODO: Test Null/Undefined/Empty Fields - test RequiresTools slice containing explicit nil or empty slice vs uninitialized nil slice
	// TODO: Test Null/Undefined/Empty Fields - test RequiresTools list containing mix of null, empty strings, and valid values
	// TODO: Test User Request Extremes - test RequiresTools containing 100,000 duplicated tool names to check performance
	// TODO: Test Type Coercion - simulate YAML parser injecting bool into RequiresTools list
	// TODO: Test Type Coercion - simulate YAML parser injecting struct/map into RequiresTools list
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

// The validator's known set is catalog.Names. Every name production can
// register is accepted; a name outside that set is refused.
func TestValidateRequiresToolsList_UsesProductionCatalog(t *testing.T) {
	names, err := catalog.Names()
	if err != nil {
		t.Fatalf("catalog.Names: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("catalog.Names returned no tools")
	}
	if issues := validateRequiresToolsList("test.yaml", "test-atom", names); len(issues) != 0 {
		t.Fatalf("registered tools refused: %s", errorMessages(issues))
	}

	const outsider = "begin_transaction"
	for _, name := range names {
		if name == outsider {
			t.Fatalf("catalog.Names contains %q", outsider)
		}
	}
	issues := validateRequiresToolsList("test.yaml", "test-atom", []string{outsider})
	if len(issues) != 1 {
		t.Fatalf("expected one issue for %q, got %s", outsider, errorMessages(issues))
	}
	if issues[0].Severity != severityError || !strings.Contains(issues[0].Message, "unknown tool") || !strings.Contains(issues[0].Message, outsider) {
		t.Fatalf("unexpected issue: %+v", issues[0])
	}
}

func TestValidateAtomTree_RootErrors(t *testing.T) {
	dir := t.TempDir()
	missing := dir + "/does-not-exist"
	if _, _, err := validateAtomTree(missing, validationOptions{}); err == nil {
		t.Fatalf("expected error for missing root, got nil")
	}
	filePath := dir + "/file.yaml"
	if err := os.WriteFile(filePath, []byte("id: /x\ncategory: protocol\nis_mandatory: true\ncontent: hi\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if _, _, err := validateAtomTree(filePath, validationOptions{}); err == nil {
		t.Fatalf("expected error when root is a file, got nil")
	} else if !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("expected not-a-directory error, got %v", err)
	}
}

func TestValidateAtomTree_DuplicateAndDependencyBranches(t *testing.T) {
	writeAtom := func(t *testing.T, dir, name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	mkBody := func(id string, extra string) string {
		return "id: " + id + "\ncategory: protocol\nis_mandatory: true\ncontent: hello\n" + extra
	}

	t.Run("duplicate ids", func(t *testing.T) {
		dir := t.TempDir()
		writeAtom(t, dir, "a.yaml", mkBody("/dup", ""))
		writeAtom(t, dir, "b.yaml", mkBody("/dup", ""))
		issues, stats, err := validateAtomTree(dir, validationOptions{})
		if err != nil {
			t.Fatalf("validateAtomTree: %v", err)
		}
		if stats.Files != 2 || stats.Atoms != 2 {
			t.Fatalf("expected 2 files/atoms, got %+v", stats)
		}
		found := false
		for _, it := range issues {
			if it.Severity == severityError && strings.Contains(it.Message, "duplicate atom id") {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected duplicate atom id error, got %+v", issues)
		}
	})

	t.Run("depends_on branches", func(t *testing.T) {
		dir := t.TempDir()
		writeAtom(t, dir, "base.yaml", mkBody("/base", ""))
		// An empty depends_on entry is rejected by the shared strict parser
		// before cross-file validation runs, so it surfaces as a parse issue.
		// A self-dependency fails the same way, so the missing reference that
		// exercises the cross-file check lives in its own file.
		writeAtom(t, dir, "empty.yaml", "id: /empty\ncategory: protocol\nis_mandatory: true\ncontent: hi\ndepends_on:\n  - \"\"\n")
		writeAtom(t, dir, "self.yaml", "id: /selfdep\ncategory: protocol\nis_mandatory: true\ncontent: hi\ndepends_on:\n  - /selfdep\n")
		writeAtom(t, dir, "missing.yaml", "id: /needs-missing\ncategory: protocol\nis_mandatory: true\ncontent: hi\ndepends_on:\n  - /missing-dep\n")
		issues, _, err := validateAtomTree(dir, validationOptions{})
		if err != nil {
			t.Fatalf("validateAtomTree: %v", err)
		}
		foundParse := false
		for _, it := range issues {
			if it.Severity == severityError && strings.Contains(it.Message, "YAML parse failed") && strings.Contains(it.Message, "empty value") {
				foundParse = true
			}
		}
		if !foundParse {
			t.Fatalf("expected empty depends_on parse issue, got %+v", issues)
		}
		foundSelf := false
		for _, it := range issues {
			if it.Severity == severityError && strings.Contains(it.Message, "cannot depend on itself") {
				foundSelf = true
			}
		}
		if !foundSelf {
			t.Fatalf("expected self-dependency parse issue, got %+v", issues)
		}
		foundMissing := false
		for _, it := range issues {
			if it.Severity == severityError && strings.Contains(it.Message, "depends_on references missing atom") {
				foundMissing = true
			}
		}
		if !foundMissing {
			t.Fatalf("expected missing depends_on issue, got %+v", issues)
		}
	})

	t.Run("conflicts_with branches", func(t *testing.T) {
		dir := t.TempDir()
		writeAtom(t, dir, "base.yaml", mkBody("/base", ""))
		// Empty and self conflicts are rejected by the shared parser as parse
		// issues, so the missing reference that exercises the cross-file
		// warning lives in its own file.
		writeAtom(t, dir, "empty.yaml", "id: /empty\ncategory: protocol\nis_mandatory: true\ncontent: hi\nconflicts_with:\n  - \"\"\n")
		writeAtom(t, dir, "self.yaml", "id: /selfconf\ncategory: protocol\nis_mandatory: true\ncontent: hi\nconflicts_with:\n  - /selfconf\n")
		writeAtom(t, dir, "missing.yaml", "id: /needs-missing\ncategory: protocol\nis_mandatory: true\ncontent: hi\nconflicts_with:\n  - /missing-conflict\n")
		issues, _, err := validateAtomTree(dir, validationOptions{})
		if err != nil {
			t.Fatalf("validateAtomTree: %v", err)
		}
		foundParse := false
		for _, it := range issues {
			if it.Severity == severityError && strings.Contains(it.Message, "YAML parse failed") && strings.Contains(it.Message, "empty value") {
				foundParse = true
			}
		}
		if !foundParse {
			t.Fatalf("expected empty conflicts_with parse issue, got %+v", issues)
		}
		foundSelf := false
		for _, it := range issues {
			if it.Severity == severityError && strings.Contains(it.Message, "cannot conflict with itself") {
				foundSelf = true
			}
		}
		if !foundSelf {
			t.Fatalf("expected self-conflict parse issue, got %+v", issues)
		}
		foundWarn := false
		for _, it := range issues {
			if it.Severity == severityWarning && strings.Contains(it.Message, "conflicts_with references missing atom") {
				foundWarn = true
			}
		}
		if !foundWarn {
			t.Fatalf("expected missing-conflict warning, got %+v", issues)
		}
	})

	t.Run("skips non-yaml and counts stats", func(t *testing.T) {
		dir := t.TempDir()
		writeAtom(t, dir, "a.yaml", mkBody("/a", ""))
		if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore me"), 0644); err != nil {
			t.Fatalf("write txt: %v", err)
		}
		issues, stats, err := validateAtomTree(dir, validationOptions{})
		if err != nil {
			t.Fatalf("validateAtomTree: %v", err)
		}
		if len(issues) != 0 {
			t.Fatalf("expected no issues, got %+v", issues)
		}
		if stats.Files != 1 || stats.Atoms != 1 {
			t.Fatalf("expected 1 file/atom, got %+v", stats)
		}
		if len(stats.AtomIDs) != 1 || stats.AtomIDs[0] != "/a" {
			t.Fatalf("expected AtomIDs [/a], got %v", stats.AtomIDs)
		}
	})
}

func TestValidateAtomDef_ContentFileBranches(t *testing.T) {
	dir := t.TempDir()
	valid := map[string]struct{}{"protocol": {}}
	opts := validationOptions{}
	newDef := func() atomDefinition {
		mand := true
		return atomDefinition{ID: "/a", Category: "protocol", IsMandatory: &mand, Content: "hello"}
	}

	t.Run("missing is_mandatory", func(t *testing.T) {
		d := newDef()
		d.IsMandatory = nil
		if msg := errorMessages(validateAtomDef("p.yaml", "p.yaml", d, valid, opts)); !strings.Contains(msg, "missing required field: is_mandatory") {
			t.Fatalf("expected is_mandatory error, got %q", msg)
		}
	})

	t.Run("missing category", func(t *testing.T) {
		d := newDef()
		d.Category = ""
		if msg := errorMessages(validateAtomDef("p.yaml", "p.yaml", d, valid, opts)); !strings.Contains(msg, "missing required field: category") {
			t.Fatalf("expected category error, got %q", msg)
		}
	})

	t.Run("both content and content_file warns", func(t *testing.T) {
		contentPath := filepath.Join(dir, "both.txt")
		if err := os.WriteFile(contentPath, []byte("body"), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
		d := newDef()
		d.ContentFile = "both.txt"
		issues := validateAtomDef(filepath.Join(dir, "a.yaml"), "a.yaml", d, valid, opts)
		found := false
		for _, it := range issues {
			if it.Severity == severityWarning && strings.Contains(it.Message, "content_file will be ignored") {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected both-content warning, got %+v", issues)
		}
	})

	t.Run("absolute content_file errors", func(t *testing.T) {
		d := newDef()
		d.Content = ""
		d.ContentFile = "/abs/path.txt"
		if msg := errorMessages(validateAtomDef("p.yaml", "p.yaml", d, valid, opts)); !strings.Contains(msg, "must be relative") {
			t.Fatalf("expected relative error, got %q", msg)
		}
	})

	t.Run("missing content_file errors", func(t *testing.T) {
		d := newDef()
		d.Content = ""
		d.ContentFile = "does-not-exist.txt"
		if msg := errorMessages(validateAtomDef(filepath.Join(dir, "a.yaml"), "a.yaml", d, valid, opts)); !strings.Contains(msg, "content_file read failed") {
			t.Fatalf("expected read-failed error, got %q", msg)
		}
	})

	t.Run("empty content_file errors", func(t *testing.T) {
		emptyPath := filepath.Join(dir, "empty.txt")
		if err := os.WriteFile(emptyPath, []byte("   \n"), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
		d := newDef()
		d.Content = ""
		d.ContentFile = "empty.txt"
		if msg := errorMessages(validateAtomDef(filepath.Join(dir, "a.yaml"), "a.yaml", d, valid, opts)); !strings.Contains(msg, "content_file is empty") {
			t.Fatalf("expected empty error, got %q", msg)
		}
	})
}

func TestValidateSelectorList_CanonicalBranches(t *testing.T) {
	t.Run("empty returns nil", func(t *testing.T) {
		if got := validateSelectorList("f", "/a", "operational_modes", nil, selectorStyleSlashPref, validationOptions{}); got != nil {
			t.Fatalf("expected nil, got %+v", got)
		}
	})

	t.Run("enforce canonical reports error", func(t *testing.T) {
		issues := validateSelectorList("f", "/a", "operational_modes", []string{"debug"}, selectorStyleSlashPref, validationOptions{EnforceCanonicalSelectors: true})
		if len(issues) != 1 || issues[0].Severity != severityError {
			t.Fatalf("expected one canonical error, got %+v", issues)
		}
		if !strings.Contains(issues[0].Message, "non-canonical") {
			t.Fatalf("expected non-canonical message, got %+v", issues)
		}
	})

	t.Run("silent when warnings disabled", func(t *testing.T) {
		issues := validateSelectorList("f", "/a", "operational_modes", []string{"debug"}, selectorStyleSlashPref, validationOptions{})
		if len(issues) != 0 {
			t.Fatalf("expected no issues when warnings disabled, got %+v", issues)
		}
	})

	t.Run("no-slash style flags slashed", func(t *testing.T) {
		opts := validationOptions{WarnNoncanonicalSelectors: true}
		issues := validateSelectorList("f", "/a", "world_states", []string{"/failing_tests"}, selectorStyleNoSlash, opts)
		if len(issues) != 1 || issues[0].Severity != severityWarning {
			t.Fatalf("expected one warning, got %+v", issues)
		}
		if ok := validateSelectorList("f", "/a", "world_states", []string{"failing_tests"}, selectorStyleNoSlash, opts); len(ok) != 0 {
			t.Fatalf("expected canonical bare value to pass, got %+v", ok)
		}
	})

	t.Run("any style never flags", func(t *testing.T) {
		opts := validationOptions{WarnNoncanonicalSelectors: true}
		if got := validateSelectorList("f", "/a", "field", []string{"x", "/y"}, selectorStyleAny, opts); len(got) != 0 {
			t.Fatalf("expected no issues for any style, got %+v", got)
		}
	})

	t.Run("trims before duplicate check", func(t *testing.T) {
		opts := validationOptions{}
		issues := validateSelectorList("f", "/a", "operational_modes", []string{"  /debug  ", "/debug"}, selectorStyleSlashPref, opts)
		found := false
		for _, it := range issues {
			if it.Severity == severityWarning && strings.Contains(it.Message, "duplicate value") {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected duplicate after trim, got %+v", issues)
		}
	})
}

func TestValidateWorldStatesKnownSet_Branches(t *testing.T) {
	t.Run("empty returns nil", func(t *testing.T) {
		if got := validateWorldStatesKnownSet("f", "/a", nil); got != nil {
			t.Fatalf("expected nil, got %+v", got)
		}
		if got := validateWorldStatesKnownSet("f", "/a", []string{}); got != nil {
			t.Fatalf("expected nil, got %+v", got)
		}
	})

	t.Run("known value passes", func(t *testing.T) {
		if got := validateWorldStatesKnownSet("f", "/a", []string{"failing_tests", "/diagnostics"}); len(got) != 0 {
			t.Fatalf("expected known states to pass, got %+v", got)
		}
	})

	t.Run("unknown warns and blank skipped", func(t *testing.T) {
		issues := validateWorldStatesKnownSet("f", "/a", []string{"", "   ", "/no-such-state"})
		if len(issues) != 1 {
			t.Fatalf("expected one warning, got %+v", issues)
		}
		if issues[0].Severity != severityWarning || !strings.Contains(issues[0].Message, "unknown value") {
			t.Fatalf("unexpected issue: %+v", issues[0])
		}
	})
}

func TestValidateRecommendedSelectors_Branches(t *testing.T) {
	mand := true
	base := atomDefinition{ID: "/a", Category: "protocol", IsMandatory: &mand, Content: "hi"}

	t.Run("empty atom id returns nil", func(t *testing.T) {
		d := base
		d.ID = ""
		if got := validateRecommendedSelectors("p", "campaign/x.yaml", "", d); len(got) != 0 {
			t.Fatalf("expected nil, got %+v", got)
		}
	})

	t.Run("unmatched path returns nil", func(t *testing.T) {
		if got := validateRecommendedSelectors("p", "misc/x.yaml", "/a", base); len(got) != 0 {
			t.Fatalf("expected nil, got %+v", got)
		}
	})

	t.Run("scoped dirs warn when field missing", func(t *testing.T) {
		cases := []struct {
			rel   string
			field string
		}{
			{"foo/campaign/x.yaml", "campaign_phases"},
			{"foo/init/x.yaml", "init_phases"},
			{"foo/northstar/x.yaml", "northstar_phases"},
			{"foo/ouroboros/x.yaml", "ouroboros_stages"},
			{"foo/build_layer/x.yaml", "build_layers"},
			{"foo/intent/x.yaml", "intent_verbs"},
			{"foo/language/x.yaml", "languages"},
			{"foo/framework/x.yaml", "frameworks"},
			{"foo/world_state/x.yaml", "world_states"},
			{"foo/shards/x.yaml", "shard_types"},
		}
		for _, tc := range cases {
			issues := validateRecommendedSelectors("p", tc.rel, "/a", base)
			if len(issues) != 1 || issues[0].Severity != severityWarning {
				t.Fatalf("%s: expected one warning, got %+v", tc.rel, issues)
			}
			if !strings.Contains(issues[0].Message, tc.field) {
				t.Fatalf("%s: expected field %q, got %+v", tc.rel, tc.field, issues)
			}
		}
	})

	t.Run("no warning when scoped field present", func(t *testing.T) {
		d := base
		d.CampaignPhases = []string{"/active"}
		if got := validateRecommendedSelectors("p", "foo/campaign/x.yaml", "/a", d); len(got) != 0 {
			t.Fatalf("expected no warning, got %+v", got)
		}
	})

	t.Run("path match is case insensitive", func(t *testing.T) {
		if got := validateRecommendedSelectors("p", "Foo/Campaign/x.yaml", "/a", base); len(got) != 1 {
			t.Fatalf("expected case-insensitive match, got %+v", got)
		}
	})
}

func TestParseAtomYAMLFile_MigrationWarning(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "legacy.yaml")
	body := "id: /legacy-priority\ncategory: protocol\nis_mandatory: true\ncontent: hello\npriority: 50\n"
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	defs, issues := parseAtomYAMLFile(path)
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}
	found := false
	for _, it := range issues {
		if it.Severity == severityWarning && strings.Contains(it.Message, "legacy-priority-ignored") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected legacy-priority migration warning, got %+v", issues)
	}
}
