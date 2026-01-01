package chat

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/config"
	"codenerd/internal/embedding"
	"codenerd/internal/logging"
	"codenerd/internal/prompt"
	"codenerd/internal/store"
	nerdsystem "codenerd/internal/system"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) ingestAgentDocs(agentName, docPath string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), config.GetLLMTimeouts().DocumentProcessingTimeout)
		defer cancel()

		registry := m.loadAgentRegistry()
		if registry == nil || len(registry.Agents) == 0 {
			return responseMsg("No agent registry found. Run `/init` or `/agents` first.")
		}

		var kbPath string
		for _, a := range registry.Agents {
			if strings.EqualFold(a.Name, agentName) {
				kbPath = a.KnowledgePath
				break
			}
		}
		if kbPath == "" {
			return responseMsg(fmt.Sprintf("Unknown agent %q. Run `/agents` to see available agents.", agentName))
		}

		root := strings.TrimSpace(docPath)
		if root == "" {
			return responseMsg("Usage: `/ingest <agent> <path>`")
		}
		if !filepath.IsAbs(root) {
			root = filepath.Join(m.workspace, root)
		}
		info, err := os.Stat(root)
		if err != nil {
			return responseMsg(fmt.Sprintf("Ingest failed: %v", err))
		}

		// Ensure KB directory exists (some agents may have missing DB directories).
		if err := os.MkdirAll(filepath.Dir(kbPath), 0755); err != nil {
			return responseMsg(fmt.Sprintf("Ingest failed: mkdir KB dir: %v", err))
		}

		// Create embedding engine (optional) so prompt_atoms get embeddings for vector search.
		var embeddingEngine embedding.EmbeddingEngine
		if m.Config != nil {
			embCfg := m.Config.GetEmbeddingConfig()
			if embCfg.Provider != "" {
				engine, err := embedding.NewEngine(embedding.Config{
					Provider:       embCfg.Provider,
					OllamaEndpoint: embCfg.OllamaEndpoint,
					OllamaModel:    embCfg.OllamaModel,
					GenAIAPIKey:    embCfg.GenAIAPIKey,
					GenAIModel:     embCfg.GenAIModel,
					TaskType:       embCfg.TaskType,
				})
				if err != nil {
					logging.Boot("Warning: embedding engine init failed (ingest will proceed without embeddings): %v", err)
				} else {
					embeddingEngine = engine
				}
			}
		}

		// Ensure prompt_atoms schema and store ingested content as prompt atoms for JIT selection.
		db, err := sql.Open("sqlite3", kbPath)
		if err != nil {
			return responseMsg(fmt.Sprintf("Ingest failed: open agent DB: %v", err))
		}
		defer db.Close()

		atomLoader := prompt.NewAtomLoader(embeddingEngine)
		if err := atomLoader.EnsureSchema(ctx, db); err != nil {
			return responseMsg(fmt.Sprintf("Ingest failed: ensure prompt atom schema: %v", err))
		}

		// Also ingest into LocalStore tables (knowledge_atoms + vectors) when available.
		var localDB *store.LocalStore
		if ls, err := store.NewLocalStore(kbPath); err == nil {
			localDB = ls
			defer localDB.Close()
			if embeddingEngine != nil {
				localDB.SetEmbeddingEngine(embeddingEngine)
			}
		}

		files, err := collectIngestFiles(root, info.IsDir())
		if err != nil {
			return responseMsg(fmt.Sprintf("Ingest failed: %v", err))
		}
		if len(files) == 0 {
			return responseMsg("No ingestable files found (supported: .md, .txt, .yaml, .json, etc).")
		}

		var (
			promptAtomsStored int
			knowledgeChunks   int
			skippedFiles      []string
		)

		agentKey := strings.ToLower(agentName)
		for _, path := range files {
			data, err := os.ReadFile(path)
			if err != nil {
				skippedFiles = append(skippedFiles, path)
				continue
			}

			rel := path
			if r, err := filepath.Rel(m.workspace, path); err == nil && r != "" && !strings.HasPrefix(r, "..") {
				rel = filepath.ToSlash(r)
			} else {
				rel = filepath.ToSlash(path)
			}

			chunks := chunkTextRunes(string(data), 2000)
			if len(chunks) == 0 {
				skippedFiles = append(skippedFiles, path)
				continue
			}

			sourceHash := prompt.HashContent(rel)[:8]
			if localDB != nil {
				_ = localDB.StoreLink(root, "/has_file", rel, 1.0, map[string]interface{}{"path": rel})
			}

			for idx, chunk := range chunks {
				chunkHash := prompt.HashContent(chunk)[:12]
				atomID := fmt.Sprintf("ingest/%s/%s/%03d/%s", agentKey, sourceHash, idx, chunkHash)

				content := fmt.Sprintf("SOURCE: %s (chunk %d/%d)\n\n%s", rel, idx+1, len(chunks), chunk)
				atom := prompt.NewPromptAtom(atomID, prompt.CategoryDomain, content)
				atom.Priority = 60
				atom.IsMandatory = false

				if err := atomLoader.StoreAtom(ctx, db, atom); err == nil {
					promptAtomsStored++
				}

				if localDB != nil {
					meta := map[string]interface{}{
						"path":         rel,
						"chunk_index":  idx,
						"total":        len(chunks),
						"source":       "agent_ingest",
						"agent":        agentName,
						"content_type": contentTypeForIngestPath(rel),
					}
					_ = localDB.StoreVectorWithEmbedding(ctx, chunk, meta)
					_ = localDB.StoreKnowledgeAtom(rel, chunk, 0.9)
					_ = localDB.StoreLink(rel, "/has_chunk", fmt.Sprintf("%s#%d", rel, idx), 0.5, meta)
					knowledgeChunks++
				}
			}
		}

		// Update .nerd/agents.json KB size/status for UI immediately.
		if err := nerdsystem.SyncAgentRegistryFromDisk(m.workspace); err != nil {
			logging.Get(logging.CategoryContext).Warn("Failed to sync agent registry after ingest: %v", err)
		}

		var sb strings.Builder
		sb.WriteString("## Ingest Complete\n\n")
		sb.WriteString(fmt.Sprintf("- Agent: %s\n", agentName))
		sb.WriteString(fmt.Sprintf("- KB: %s\n", kbPath))
		sb.WriteString(fmt.Sprintf("- Source: %s\n", root))
		sb.WriteString(fmt.Sprintf("- Files: %d\n", len(files)))
		sb.WriteString(fmt.Sprintf("- Prompt atoms stored: %d\n", promptAtomsStored))
		if localDB != nil {
			sb.WriteString(fmt.Sprintf("- Knowledge chunks stored: %d\n", knowledgeChunks))
		} else {
			sb.WriteString("- Knowledge chunks stored: 0 (LocalStore unavailable)\n")
		}
		if len(skippedFiles) > 0 {
			sb.WriteString(fmt.Sprintf("\nSkipped %d file(s) due to read/parse errors.\n", len(skippedFiles)))
		}
		sb.WriteString("\nNext: `/spawn " + agentName + " <task>` (the agent can now retrieve this content via JIT vector search if embeddings are enabled).")
		return responseMsg(sb.String())
	}
}

func collectIngestFiles(root string, isDir bool) ([]string, error) {
	if !isDir {
		if isSupportedIngestExt(root) {
			return []string{root}, nil
		}
		return nil, nil
	}

	var files []string
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !isSupportedIngestExt(path) {
			return nil
		}
		files = append(files, path)
		return nil
	}); err != nil {
		return nil, err
	}
	return files, nil
}

func chunkTextRunes(text string, maxLen int) []string {
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

func isSupportedIngestExt(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".mdx", ".txt", ".rst", ".adoc", ".asciidoc",
		".yaml", ".yml", ".json", ".toml", ".ini", ".cfg",
		".go", ".ts", ".tsx", ".js", ".jsx", ".cs", ".java", ".py", ".mg", ".gl":
		return true
	default:
		return false
	}
}

func contentTypeForIngestPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".cs", ".java", ".py", ".mg", ".gl":
		return "code"
	case ".md", ".mdx", ".txt", ".rst", ".adoc", ".asciidoc",
		".yaml", ".yml", ".json", ".toml", ".ini", ".cfg":
		return "documentation"
	default:
		return "documentation"
	}
}
