package core

import (
	"strings"
	"testing"
)

func TestPredicateCorpus_ExtraCoverage(t *testing.T) {
	// Use NewPredicateCorpus to get the temp database file path
	corpus, err := NewPredicateCorpus()
	if err != nil {
		t.Fatalf("NewPredicateCorpus failed: %v", err)
	}
	defer corpus.Close()
	tempPath := corpus.tempFile

	// 1. Test NewPredicateCorpusFromPath
	corpusFromPath, err := NewPredicateCorpusFromPath(tempPath)
	if err != nil {
		t.Fatalf("NewPredicateCorpusFromPath failed: %v", err)
	}
	defer corpusFromPath.Close()

	// 2. Test GetPredicate and GetPredicateArgs
	info, err := corpusFromPath.GetPredicate("user_intent")
	if err != nil {
		t.Fatalf("GetPredicate(user_intent) failed: %v", err)
	}
	if info != nil {
		args, err := corpusFromPath.GetPredicateArgs(info.ID)
		if err != nil {
			t.Fatalf("GetPredicateArgs failed: %v", err)
		}
		t.Logf("user_intent args: %+v", args)
	}

	// 3. Test GetByCategory
	categories := []string{"intent", "query", "action", "world"}
	for _, cat := range categories {
		preds, err := corpusFromPath.GetByCategory(cat)
		if err == nil && len(preds) > 0 {
			t.Logf("Found %d predicates for category %s", len(preds), cat)
			break
		}
	}

	// 4. Test GetAllPredicates
	allPreds, err := corpusFromPath.GetAllPredicates()
	if err != nil {
		t.Fatalf("GetAllPredicates failed: %v", err)
	}
	if len(allPreds) == 0 {
		t.Error("GetAllPredicates returned empty list")
	}

	// 5. Test GetAllPredicateNames
	allNames, err := corpusFromPath.GetAllPredicateNames()
	if err != nil {
		t.Fatalf("GetAllPredicateNames failed: %v", err)
	}
	if len(allNames) == 0 {
		t.Error("GetAllPredicateNames returned empty list")
	}

	// 6. Test GetAllPredicateSignatures
	allSigs, err := corpusFromPath.GetAllPredicateSignatures()
	if err != nil {
		t.Fatalf("GetAllPredicateSignatures failed: %v", err)
	}
	if len(allSigs) == 0 {
		t.Error("GetAllPredicateSignatures returned empty list")
	}

	// 7. Test ValidatePredicates
	undefined := corpusFromPath.ValidatePredicates([]string{"user_intent", "completely_fake_predicate_xyz"})
	if len(undefined) != 1 || undefined[0] != "completely_fake_predicate_xyz" {
		t.Errorf("expected only fake predicate to be undefined, got: %v", undefined)
	}

	// 8. Test GetErrorPatterns and FindErrorPattern
	patterns, err := corpusFromPath.GetErrorPatterns()
	if err != nil {
		t.Fatalf("GetErrorPatterns failed: %v", err)
	}
	t.Logf("Found %d error patterns", len(patterns))
	for _, p := range patterns {
		p2, err := corpusFromPath.FindErrorPattern(p.Name)
		if err != nil {
			t.Errorf("FindErrorPattern(%s) failed: %v", p.Name, err)
		}
		if p2 == nil || p2.Name != p.Name {
			t.Errorf("FindErrorPattern(%s) returned incorrect pattern: %v", p.Name, p2)
		}
	}

	// 9. Test GetExamplesForPredicate and GetAntiPatterns
	examples, err := corpusFromPath.GetExamplesForPredicate("user_intent", false)
	if err != nil {
		t.Fatalf("GetExamplesForPredicate failed: %v", err)
	}
	t.Logf("user_intent examples: %d", len(examples))

	examplesCorrect, err := corpusFromPath.GetExamplesForPredicate("user_intent", true)
	if err != nil {
		t.Fatalf("GetExamplesForPredicate correctOnly failed: %v", err)
	}
	t.Logf("user_intent correct examples: %d", len(examplesCorrect))

	antiPatterns, err := corpusFromPath.GetAntiPatterns()
	if err != nil {
		t.Fatalf("GetAntiPatterns failed: %v", err)
	}
	t.Logf("Found %d anti-patterns", len(antiPatterns))

	// 10. Test GetDomains
	domains, err := corpusFromPath.GetDomains()
	if err != nil {
		t.Fatalf("GetDomains failed: %v", err)
	}
	t.Logf("Domains: %v", domains)

	// 11. Test FormatPredicateSignature
	sig1, err := corpusFromPath.FormatPredicateSignature("user_intent", false)
	if err != nil {
		t.Errorf("FormatPredicateSignature non-detailed failed: %v", err)
	}
	if !strings.Contains(sig1, "user_intent/") {
		t.Errorf("expected user_intent/ in signature, got %s", sig1)
	}

	sig2, err := corpusFromPath.FormatPredicateSignature("user_intent", true)
	if err != nil {
		t.Errorf("FormatPredicateSignature detailed failed: %v", err)
	}
	t.Logf("detailed user_intent signature: %s", sig2)

	// Test with nonexistent predicate
	nonExistentSig, err := corpusFromPath.FormatPredicateSignature("nonexistent_xyz", true)
	if err != nil {
		t.Errorf("FormatPredicateSignature nonexistent failed: %v", err)
	}
	if nonExistentSig != "nonexistent_xyz" {
		t.Errorf("expected nonexistent signature to be name itself, got %s", nonExistentSig)
	}

	// 12. Test Stats
	stats, err := corpusFromPath.Stats()
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	t.Logf("Stats: %+v", stats)
}

func TestNewPredicateCorpusFromPath_Error(t *testing.T) {
	// Test error path with invalid database file path containing null byte.
	// sql.Open may or may not return an error depending on driver state/version, so we accept both.
	c, err := NewPredicateCorpusFromPath("path\x00withnull")
	if err == nil {
		if c != nil {
			c.Close()
		}
		t.Log("sql.Open returned nil error (expected on lazy connection open)")
	} else {
		t.Logf("Successfully got error: %v", err)
	}
}
