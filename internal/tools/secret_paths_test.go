package tools

import "testing"

func TestIsSecretPath_TheDefaultsCatchAWorkspacesOwnKeys(t *testing.T) {
	secretPatterns.Store(nil) // no boot: the documented defaults are in force
	cases := []struct {
		path string
		want bool
	}{
		{".env", true},
		{"C:\\CodeProjects\\codeNERD\\.env", true},
		{"./.env", true},
		{".ENV", true}, // Windows: the same file
		{".env.production", true},
		{"deploy/prod.env", true},
		{".nerd/config.json", true},
		{"C:/CodeProjects/codeNERD/.nerd/config.json", true},
		{".nerd\\config.json", true},
		{"certs/server.pem", true},
		{"id_ed25519", true},
		{"home/.aws/credentials", true},
		{"gcloud/app_credentials.json", true},

		{"internal/config/config.go", false},
		{"config.json", false}, // only .nerd/config.json holds the keys
		{"internal/auth/credentials.go", false},
		{"Docs/architecture/features/TODO.md", false},
		{"environment.go", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsSecretPath(c.path); got != c.want {
			t.Errorf("IsSecretPath(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestIsSecretPath_TheConfiguredListReplacesTheDefaults(t *testing.T) {
	t.Cleanup(func() { secretPatterns.Store(nil) })

	SetSecretPathPatterns([]string{"*.vault"})
	if !IsSecretPath("ops/prod.vault") {
		t.Error("a configured pattern was not honoured")
	}
	if IsSecretPath(".env") {
		t.Error("a configured list must replace the defaults, not add to them")
	}

	// An explicit empty list is the user's decision that nothing is secret.
	SetSecretPathPatterns([]string{})
	if IsSecretPath(".env") {
		t.Error("an explicit empty list must protect nothing")
	}
}

func TestSecretPathInCommand_FindsADirectReference(t *testing.T) {
	secretPatterns.Store(nil)
	for _, cmd := range []string{
		"grep KEY .env",
		`python -c "print(open('.env').read())"`,
		"cp .nerd/config.json out.txt",
		"type C:\\CodeProjects\\codeNERD\\.env",
		"cat<.env",
	} {
		if _, ok := SecretPathInCommand(cmd); !ok {
			t.Errorf("SecretPathInCommand(%q) found nothing", cmd)
		}
	}
	for _, cmd := range []string{"go test ./...", "git status --porcelain", "grep -rn env internal/"} {
		if tok, ok := SecretPathInCommand(cmd); ok {
			t.Errorf("SecretPathInCommand(%q) flagged %q", cmd, tok)
		}
	}
}
