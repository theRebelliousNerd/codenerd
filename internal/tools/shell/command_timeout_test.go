package shell

import "testing"

func TestDefaultCommandTimeout(t *testing.T) {
	cases := []struct {
		command string
		want    int
	}{
		{"go test ./internal/retrieval/ -run 'Tiered|Semantic' -count=1", defaultToolchainTimeoutSeconds},
		{"go build ./...", defaultToolchainTimeoutSeconds},
		{"go vet ./cmd/nerd/", defaultToolchainTimeoutSeconds},
		{`C:\Go\bin\go.exe test ./...`, defaultToolchainTimeoutSeconds},
		{"/usr/local/go/bin/go build ./cmd/nerd", defaultToolchainTimeoutSeconds},
		{"cargo test", defaultToolchainTimeoutSeconds},
		{"npm test", defaultToolchainTimeoutSeconds},
		{"make", defaultToolchainTimeoutSeconds},
		{"pytest tests/", defaultToolchainTimeoutSeconds},
		{"go version", defaultShortTimeoutSeconds},
		{"go env GOPATH", defaultShortTimeoutSeconds},
		{"git status", defaultShortTimeoutSeconds},
		{"ls -la", defaultShortTimeoutSeconds},
		{"", defaultShortTimeoutSeconds},
		{"go", defaultShortTimeoutSeconds},
	}
	for _, tc := range cases {
		if got := defaultCommandTimeout(tc.command); got != tc.want {
			t.Errorf("defaultCommandTimeout(%q) = %d, want %d", tc.command, got, tc.want)
		}
	}
}
