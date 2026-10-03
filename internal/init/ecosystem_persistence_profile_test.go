package init

import (
	"context"
	"database/sql"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/embedding"
	"codenerd/internal/prompt"
	"codenerd/internal/store"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

func TestPersistEcosystemAtoms_ProfileHotPersistsWALAndPreservesRows(t *testing.T) {
	for _, withEmbedding := range []bool{false, true} {
		name := "without_embedding"
		if withEmbedding {
			name = "with_embedding"
		}
		t.Run(name, func(t *testing.T) {
			workspace, corpus, before, beforeTags := ecosystemProfileFixture(t)
			atom := ecosystemProfileAtom()
			engine := &ecosystemProfileEmbedding{}
			ini := &Initializer{config: InitConfig{Workspace: workspace}}
			if withEmbedding {
				ini.embedEngine = engine
			}
			type contextKey struct{}
			ctx := context.WithValue(context.Background(), contextKey{}, "ecosystem-caller")
			result := &InitResult{}

			ini.persistEcosystemAtoms(ctx, []*store.PromptAtom{atom}, result)

			require.Empty(t, result.Warnings)
			require.Empty(t, result.Failures)
			require.Equal(t, []*store.PromptAtom{atom}, ini.projectAtoms)
			ecosystemProfileHeader(t, corpus, 2)
			db := ecosystemProfileRawOpen(t, corpus)
			var journal string
			require.NoError(t, db.QueryRow("PRAGMA journal_mode").Scan(&journal))
			require.Equal(t, "wal", strings.ToLower(journal))
			require.Equal(t, before, ecosystemProfileExistingRow(t, db))
			require.Equal(t, beforeTags, ecosystemProfileTags(t, db, "existing/curated"))

			var got store.PromptAtom
			var task, source sql.NullString
			require.NoError(t, db.QueryRow(`
				SELECT atom_id, version, content, token_count, content_hash,
				       category, subcategory, priority, is_mandatory,
				       embedding, embedding_task, source_file
				FROM prompt_atoms WHERE atom_id = ?`, atom.AtomID).Scan(
				&got.AtomID, &got.Version, &got.Content, &got.TokenCount, &got.ContentHash,
				&got.Category, &got.Subcategory, &got.Priority, &got.IsMandatory,
				&got.Embedding, &task, &source))
			want := *atom
			want.Languages = nil
			want.Frameworks = nil
			if withEmbedding {
				want.Embedding = ecosystemProfileVectorBytes()
			}
			require.Equal(t, want, got, "identity, content, metadata and embedding must survive persistence")
			require.False(t, source.Valid, "ecosystem atoms remain project-owned")
			require.Equal(t, []string{"framework:/gin:0", "lang:/go:0"}, ecosystemProfileTags(t, db, atom.AtomID))
			if withEmbedding {
				require.Equal(t, sql.NullString{String: "RETRIEVAL_DOCUMENT", Valid: true}, task)
				require.Equal(t, 1, engine.calls)
				require.Equal(t, ctx, engine.caller, "loader must receive the caller's context")
				require.Equal(t, atom.Content, engine.text)
				require.Equal(t, "RETRIEVAL_DOCUMENT", engine.task)
			} else {
				require.Empty(t, got.Embedding)
				require.False(t, task.Valid)
				require.Zero(t, engine.calls)
			}
			require.NoError(t, db.Close())

			ini.persistEcosystemAtoms(ctx, []*store.PromptAtom{atom}, result)
			require.Empty(t, result.Warnings)
			ecosystemProfileHeader(t, corpus, 2)
			reopened := ecosystemProfileRawOpen(t, corpus)
			var count int
			require.NoError(t, reopened.QueryRow("SELECT count(*) FROM prompt_atoms").Scan(&count))
			require.Equal(t, 2, count)
			require.Equal(t, before, ecosystemProfileExistingRow(t, reopened))
			require.Equal(t, beforeTags, ecosystemProfileTags(t, reopened, "existing/curated"))
		})
	}
}

func TestPersistEcosystemAtoms_ProfileHotMissingCorpus(t *testing.T) {
	workspace := t.TempDir()
	ini := &Initializer{config: InitConfig{Workspace: workspace}}
	atom := ecosystemProfileAtom()
	result := &InitResult{}

	ini.persistEcosystemAtoms(context.Background(), []*store.PromptAtom{atom}, result)

	require.Empty(t, result.Warnings)
	require.Empty(t, result.Failures)
	require.Equal(t, []*store.PromptAtom{atom}, ini.projectAtoms)
	_, err := os.Stat(filepath.Join(workspace, ".nerd", "prompts", "corpus.db"))
	require.True(t, os.IsNotExist(err), "missing corpus must not be created")
}

func TestPersistEcosystemAtoms_ProfileHotCorpusErrorsRemainVisible(t *testing.T) {
	for _, name := range []string{"malformed", "open_error"} {
		t.Run(name, func(t *testing.T) {
			workspace := t.TempDir()
			corpus := filepath.Join(workspace, ".nerd", "prompts", "corpus.db")
			require.NoError(t, os.MkdirAll(filepath.Dir(corpus), 0o755))
			malformed := []byte(strings.Repeat("not a SQLite database\n", 20))
			if name == "malformed" {
				require.NoError(t, os.WriteFile(corpus, malformed, 0o600))
			} else {
				require.NoError(t, os.Mkdir(corpus, 0o755))
			}
			atom := ecosystemProfileAtom()
			ini := &Initializer{config: InitConfig{Workspace: workspace}}
			result := &InitResult{}

			ini.persistEcosystemAtoms(context.Background(), []*store.PromptAtom{atom}, result)

			require.Len(t, result.Warnings, 1)
			require.Contains(t, result.Warnings[0], "prompt atom "+atom.AtomID+" was not ingested:")
			require.Empty(t, result.Failures)
			require.Equal(t, []*store.PromptAtom{atom}, ini.projectAtoms)
			if name == "malformed" {
				got, err := os.ReadFile(corpus)
				require.NoError(t, err)
				require.Equal(t, malformed, got, "failed open must not replace the corpus")
			}
		})
	}
}

func TestPersistEcosystemAtoms_ProfileHotCanceledContext(t *testing.T) {
	for _, name := range []string{"before_open", "during_embedding"} {
		t.Run(name, func(t *testing.T) {
			workspace, corpus, before, beforeTags := ecosystemProfileFixture(t)
			engine := &ecosystemProfileEmbedding{}
			ini := &Initializer{config: InitConfig{Workspace: workspace}, embedEngine: engine}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if name == "before_open" {
				cancel()
			} else {
				engine.cancel = cancel
			}
			atom := ecosystemProfileAtom()
			result := &InitResult{}

			ini.persistEcosystemAtoms(ctx, []*store.PromptAtom{atom}, result)

			require.Len(t, result.Warnings, 1)
			require.Contains(t, result.Warnings[0], "prompt atom "+atom.AtomID+" was not ingested:")
			require.Contains(t, result.Warnings[0], context.Canceled.Error())
			require.Empty(t, result.Failures)
			require.Equal(t, []*store.PromptAtom{atom}, ini.projectAtoms)
			if name == "before_open" {
				require.Zero(t, engine.calls, "canceled persistence must not request an embedding")
				ecosystemProfileHeader(t, corpus, 1)
			} else {
				require.Equal(t, 1, engine.calls)
				require.Equal(t, ctx, engine.caller)
				ecosystemProfileHeader(t, corpus, 2)
			}
			db := ecosystemProfileRawOpen(t, corpus)
			var count int
			require.NoError(t, db.QueryRow("SELECT count(*) FROM prompt_atoms WHERE atom_id = ?", atom.AtomID).Scan(&count))
			require.Zero(t, count, "cancellation must roll back the atom transaction")
			require.Equal(t, before, ecosystemProfileExistingRow(t, db))
			require.Equal(t, beforeTags, ecosystemProfileTags(t, db, "existing/curated"))
		})
	}
}

func ecosystemProfileFixture(t *testing.T) (string, string, []any, []string) {
	t.Helper()
	workspace := t.TempDir()
	corpus := filepath.Join(workspace, ".nerd", "prompts", "corpus.db")
	require.NoError(t, os.MkdirAll(filepath.Dir(corpus), 0o755))
	db := ecosystemProfileRawOpen(t, corpus)
	var mode string
	require.NoError(t, db.QueryRow("PRAGMA journal_mode = DELETE").Scan(&mode))
	require.Equal(t, "delete", strings.ToLower(mode))
	require.NoError(t, prompt.NewAtomLoader(nil).EnsureSchema(context.Background(), db))
	_, err := db.Exec(`
		INSERT INTO prompt_atoms (
			atom_id, version, content, token_count, content_hash, description,
			content_concise, content_min, category, subcategory, priority,
			is_mandatory, is_exclusive, depends_on, conflicts_with, embedding,
			embedding_task, embedding_model, source_file, created_at
		) VALUES (?, 7, 'curated content', 23, 'curated hash', 'curated description',
			'concise', 'minimum', 'domain', 'curated', 83, 1, 'exclusive group',
			'["other/required"]', '["other/conflict"]', ?, 'RETRIEVAL_QUERY',
			'curated-model', 'curated/source.yaml', '2024-01-02 03:04:05')`,
		"existing/curated", []byte{0, 255, 1, 128, 0, 42, 7, 9})
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO atom_context_tags (atom_id, dimension, tag, is_exclusion) VALUES (?, 'lang', '/rust', 1)", "existing/curated")
	require.NoError(t, err)
	before := ecosystemProfileExistingRow(t, db)
	tags := ecosystemProfileTags(t, db, "existing/curated")
	require.NoError(t, db.Close(), "fixture must close before the initializer opens it")
	ecosystemProfileHeader(t, corpus, 1)
	return workspace, corpus, before, tags
}

func ecosystemProfileRawOpen(t *testing.T, corpus string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", corpus)
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	return db
}

func ecosystemProfileHeader(t *testing.T, corpus string, version byte) {
	t.Helper()
	header, err := os.ReadFile(corpus)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(header), 20)
	require.Equal(t, "SQLite format 3\x00", string(header[:16]))
	require.Equal(t, []byte{version, version}, header[18:20])
}

