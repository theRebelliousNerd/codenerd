package main

import (
	"strings"
	"testing"

	nerdinit "codenerd/internal/init"
)

// Init applied InitConfig.PreferenceHints, but no front end supplied them:
// read and never written. nerd init --prefer supplies them.
func TestPreferFlag_ReachesInit(t *testing.T) {
	if initCmd.Flags().Lookup("prefer") == nil {
		t.Fatal("nerd init has no --prefer flag")
	}
	saved := initPreferFlags
	t.Cleanup(func() { initPreferFlags = saved })

	initPreferFlags = []string{"strict", "table_driven_tests"}
	cfg := nerdinit.DefaultInitConfig(t.TempDir())
	if err := applyInitPreferenceFlags(&cfg); err != nil {
		t.Fatal(err)
	}
	if strings.Join(cfg.PreferenceHints, ",") != "strict,table_driven_tests" {
		t.Fatalf("PreferenceHints = %v", cfg.PreferenceHints)
	}
	initPreferFlags = []string{"stirct"}
	if err := applyInitPreferenceFlags(&cfg); err == nil || !strings.Contains(err.Error(), "stirct") {
		t.Fatalf("an unknown hint was accepted: %v", err)
	}
}
