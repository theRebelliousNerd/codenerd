package init

import "testing"

// Every hint --prefer accepts sets its keys; an unknown one is refused.
func TestPreferenceHints_ApplyTheirKeysAndRefuseUnknown(t *testing.T) {
	ini := &Initializer{config: InitConfig{PreferenceHints: []string{"strict", "expert"}}}
	prefs := ini.initPreferences()
	if !prefs.RequireTests || !prefs.RequireReview || prefs.ExplanationLevel != "expert" {
		t.Fatalf("hints not applied: %+v", prefs)
	}
	keys := ini.hintOverriddenPreferenceKeys()
	for _, k := range []string{"require_tests", "require_review", "explanation_level"} {
		if !keys[k] {
			t.Errorf("key %s not marked as set by a hint", k)
		}
	}
	if err := ValidatePreferenceHints([]string{"strict", "nope"}); err == nil {
		t.Fatal("an unknown hint was accepted")
	}
	if err := ValidatePreferenceHints(PreferenceHintNames()); err != nil {
		t.Fatalf("a known hint was refused: %v", err)
	}
}
