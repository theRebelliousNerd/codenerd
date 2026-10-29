package init

import "testing"

func depNames(deps []DependencyInfo) map[string]bool {
	m := make(map[string]bool, len(deps))
	for _, d := range deps {
		m[d.Name] = true
	}
	return m
}

func TestParseCargoLock(t *testing.T) {
	i := &Initializer{}
	content := `
[[package]]
name = "tokio"
version = "1.0"

[[package]]
name = "rdkafka"
version = "0.30"

[[package]]
name = "some_unknown_crate"
version = "0.1"
`
	got := depNames(i.parseCargoLock(content))
	if !got["tokio"] {
		t.Error("expected tokio to be detected")
	}
	if !got["kafka"] { // rdkafka maps to "kafka"
		t.Error("expected rdkafka to be mapped to 'kafka'")
	}
	if got["some_unknown_crate"] {
		t.Error("non-notable crate should be ignored")
	}
}

func TestParseYarnLock(t *testing.T) {
	i := &Initializer{}
	content := `
"webpack@^5.0.0":
  version "5.1.0"
"jest@^29.0.0":
  version "29.1.0"
"left-pad@^1.0.0":
  version "1.3.0"
`
	got := depNames(i.parseYarnLock(content))
	if !got["webpack"] || !got["jest"] {
		t.Errorf("expected webpack and jest detected, got %v", got)
	}
	if got["left-pad"] {
		t.Error("non-notable package should be ignored")
	}
}

// pnpm reports the vocabulary's technology keys, as every lock scanner does:
// tailwindcss is "tailwind" here exactly as package-lock.json reports it.
func TestParsePnpmLock(t *testing.T) {
	i := &Initializer{}
	content := "packages:\n  /tailwindcss@3.4.0:\n  /vite@5.0.0:\n  /left-pad@1.3.0:\n"
	got := depNames(i.parsePnpmLock(content))
	if !got["tailwind"] || !got["vite"] {
		t.Errorf("expected tailwind and vite detected, got %v", got)
	}
	if got["left-pad"] {
		t.Error("a package outside the vocabulary should be ignored")
	}
}
