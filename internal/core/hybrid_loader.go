package core

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"codenerd/internal/logging"
)

// HybridIntent represents a canonical intent pattern from a hybrid .mg file.
// Example: INTENT: "my car won't start" -> /breakdown_support "target"
type HybridIntent struct {
	Phrase     string
	Verb       string
	Target     string
	Constraint string
}

// HybridPrompt represents an atomic prompt from a hybrid .mg file.
// Example: PROMPT: /role_coder [role] -> "You are a Senior Go Engineer..."
type HybridPrompt struct {
	ID         string
	Category   string
	Content    string
	Tags       []string
	SourceFile string
}

// HybridLoadResult contains logic code and extracted EDB facts from a hybrid .mg file.
// Hybrid files may include DATA directives (TAXONOMY/INTENT/PROMPT) that are not valid Mangle.
type HybridLoadResult struct {
	Logic   string
	Facts   []Fact
	Intents []HybridIntent
	Prompts []HybridPrompt
}

// LoadHybridMangleFile parses a hybrid Mangle file.
// - TAXONOMY lines are converted to subclass_of/2 EDB facts.
// - INTENT lines are converted to intent_definition/3 EDB facts for semantic routing.
// - PROMPT lines are extracted into HybridPrompt entries for JIT atom routing.
// - All other lines are treated as real Mangle logic and returned as a string.
func LoadHybridMangleFile(path string) (HybridLoadResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return HybridLoadResult{}, err
	}
	defer file.Close()

	var logic strings.Builder
	var facts []Fact
	var intents []HybridIntent
	var prompts []HybridPrompt

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			logic.WriteString("\n")
			continue
		}

		switch {
		case strings.HasPrefix(line, "TAXONOMY:"):
			chain := strings.TrimSpace(strings.TrimPrefix(line, "TAXONOMY:"))
			parts := strings.Split(chain, ">")
			nodes := make([]string, 0, len(parts))
			for _, p := range parts {
				n := strings.TrimSpace(p)
				if n != "" {
					nodes = append(nodes, n)
				}
			}
			for i := 0; i < len(nodes)-1; i++ {
				parent := nodes[i]
				child := nodes[i+1]
				facts = append(facts, Fact{
					Predicate: "subclass_of",
					Args:      []interface{}{child, parent},
				})
			}
			continue

		case strings.HasPrefix(line, "INTENT:"):
			intent, ok := parseIntentDirective(line)
			if !ok {
				logging.Get(logging.CategoryKernel).Warn("LoadHybridMangleFile: failed to parse INTENT line in %s: %q", path, line)
				continue
			}
			intents = append(intents, intent)
			// Store canonical intents as EDB facts for later semantic embedding.
			facts = append(facts, Fact{
				Predicate: "intent_definition",
				Args:      []interface{}{intent.Phrase, intent.Verb, intent.Target},
			})
			continue

		case strings.HasPrefix(line, "PROMPT:"):
			prompt, ok := parsePromptDirective(line)
			if !ok {
				logging.Get(logging.CategoryKernel).Warn("LoadHybridMangleFile: failed to parse PROMPT line in %s: %q", path, line)
				continue
			}
			prompt.SourceFile = path
			prompts = append(prompts, prompt)
			continue
		}

		logic.WriteString(scanner.Text())
		logic.WriteString("\n")
	}
	if err := scanner.Err(); err != nil {
		return HybridLoadResult{}, fmt.Errorf("scan failed: %w", err)
	}

	return HybridLoadResult{Logic: logic.String(), Facts: facts, Intents: intents, Prompts: prompts}, nil
}

// parseIntentDirective parses an INTENT: directive into a HybridIntent.
func parseIntentDirective(line string) (HybridIntent, bool) {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "INTENT:"))
	rest = stripInlineComment(rest)
	parts := strings.SplitN(rest, "->", 2)
	if len(parts) != 2 {
		return HybridIntent{}, false
	}

	phrasePart := strings.TrimSpace(parts[0])
	phrase := trimQuotes(phrasePart)
	if phrase == "" {
		return HybridIntent{}, false
	}

	mappingPart := strings.TrimSpace(parts[1])
	mappingPart = stripInlineComment(mappingPart)
	fields := strings.Fields(mappingPart)
	if len(fields) == 0 {
		return HybridIntent{}, false
	}

	intent := HybridIntent{
		Phrase: phrase,
		Verb:   fields[0],
	}
	if len(fields) > 1 {
		intent.Target = trimQuotes(fields[1])
	}
	if len(fields) > 2 {
		intent.Constraint = trimQuotes(strings.Join(fields[2:], " "))
	}
	return intent, true
}

// parsePromptDirective parses a PROMPT: directive into a HybridPrompt.
func parsePromptDirective(line string) (HybridPrompt, bool) {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "PROMPT:"))
	rest = stripInlineComment(rest)
	parts := strings.SplitN(rest, "->", 2)
	if len(parts) != 2 {
		return HybridPrompt{}, false
	}

	left := strings.TrimSpace(parts[0])
	right := strings.TrimSpace(parts[1])
	content := trimQuotes(right)
	if content == "" {
		return HybridPrompt{}, false
	}

	// Extract bracket tags from left side.
	var tags []string
	leftNoTags := left
	for {
		start := strings.Index(leftNoTags, "[")
		end := strings.Index(leftNoTags, "]")
		if start == -1 || end == -1 || end < start {
			break
		}
		tag := strings.TrimSpace(leftNoTags[start+1 : end])
		if tag != "" {
			tags = append(tags, tag)
		}
		leftNoTags = strings.TrimSpace(leftNoTags[:start] + " " + leftNoTags[end+1:])
	}

	fields := strings.Fields(leftNoTags)
	if len(fields) == 0 {
		return HybridPrompt{}, false
	}

	id := trimQuotes(fields[0])
	id = strings.TrimPrefix(id, "/")
	if id == "" {
		return HybridPrompt{}, false
	}

	category := ""
	if len(tags) > 0 {
		category = tags[0]
	}

	return HybridPrompt{
		ID:       id,
		Category: category,
		Content:  content,
		Tags:     tags,
	}, true
}

func stripInlineComment(s string) string {
	// Strip # and // comments (best-effort).
	if idx := strings.Index(s, "#"); idx != -1 {
		s = s[:idx]
	}
	if idx := strings.Index(s, "//"); idx != -1 {
		s = s[:idx]
	}
	return strings.TrimSpace(s)
}

func trimQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		first := s[0]
		last := s[len(s)-1]
		if (first == '"' && last == '"') || (first == '`' && last == '`') {
			return s[1 : len(s)-1]
		}
	}
	return strings.Trim(s, "\"`")
}
