package orient

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"sort"
	"strings"

	"codenerd/internal/types"
)

// Source is one parsed agent-CLI document. Discover records what the file
// says; it does not decide which copy wins or which shard it becomes.
type Source struct {
	ID            string
	Tool          string
	Kind          string
	Name          string
	Path          string
	Tracked       bool
	Digest        string
	Topics        []string
	Tags          []string
	ScopeDir      string
	Scopes        []string
	Description   string
	DeclaredTools []string
	Frontmatter   map[string]any
	Body          string
	Prompt        string
}

// Discover walks the known agent-CLI roots and instruction filenames under
// root. It also reads Claude Code project memory from the user home when that
// directory exists. Git failure leaves every source untracked; the sources
// that could be read are still returned.
func Discover(root string) ([]Source, error) {
	home, _ := os.UserHomeDir()
	return discover(root, home)
}

// Facts converts sources into the extensional rows the ecosystem policy reads.
// Paths, names and digests are strings. Tool, kind and tracked are name atoms.
// Nothing in a body is shortened here.
func Facts(sources []Source) []types.Fact {
	ids := make([]string, len(sources))
	for i, src := range sources {
		ids[i] = src.ID
	}
	sort.Strings(ids)
	ordOf := make(map[string]int64, len(ids))
	for i, id := range ids {
		ordOf[id] = int64(i)
	}

	var facts []types.Fact
	for _, src := range sources {
		tracked := "/no"
		if src.Tracked {
			tracked = "/yes"
		}
		facts = append(facts,
			types.Fact{Predicate: "agent_source", Args: []any{
				src.ID, nameAtom(src.Tool), nameAtom(src.Kind), src.Name, src.Path, nameAtom(tracked),
			}},
			types.Fact{Predicate: "agent_source_digest", Args: []any{src.ID, src.Digest}},
			types.Fact{Predicate: "agent_source_norm", Args: []any{src.ID, normName(src.Name)}},
			types.Fact{Predicate: "agent_source_bytes", Args: []any{src.ID, int64(len(src.Body))}},
			types.Fact{Predicate: "agent_source_ord", Args: []any{src.ID, ordOf[src.ID]}},
		)
		for _, topic := range sortedUnique(src.Topics) {
			facts = append(facts, types.Fact{Predicate: "agent_source_topic", Args: []any{src.ID, topic}})
		}
		for _, tag := range sortedUnique(src.Tags) {
			facts = append(facts, types.Fact{Predicate: "agent_source_tag", Args: []any{src.ID, tag}})
		}
		if src.Description != "" {
			facts = append(facts, types.Fact{Predicate: "agent_source_description", Args: []any{src.ID, src.Description}})
		}
		for _, tool := range sortedUnique(src.DeclaredTools) {
			facts = append(facts, types.Fact{Predicate: "agent_source_declared_tool", Args: []any{src.ID, tool}})
		}
		scopes := src.Scopes
		if len(scopes) == 0 && src.ScopeDir != "" {
			scopes = []string{src.ScopeDir}
		}
		for _, dir := range sortedUnique(scopes) {
			facts = append(facts, types.Fact{Predicate: "agent_source_scope", Args: []any{src.ID, dir}})
		}
	}
	facts = append(facts, referenceFacts(sources)...)
	return facts
}

func referenceFacts(sources []Source) []types.Fact {
	var facts []types.Fact
	for _, from := range sources {
		for _, to := range sources {
			if from.ID == to.ID || from.Body == "" {
				continue
			}
			if !bodyRefers(from.Body, to) {
				continue
			}
			facts = append(facts, types.Fact{Predicate: "agent_source_refers", Args: []any{from.ID, to.ID}})
		}
	}
	return facts
}

func bodyRefers(body string, to Source) bool {
	if len(to.Path) >= 3 && strings.Contains(to.Path, "/") && strings.Contains(body, to.Path) {
		return true
	}
	return len(to.Name) >= 4 && containsToken(body, to.Name)
}

func containsToken(body, token string) bool {
	if token == "" {
		return false
	}
	rest := body
	for {
		i := strings.Index(rest, token)
		if i < 0 {
			return false
		}
		before := i == 0 || !isASCIIAlnum(rest[i-1])
		end := i + len(token)
		after := end >= len(rest) || !isASCIIAlnum(rest[end])
		if before && after {
			return true
		}
		rest = rest[i+1:]
	}
}

func isASCIIAlnum(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

func nameAtom(name string) types.MangleAtom {
	if !strings.HasPrefix(name, "/") {
		name = "/" + name
	}
	return types.MangleAtom(name)
}

func normName(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func digest(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

func normalizeText(s string) string {
	s = strings.TrimPrefix(s, "\uFEFF")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

func sortedUnique(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func addNameTag(src *Source) {
	if strings.TrimSpace(src.Name) == "" {
		return
	}
	if !containsString(src.Tags, src.Name) {
		src.Tags = append(src.Tags, src.Name)
	}
	if !containsString(src.Topics, src.Name) {
		src.Topics = append(src.Topics, src.Name)
	}
}

func addTopic(src *Source, topic string) {
	topic = strings.TrimSpace(topic)
	if topic == "" || containsString(src.Topics, topic) {
		return
	}
	src.Topics = append(src.Topics, topic)
}
