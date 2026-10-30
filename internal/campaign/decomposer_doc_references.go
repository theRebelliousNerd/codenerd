package campaign

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"codenerd/internal/core"
	"codenerd/internal/logging"
)

// maxDocReferenceBytes bounds how much of one document is read for links: a
// specification that large is not a document someone links from.
const maxDocReferenceBytes = 2 << 20

var markdownLink = regexp.MustCompile(`\]\(([^)\s#]+)(?:#[^)]*)?\)`)

// docReferenceFacts finds the links between the documents of one campaign's
// source set: doc_reference(From, To) for every relative markdown link in a
// document that resolves to another document in the set. The selection policy
// (selection_policy.mg) carries a document's relevance across these edges, so a
// page the goal's topics reach pulls in the pages it sends the reader to.
//
// Only links inside the set count: a link to a file the campaign was not given
// is not something it can read. A document that cannot be read has no edges.
func docReferenceFacts(files []FileMetadata) []core.Fact {
	inSet := make(map[string]string, len(files))
	for _, f := range files {
		inSet[filepath.Clean(f.Path)] = f.Path
	}
	var facts []core.Fact
	seen := map[[2]string]bool{}
	for _, from := range files {
		switch strings.ToLower(filepath.Ext(from.Path)) {
		case ".md", ".markdown", ".mdx":
		default:
			continue
		}
		if from.SizeBytes > maxDocReferenceBytes {
			continue
		}
		data, err := os.ReadFile(from.Path)
		if err != nil {
			logging.CampaignDebug("doc references: %s unreadable: %v", from.Path, err)
			continue
		}
		dir := filepath.Dir(from.Path)
		for _, m := range markdownLink.FindAllStringSubmatch(string(data), -1) {
			target := m[1]
			if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			to, ok := inSet[filepath.Clean(filepath.Join(dir, filepath.FromSlash(target)))]
			if !ok || to == from.Path {
				continue
			}
			edge := [2]string{from.Path, to}
			if seen[edge] {
				continue
			}
			seen[edge] = true
			facts = append(facts, core.Fact{Predicate: "doc_reference", Args: []any{from.Path, to}})
		}
	}
	return facts
}
