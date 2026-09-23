package codedom_test

import (
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/tools"
)

// A secret file's contents never reach the model through the structural
// tools, exactly as through grep: the index never parses one, find_text never
// searches one, and the element tools refuse one by path.
func TestStructuralTools_NeverReturnASecretFilesContents(t *testing.T) {
	tools.SetSecretPathPatterns(append(config.DefaultSecretPaths(), "vault.go"))
	t.Cleanup(func() { tools.SetSecretPathPatterns(config.DefaultSecretPaths()) })

	reg, _ := structWorkspace(t, map[string]string{
		"go.mod":                "module fixture\n\ngo 1.24\n",
		".env":                  "XAI_API_KEY=sk-not-a-real-key\n",
		"internal/app/app.go":   "package app\n\n// XAI_API_KEY is read from the environment.\nfunc Key() string { return \"XAI_API_KEY\" }\n",
		"internal/app/vault.go": "package app\n\n// Vault holds the key.\nconst vaultKey = \"sk-vault-not-real\"\n",
	})

	out := run(t, reg, "find_text", map[string]any{"text": "sk-"})
	if strings.Contains(out, "sk-not-a-real-key") || strings.Contains(out, "sk-vault-not-real") {
		t.Fatalf("find_text returned a secret file's contents:\n%s", out)
	}
	ordinary := run(t, reg, "find_text", map[string]any{"text": "XAI_API_KEY"})
	if !strings.Contains(ordinary, "internal/app.Key") {
		t.Fatalf("ordinary files are still searched:\n%s", ordinary)
	}
	if _, err := reg.Execute(t.Context(), "find_text", map[string]any{"text": "sk-", "path": ".env"}); err == nil || !strings.Contains(err.Error(), "secret file") {
		t.Fatalf("find_text aimed at a secret file must refuse: %v", err)
	}
	symbols := run(t, reg, "find_symbol", map[string]any{"name": "vaultKey"})
	if !tools.StructuralMissed(symbols) {
		t.Fatalf("a secret Go file is never indexed:\n%s", symbols)
	}
	if _, err := reg.Execute(t.Context(), "get_elements", map[string]any{"path": "internal/app/vault.go"}); err == nil || !strings.Contains(err.Error(), "secret file") {
		t.Fatalf("get_elements on a secret file must refuse: %v", err)
	}
	if _, err := reg.Execute(t.Context(), "get_element", map[string]any{"ref": "internal/app/vault.go:vaultKey"}); err == nil || !strings.Contains(err.Error(), "secret file") {
		t.Fatalf("get_element on a secret file must refuse: %v", err)
	}
}
