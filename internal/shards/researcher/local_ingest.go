package researcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// IngestLocalDocs walks a directory of local documents, chunking and inserting
// them into the shard's LocalStore with embeddings, knowledge atoms, and graph links.
// Returns the number of chunks ingested.
func (r *ResearcherShard) IngestLocalDocs(ctx context.Context, root string) (int, error) {
	r.mu.RLock()
	db := r.localDB
	r.mu.RUnlock()

	if db == nil {
		return 0, fmt.Errorf("localDB not configured on ResearcherShard")
	}

	info, err := os.Stat(root)
	if err != nil {
		return 0, err
	}
	if !info.IsDir() {
		return 0, fmt.Errorf("path is not a directory: %s", root)
	}

	total := 0

	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !isSupportedDocExt(path) {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		text := string(data)
		chunks := chunkText(text, 2000)
		if len(chunks) == 0 {
			return nil
		}

		// Graph: root -> file
		_ = db.StoreLink(root, "/has_file", path, 1.0, map[string]interface{}{"path": path})

		for idx, chunk := range chunks {
			meta := map[string]interface{}{
				"path":        path,
				"chunk_index": idx,
				"total":       len(chunks),
				"source":      "local_doc_ingest",
			}
			_ = db.StoreVectorWithEmbedding(ctx, chunk, meta)
			_ = db.StoreKnowledgeAtom(path, chunk, 0.9)

			chunkID := fmt.Sprintf("%s#%d", path, idx)
			_ = db.StoreLink(path, "/has_chunk", chunkID, 0.5, meta)
			total++
		}
		return nil
	})

	return total, nil
}

// chunkText splits into rune-safe chunks.
func chunkText(text string, maxLen int) []string {
	if maxLen <= 0 {
		maxLen = 2000
	}
	rs := []rune(text)
	if len(rs) == 0 {
		return nil
	}
	out := make([]string, 0, (len(rs)/maxLen)+1)
	for i := 0; i < len(rs); i += maxLen {
		end := i + maxLen
		if end > len(rs) {
			end = len(rs)
		}
		out = append(out, string(rs[i:end]))
	}
	return out
}

func isSupportedDocExt(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".md", ".mdx", ".txt", ".rst", ".adoc", ".asciidoc",
		".yaml", ".yml", ".json", ".toml", ".ini", ".cfg",
		".go", ".ts", ".tsx", ".js", ".jsx", ".cs", ".java", ".py":
		return true
	default:
		return false
	}
}
