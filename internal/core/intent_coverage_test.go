package core

import (
	"testing"
)

// --- isSupportedVerb ---

// --- inferCategoryFromVerb ---

// --- inferTargetFromText ---

// --- splitFirstToken ---

// --- InferIntentFromTask additional cases ---

func TestIntentDefaults(t *testing.T) {
	files := DefaultIntentSchemaFiles()
	if len(files) == 0 {
		t.Error("expected non-empty default intent schema files")
	}
	for _, f := range files {
		if f == "" {
			t.Error("expected non-empty schema file path")
		}
	}

	preds := defaultIntentFactPredicates()
	if len(preds) == 0 {
		t.Error("expected non-empty default intent fact predicates")
	}
	if _, ok := preds["intent_definition"]; !ok {
		t.Error("expected 'intent_definition' in default intent fact predicates")
	}
}
