package config

import "testing"

func TestMissionConfigBoundsArePublishedAndValidated(t *testing.T) {
	cfg := (*UserConfig)(nil).GetMissionConfig()
	if problems := cfg.Check("mission"); len(problems) != 0 {
		t.Fatal(problems)
	}
	if len(cfg.Params()) != 6 {
		t.Fatalf("missing mission budget params: %v", cfg.Params())
	}
	cfg.Concurrency = 2
	if len(cfg.Check("mission")) == 0 {
		t.Fatal("shared-checkout parallel mutations admitted")
	}
	cfg.Concurrency = 1
	cfg.MaxSpendMicroUSD = -1
	if len(cfg.Check("mission")) == 0 {
		t.Fatal("negative spend admitted")
	}
}