func ecosystemProfileExistingRow(t *testing.T, db *sql.DB) []any {
	t.Helper()
	rows, err := db.Query("SELECT * FROM prompt_atoms WHERE atom_id = ?", "existing/curated")
	require.NoError(t, err)
	defer rows.Close()
	columns, err := rows.Columns()
	require.NoError(t, err)
	values := make([]any, len(columns))
	dest := make([]any, len(columns))
	for n := range values {
		dest[n] = &values[n]
	}
	require.True(t, rows.Next(), "preexisting row must survive")
	require.NoError(t, rows.Scan(dest...))
	for n, value := range values {
		if blob, ok := value.([]byte); ok {
			values[n] = append([]byte(nil), blob...)
		}
	}
	require.False(t, rows.Next())
	require.NoError(t, rows.Err())
	return values
}

func ecosystemProfileTags(t *testing.T, db *sql.DB, atomID string) []string {
	t.Helper()
	rows, err := db.Query(`SELECT dimension || ':' || tag || ':' || is_exclusion
		FROM atom_context_tags WHERE atom_id = ? ORDER BY dimension, tag`, atomID)
	require.NoError(t, err)
	defer rows.Close()
	var tags []string
	for rows.Next() {
		var tag string
		require.NoError(t, rows.Scan(&tag))
		tags = append(tags, tag)
	}
	require.NoError(t, rows.Err())
	return tags
}

