package system

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/config"
)

func TestGetOrBootCortex_ConfiguredProviderCredentialChangesIdentity(t *testing.T) {
	workspace := t.TempDir()
	writeCortexIdentityConfig(t, workspace, `{"provider":"openai","model":"gpt-5.4","openai_api_key":"provider-key-one"}`)
	bootCalls := 0
	boot := func(_ context.Context, ws, _ string, _ []string) (*Cortex, error) {
		bootCalls++
		return &Cortex{Workspace: ws}, nil
	}

	first, err := getOrBootCortex(context.Background(), workspace, "", nil, boot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Close() })

	writeCortexIdentityConfig(t, workspace, `{"provider":"openai","model":"gpt-5.4","openai_api_key":"provider-key-two"}`)
	second, err := getOrBootCortex(context.Background(), workspace, "", nil, boot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Close() })
	if second == first || bootCalls != 2 {
		t.Fatalf("configured provider key change reused Cortex: same=%v boot calls=%d", second == first, bootCalls)
	}
}

func TestGetOrBootCortex_Context7EnvironmentCredentialChangesIdentity(t *testing.T) {
	t.Setenv("CONTEXT7_API_KEY", "context7-env-key-one")
	workspace := t.TempDir()
	writeCortexIdentityConfig(t, workspace, `{}`)
	bootCalls := 0
	boot := func(_ context.Context, ws, _ string, _ []string) (*Cortex, error) {
		bootCalls++
		return &Cortex{Workspace: ws}, nil
	}

	first, err := getOrBootCortex(context.Background(), workspace, "", nil, boot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Close() })

	t.Setenv("CONTEXT7_API_KEY", "context7-env-key-two")
	second, err := getOrBootCortex(context.Background(), workspace, "", nil, boot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Close() })
	if second == first || bootCalls != 2 {
		t.Fatalf("effective Context7 key change reused Cortex: same=%v boot calls=%d", second == first, bootCalls)
	}
}

func TestGetOrBootCortex_ExecutionPolicyChangesIdentity(t *testing.T) {
	workspace := t.TempDir()
	writeCortexIdentityConfig(t, workspace, `{"execution":{"allowed_binaries":["go"],"working_directory":"."}}`)
	bootCalls := 0
	boot := func(_ context.Context, ws, _ string, _ []string) (*Cortex, error) {
		bootCalls++
		return &Cortex{Workspace: ws}, nil
	}

	first, err := getOrBootCortex(context.Background(), workspace, "caller-key", nil, boot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Close() })

	writeCortexIdentityConfig(t, workspace, `{"execution":{"allowed_binaries":["go","git"],"working_directory":"."}}`)
	second, err := getOrBootCortex(context.Background(), workspace, "caller-key", nil, boot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Close() })
	if second == first || bootCalls != 2 {
		t.Fatalf("execution policy change reused Cortex: same=%v boot calls=%d", second == first, bootCalls)
	}
}

func TestGetOrBootCortex_BrowserPolicyChangesIdentity(t *testing.T) {
	workspace := t.TempDir()
	writeCortexIdentityConfig(t, workspace, `{"browser":{"policy":{"slow_api_ms":1000}}}`)
	bootCalls := 0
	boot := func(_ context.Context, ws, _ string, _ []string) (*Cortex, error) {
		bootCalls++
		return &Cortex{Workspace: ws}, nil
	}

	first, err := getOrBootCortex(context.Background(), workspace, "caller-key", nil, boot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Close() })

	writeCortexIdentityConfig(t, workspace, `{"browser":{"policy":{"slow_api_ms":1250}}}`)
	second, err := getOrBootCortex(context.Background(), workspace, "caller-key", nil, boot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Close() })
	if second == first || bootCalls != 2 {
		t.Fatalf("browser policy change reused Cortex: same=%v boot calls=%d", second == first, bootCalls)
	}
}

func TestGetOrBootCortex_ReusesCanonicalConfigAndDoesNotSerializeCredentials(t *testing.T) {
	t.Setenv("CONTEXT7_API_KEY", "context7-env-secret")
	workspace := t.TempDir()
	writeCortexIdentityConfig(t, workspace, `{"provider":"openai","model":"gpt-5.4","openai_api_key":"provider-secret","context7_api_key":"context7-file-secret","execution":{"allowed_binaries":["go","git"],"working_directory":"."}}`)
	bootCalls := 0
	boot := func(_ context.Context, ws, _ string, _ []string) (*Cortex, error) {
		bootCalls++
		return &Cortex{Workspace: ws}, nil
	}

	first, err := getOrBootCortex(context.Background(), workspace, "caller-secret", []string{"router", "planner"}, boot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Close() })

	// JSON object ordering and whitespace are representation details. The
	// validated typed config is serialized canonically for its digest.
	writeCortexIdentityConfig(t, workspace, `{
  "execution": { "working_directory": ".", "allowed_binaries": ["go", "git"] },
  "openai_api_key": "provider-secret",
  "context7_api_key": "context7-file-secret",
  "model": "gpt-5.4",
  "provider": "openai"
}`)
	again, err := getOrBootCortex(context.Background(), workspace, "caller-secret", []string{"planner", "router", "router"}, boot)
	if err != nil {
		t.Fatal(err)
	}
	if again != first || bootCalls != 1 {
		t.Fatalf("equivalent config did not reuse Cortex: same=%v boot calls=%d", again == first, bootCalls)
	}

	cfg := &config.UserConfig{Provider: "openai", OpenAIAPIKey: "provider-secret", Context7APIKey: "context7-file-secret"}
	identity, err := newCortexIdentity(workspace, cfg, "caller-secret", []string{"router", "planner"})
	if err != nil {
		t.Fatal(err)
	}
	serialized, err := json.Marshal(identity)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"provider-secret", "caller-secret", "context7-env-secret", "context7-file-secret"} {
		if strings.Contains(string(serialized), secret) {
			t.Fatalf("identity serialized credential %q: %s", secret, serialized)
		}
	}
	if len(identity.ConfigDigest) != 64 || len(identity.APIKeyDigest) != 64 || len(identity.Context7KeyDigest) != 64 {
		t.Fatalf("identity digests are missing or malformed: %+v", identity)
	}
}

func TestGetOrBootCortex_InvalidConfigIsNeverCached(t *testing.T) {
	workspace := t.TempDir()
	writeCortexIdentityConfig(t, workspace, `{"unknown_policy":true}`)
	bootCalls := 0
	boot := func(_ context.Context, ws, _ string, _ []string) (*Cortex, error) {
		bootCalls++
		return &Cortex{Workspace: ws}, nil
	}

	first, err := getOrBootCortex(context.Background(), workspace, "", nil, boot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Close() })
	second, err := getOrBootCortex(context.Background(), workspace, "", nil, boot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Close() })
	if first == second || bootCalls != 2 {
		t.Fatalf("invalid config was reused: same=%v boot calls=%d", first == second, bootCalls)
	}
}

func writeCortexIdentityConfig(t *testing.T, workspace, contents string) {
	t.Helper()
	configDir := filepath.Join(workspace, ".nerd")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
