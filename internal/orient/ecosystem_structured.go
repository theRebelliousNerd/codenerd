package orient

import (
	"encoding/json"
	"fmt"
	"path"

	"gopkg.in/yaml.v3"
)

// parseRoomodes turns each customModes entry into its own source. The body of
// every mode is the whole file: splitting the file per mode would drop keys
// that are not that mode's fields, and the digest then matches across modes
// of one file. Same tool and same digest are not cross-tool duplicates.
// JSON is tried first, then YAML. A file that is neither is still one source.
func parseRoomodes(rel, text string, tracked bool) []Source {
	doc, ok := decodeObject(text)
	if !ok {
		return []Source{roomodesFallback(rel, text, tracked)}
	}
	raw, _ := doc["customModes"].([]any)
	if len(raw) == 0 {
		return []Source{roomodesFallback(rel, text, tracked)}
	}
	var out []Source
	for i, item := range raw {
		mode, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, oneMode(rel, text, tracked, doc, mode, i))
	}
	if len(out) == 0 {
		return []Source{roomodesFallback(rel, text, tracked)}
	}
	return out
}

func decodeObject(text string) (map[string]any, bool) {
	var doc map[string]any
	if err := json.Unmarshal([]byte(text), &doc); err == nil && doc != nil {
		return doc, true
	}
	doc = nil
	if err := yaml.Unmarshal([]byte(text), &doc); err == nil && doc != nil {
		return doc, true
	}
	return nil, false
}

func oneMode(rel, text string, tracked bool, doc, mode map[string]any, index int) Source {
	fm := map[string]any{}
	for k, v := range mode {
		fm[k] = v
	}
	for k, v := range doc {
		if k == "customModes" {
			continue
		}
		if _, exists := fm[k]; exists {
			fm["file:"+k] = v
			continue
		}
		fm[k] = v
	}
	slug := stringField(fm, "slug")
	name := stringField(fm, "name")
	if name == "" {
		name = slug
	}
	if slug == "" {
		slug = name
	}
	if slug == "" {
		slug = fmt.Sprintf("mode%d", index+1)
	}
	if name == "" {
		name = slug
	}
	src := Source{
		ID:            rel + "#" + slug,
		Tool:          "roo",
		Kind:          "mode",
		Name:          name,
		Path:          rel,
		Tracked:       tracked,
		Body:          text,
		Digest:        digest(text),
		Frontmatter:   fm,
		Description:   stringField(fm, "description"),
		Tags:          tagsOf(fm),
		DeclaredTools: toolsOf(fm),
	}
	src.Prompt = promptOf("mode", fm, text)
	return src
}

func roomodesFallback(rel, text string, tracked bool) Source {
	return Source{
		ID:      rel,
		Tool:    "roo",
		Kind:    "mode",
		Name:    "roomodes",
		Path:    rel,
		Tracked: tracked,
		Body:    text,
		Digest:  digest(text),
		Frontmatter: map[string]any{
			"unparsed_frontmatter": text,
		},
	}
}

func parseSettings(rel, text string, tracked bool) Source {
	fm := map[string]any{}
	var doc map[string]any
	if err := json.Unmarshal([]byte(text), &doc); err != nil || doc == nil {
		fm["unparsed_frontmatter"] = text
	} else {
		fm = doc
	}
	src := Source{
		ID:          rel,
		Tool:        "gemini",
		Kind:        "instructions",
		Name:        "settings",
		Path:        rel,
		Tracked:     tracked,
		Body:        text,
		Digest:      digest(text),
		Frontmatter: fm,
		Description: stringField(fm, "description"),
	}
	if path.Dir(rel) != "." {
		src.ScopeDir = path.Dir(rel)
	}
	return src
}
