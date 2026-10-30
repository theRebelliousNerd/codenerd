package shell

import (
	"os"
	"path/filepath"
	"strings"
)

// effectiveBuildTags is the custom build-tag set gate's invocation applies,
// or why it cannot be known (unresolved, with no tags).
//
// go vet, and the go list a staticcheck or golangci-lint run loads packages
// through, read GOFLAGS first and the command line second, and each -tags
// replaces the set before it: GOFLAGS=-tags=a with -tags=b on the command
// line builds b alone. The receipt used to union every -tags it saw, so it
// named a tag the run had overridden, and a file only that tag builds read
// as verified (audit run-lint-effective-build-tag-receipt). GOFLAGS is split
// the way the go command splits it: space-separated words, single or double
// quotes grouping a word.
//
// golangci-lint can also set build tags in its own config file (run.
// build-tags), which this does not read: with such a file in the run's
// directory or above it up to the workspace root, the tags are unresolved.
//
// The set says which custom tags were on; it does not say a given file was
// in the packages checked, nor stand for the platform and release tags Go
// always applies.
func effectiveBuildTags(gate, goflags string, argv []string, dir, root string) (tags []string, unresolved string) {
	if gate == "golangci-lint" {
		if cfg := golangciConfig(dir, root); cfg != "" {
			return []string{}, "golangci-lint reads build tags from its config " + cfg + ", which this receipt does not resolve"
		}
	}
	tags = []string{}
	apply := func(words []string) {
		for i, w := range words {
			switch {
			case strings.HasPrefix(w, "-tags=") || strings.HasPrefix(w, "--tags="):
				_, v, _ := strings.Cut(w, "=")
				tags = splitTags(v)
			case (w == "-tags" || w == "--tags") && i+1 < len(words):
				tags = splitTags(words[i+1])
			}
		}
	}
	apply(splitGoFlags(goflags))
	if len(argv) > 0 {
		apply(argv[1:])
	}
	return tags, ""
}

// splitTags is a -tags value's tags: comma-separated (a space-separated list
// is the legacy form go still accepts), empty for an explicit empty value.
func splitTags(v string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, t := range strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' }) {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

// splitGoFlags splits GOFLAGS into words as the go command does: on spaces,
// with a single- or double-quoted run kept as one word (quotes removed).
func splitGoFlags(s string) []string {
	var words []string
	var cur strings.Builder
	inWord := false
	var quote rune
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote, inWord = r, true
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			if inWord {
				words = append(words, cur.String())
				cur.Reset()
				inWord = false
			}
		default:
			cur.WriteRune(r)
			inWord = true
		}
	}
	if inWord {
		words = append(words, cur.String())
	}
	return words
}

// golangciConfig is the golangci-lint config file a run in dir would read,
// searched from dir up to root (dir alone when root is ""), or "".
func golangciConfig(dir, root string) string {
	names := []string{".golangci.yml", ".golangci.yaml", ".golangci.toml", ".golangci.json"}
	for d := filepath.Clean(dir); ; d = filepath.Dir(d) {
		for _, n := range names {
			if _, err := os.Stat(filepath.Join(d, n)); err == nil {
				return filepath.Join(d, n)
			}
		}
		if root == "" || d == filepath.Clean(root) || filepath.Dir(d) == d {
			return ""
		}
	}
}