func ecosystemProfileAtom() *store.PromptAtom {
	return &store.PromptAtom{
		AtomID:      "project/ecosystem/agents/rule/profile_test",
		Version:     3,
		Content:     "source: .agents/rules/profile.go\ntool: agents\nkind: rule\nscope: repository\nKeep existing ecosystem knowledge.",
		TokenCount:  29,
		ContentHash: "ecosystem-content-hash",
		Category:    "domain",
		Subcategory: "repository",
		Languages:   []string{"/go"},
		Frameworks:  []string{"/gin"},
		Priority:    60,
		IsMandatory: true,
	}
}

type ecosystemProfileEmbedding struct {
	caller context.Context
	cancel context.CancelFunc
	text   string
	task   string
	calls  int
}

var _ embedding.TaskTypeAwareEngine = (*ecosystemProfileEmbedding)(nil)

func (e *ecosystemProfileEmbedding) Embed(ctx context.Context, text string) ([]float32, error) {
	return e.EmbedWithTask(ctx, text, "")
}

func (e *ecosystemProfileEmbedding) EmbedWithTask(ctx context.Context, text, task string) ([]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	e.caller, e.text, e.task = ctx, text, task
	e.calls++
	if e.cancel != nil {
		e.cancel()
		return nil, ctx.Err()
	}
	return []float32{0.25, -0.5, 1, 2}, nil
}

func (e *ecosystemProfileEmbedding) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	vectors := make([][]float32, len(texts))
	for n, text := range texts {
		vector, err := e.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		vectors[n] = vector
	}
	return vectors, nil
}

func (*ecosystemProfileEmbedding) Dimensions() int { return 4 }
func (*ecosystemProfileEmbedding) Name() string    { return "ecosystem-profile-test" }

func ecosystemProfileVectorBytes() []byte {
	vector := []float32{0.25, -0.5, 1, 2}
	blob := make([]byte, 4*len(vector))
	for n, value := range vector {
		binary.LittleEndian.PutUint32(blob[n*4:], math.Float32bits(value))
	}
	return blob
}
