package knowledge

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/types"
)

// Measure reads one agent KB as the facts the refresh policy decides over
// (schemas_kb_refresh.mg): every source's last outcome on every subject, the
// acquired knowledge it holds with its age, what consultations retrieved, and
// the questions they found nothing for. agent is the agent's name constant
// (/goexpert). Ages are whole days before now.
func Measure(ctx context.Context, db *sql.DB, agent any, now time.Time) ([]types.Fact, error) {
	if db == nil {
		return nil, nil
	}
	tables, err := tableSet(ctx, db)
	if err != nil {
		return nil, err
	}
	nowUnix := now.UTC().Unix()
	days := func(unix sql.NullInt64) int64 {
		if !unix.Valid || unix.Int64 > nowUnix {
			return 0
		}
		return (nowUnix - unix.Int64) / int64((24 * time.Hour).Seconds())
	}
	var facts []types.Fact

	if tables["knowledge_acquisitions"] {
		version := "''"
		if cols, err := tableColumnSet(ctx, db, "knowledge_acquisitions"); err != nil {
			return nil, err
		} else if cols["version"] {
			version = "version"
		}
		rows, err := db.QueryContext(ctx, `SELECT source, subject, status, CAST(strftime('%s', attempted_at) AS INTEGER), `+version+` FROM knowledge_acquisitions`)
		if err != nil {
			return nil, fmt.Errorf("read outcomes: %w", err)
		}
		err = scanRows(rows, func(r *sql.Rows) error {
			var source, subject, status, pinned string
			var at sql.NullInt64
			if err := r.Scan(&source, &subject, &status, &at, &pinned); err != nil {
				return err
			}
			facts = append(facts, types.Fact{Predicate: "knowledge_outcome", Args: []any{agent, subject, source, types.MangleAtom("/" + status), days(at)}})
			if status == StatusAcquired && pinned != "" {
				facts = append(facts, types.Fact{Predicate: "knowledge_acquired_version", Args: []any{agent, subject, pinned}})
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("read outcomes: %w", err)
		}
	}

	if tables["knowledge_atoms"] {
		rows, err := db.QueryContext(ctx, `SELECT concept, content, COALESCE(source, ''), CAST(strftime('%s', created_at) AS INTEGER) FROM knowledge_atoms`)
		if err != nil {
			return nil, fmt.Errorf("read knowledge: %w", err)
		}
		err = scanRows(rows, func(r *sql.Rows) error {
			var concept, content, source string
			var at sql.NullInt64
			if err := r.Scan(&concept, &content, &source, &at); err != nil {
				return err
			}
			if IsLegacyPlaceholder(concept, content) {
				return nil
			}
			facts = append(facts, types.Fact{Predicate: "knowledge_atom_held", Args: []any{agent, concept, SourceKind(source), days(at)}})
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("read knowledge: %w", err)
		}
	}

	if tables["knowledge_usage"] {
		rows, err := db.QueryContext(ctx, `SELECT concept, retrieved FROM knowledge_usage`)
		if err != nil {
			return nil, fmt.Errorf("read usage: %w", err)
		}
		err = scanRows(rows, func(r *sql.Rows) error {
			var concept string
			var n int64
			if err := r.Scan(&concept, &n); err != nil {
				return err
			}
			facts = append(facts, types.Fact{Predicate: "knowledge_atom_retrieved", Args: []any{agent, concept, n}})
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("read usage: %w", err)
		}
	}

	if tables["knowledge_misses"] {
		rows, err := db.QueryContext(ctx, `SELECT query, misses FROM knowledge_misses`)
		if err != nil {
			return nil, fmt.Errorf("read misses: %w", err)
		}
		err = scanRows(rows, func(r *sql.Rows) error {
			var query string
			var n int64
			if err := r.Scan(&query, &n); err != nil {
				return err
			}
			facts = append(facts, types.Fact{Predicate: "knowledge_miss", Args: []any{agent, query, n}})
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("read misses: %w", err)
		}
	}
	return facts, nil
}

// SourceFamily groups a knowledge row's provenance: web search keeps its
// provider ("web/openai"), a versioned or per-module source collapses to its
// kind ("godoc", "repo"); a row with no provenance is "(none)".
func SourceFamily(source string) string {
	if source == "" {
		return "(none)"
	}
	kind, rest, found := strings.Cut(source, "/")
	if kind == "web" && found {
		provider, _, _ := strings.Cut(rest, "/")
		return "web/" + provider
	}
	return kind
}

// SourceKind is the producer of a knowledge row ("web", "context7",
// "godoc", "repo"); a row with no provenance is "(none)".
func SourceKind(source string) string {
	if source == "" {
		return "(none)"
	}
	kind, _, _ := strings.Cut(source, "/")
	return kind
}

func tableSet(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'table'`)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	out := map[string]bool{}
	err = scanRows(rows, func(r *sql.Rows) error {
		var name string
		if err := r.Scan(&name); err != nil {
			return err
		}
		out[name] = true
		return nil
	})
	return out, err
}

func scanRows(rows *sql.Rows, each func(*sql.Rows) error) error {
	defer rows.Close()
	for rows.Next() {
		if err := each(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}
