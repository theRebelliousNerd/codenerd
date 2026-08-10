package prompt

import (
	"strings"
	"testing"
)

func TestVerifyPE1ModelsProviders(t *testing.T) {
	// NormalizeSelectorAtom tests
	if got := NormalizeSelectorAtom("muse-spark-1.2-contributor"); got != "/muse_spark_1_2_contributor" {
		t.Fatalf("NormalizeSelectorAtom muse-spark... = %q, want /muse_spark_1_2_contributor", got)
	}
	if got := NormalizeSelectorAtom("meta"); got != "/meta" {
		t.Fatalf("NormalizeSelectorAtom meta = %q, want /meta", got)
	}
	if got := NormalizeSelectorAtom("/muse_spark_1_2_contributor"); got != "/muse_spark_1_2_contributor" {
		t.Fatalf("idempotent failed: %q", got)
	}
	if got := NormalizeSelectorAtom("/meta"); got != "/meta" {
		t.Fatalf("idempotent /meta failed: %q", got)
	}
	// Valid atom with models/providers round-trip
	dataValid := []byte(`
id: test/models
category: methodology
priority: 50
is_mandatory: false
content: body
models: [/muse_spark_1_2_contributor]
providers: [/meta]
`)
	parsed, _, err := ParsePromptAtomYAML(dataValid, "test.yaml", nil)
	if err != nil {
		t.Fatalf("valid atom failed: %v", err)
	}
	atom := parsed[0].Atom
	if len(atom.Models) != 1 || atom.Models[0] != "muse_spark_1_2_contributor" {
		t.Fatalf("models round-trip got %v want [muse_spark_1_2_contributor]", atom.Models)
	}
	if len(atom.Providers) != 1 || atom.Providers[0] != "meta" {
		t.Fatalf("providers round-trip got %v", atom.Providers)
	}
	// Neither
	dataNeither := []byte(`
id: test/neither
category: methodology
priority: 50
is_mandatory: false
content: body
`)
	parsed, _, err = ParsePromptAtomYAML(dataNeither, "test2.yaml", nil)
	if err != nil {
		t.Fatalf("neither failed: %v", err)
	}
	atom = parsed[0].Atom
	if atom.Models != nil || atom.Providers != nil {
		t.Fatalf("neither should be nil, got models=%v providers=%v", atom.Models, atom.Providers)
	}
	// Invalid dot/dash
	dataInvalidDot := []byte(`
id: test/invalid-dot
category: methodology
priority: 50
is_mandatory: false
content: body
models: [/muse-spark-1.2-contributor]
`)
	_, _, err = ParsePromptAtomYAML(dataInvalidDot, "test.yaml", nil)
	if err == nil {
		t.Fatalf("should reject dot/dash")
	}
	if !strings.Contains(err.Error(), "/muse_spark_1_2_contributor") {
		t.Fatalf("error should contain suggestion, got %v", err)
	}
	if !strings.Contains(err.Error(), "muse-spark-1.2-contributor") {
		t.Fatalf("error should contain offending, got %v", err)
	}
	// Missing slash
	dataMissingSlash := []byte(`
id: test/missing-slash
category: methodology
priority: 50
is_mandatory: false
content: body
models: [muse_spark_1_2_contributor]
`)
	_, _, err = ParsePromptAtomYAML(dataMissingSlash, "test.yaml", nil)
	if err == nil {
		t.Fatalf("should reject missing slash")
	}
	if !strings.Contains(err.Error(), "/muse_spark_1_2_contributor") {
		t.Fatalf("missing slash error should contain suggestion, got %v", err)
	}
	// Provider missing slash
	dataProvMissing := []byte(`
id: test/prov-missing
category: methodology
priority: 50
is_mandatory: false
content: body
providers: [meta]
`)
	_, _, err = ParsePromptAtomYAML(dataProvMissing, "test.yaml", nil)
	if err == nil {
		t.Fatalf("should reject provider missing slash")
	}
}
