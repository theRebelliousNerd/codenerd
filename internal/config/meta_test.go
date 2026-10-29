package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMetaProviderConfig_CheckRejectsUnknownSize(t *testing.T) {
	bad := &UserConfig{Meta: &MetaProviderConfig{SearchContextSize: "huge"}}
	if problemAt(bad.Check(nil), SeverityError, "meta.search_context_size") == nil {
		t.Fatal("search_context_size \"huge\" was accepted")
	}
	for _, size := range []string{"", "low", "MEDIUM", " high "} {
		cfg := &UserConfig{Meta: &MetaProviderConfig{SearchContextSize: size}}
		if p := problemAt(cfg.Check(nil), SeverityError, "meta.search_context_size"); p != nil {
			t.Fatalf("size %q: %v", size, p)
		}
	}
	if p := problemAt((&UserConfig{}).Check(nil), SeverityError, "meta.search_context_size"); p != nil {
		t.Fatalf("an omitted meta block was an error: %v", p)
	}
	off := false
	explicit := &UserConfig{Meta: &MetaProviderConfig{EnableWebSearch: &off, SearchContextSize: "high"}}
	if p := problemAt(explicit.Check(nil), SeverityError, "meta.enable_web_search"); p != nil {
		t.Fatalf("explicit false was an error: %v", p)
	}
}

func TestMetaProviderConfig_ExplicitFalseRoundTrips(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	body := []byte(`{"provider":"meta","meta_api_key":"k","model":"muse-spark-1.3-contributor","meta":{"enable_web_search":false,"search_context_size":"low"}}`)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadUserConfig(path)
	if err != nil {
		t.Fatalf("LoadUserConfig: %v", err)
	}
	got := loaded.GetMetaConfig()
	if got.WebSearchEnabled() {
		t.Fatal("explicit enable_web_search false became true")
	}
	if got.ResolvedSearchContextSize() != "low" {
		t.Fatalf("search_context_size = %q, want low", got.ResolvedSearchContextSize())
	}
	*got.EnableWebSearch = true
	if loaded.GetMetaConfig().WebSearchEnabled() {
		t.Fatal("GetMetaConfig aliased the stored enable_web_search pointer")
	}

	omitted := (&UserConfig{}).GetMetaConfig()
	if !omitted.WebSearchEnabled() || omitted.ResolvedSearchContextSize() != "medium" {
		t.Fatalf("omitted block resolved to enabled=%v size=%q, want true/medium", omitted.WebSearchEnabled(), omitted.ResolvedSearchContextSize())
	}

	// The default document the full dump writes must still say search is on.
	def := DefaultMetaProviderConfig()
	raw, err := json.Marshal(def)
	if err != nil {
		t.Fatal(err)
	}
	var again MetaProviderConfig
	if err := json.Unmarshal(raw, &again); err != nil {
		t.Fatal(err)
	}
	if !again.WebSearchEnabled() || again.ResolvedSearchContextSize() != "medium" {
		t.Fatalf("default marshal round trip = enabled %v size %q", again.WebSearchEnabled(), again.ResolvedSearchContextSize())
	}
}
