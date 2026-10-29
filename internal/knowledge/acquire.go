// Package knowledge acquires a shard agent's knowledge from many sources.
//
// A shard agent's knowledge base is filled by producers, each a typed
// Source with its own provenance: grounded web search on every configured
// provider that offers it, Context7 library documentation, the exact pinned
// version of a Go module's own documentation from the module cache, and the
// repository's own use of the technology. Every attempt has an Outcome --
// acquired, empty, failed or unavailable -- recorded in the KB's
// knowledge_acquisitions table, so a source that did not deliver is visible
// (init's summary, `nerd check-kb`), never a silent skip. Nothing is filled
// with placeholders: an agent no source taught has no knowledge, and says so.
package knowledge

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"codenerd/internal/store"
)

// Subject is what an agent's knowledge is acquired about.
type Subject struct {
	// Agent is the agent's identity (its lower-cased registry name).
	Agent string
	// Topic is a research topic in prose ("go concurrency", "cobra command
	// trees"), what web search and Context7 are asked about.
	Topic string
	// Technology is the technology key the topic belongs to ("cobra"), when
	// it is one of the agent's domains.
	Technology string
	// Module is a package the workspace pins for the technology
	// ("github.com/spf13/cobra"), what the pinned-documentation and
	// repository sources read. Empty when the technology pins none.
	Module string
}

// Key is the subject's identity in the acquisitions table.
func (s Subject) Key() string {
	if s.Module != "" {
		return s.Module
	}
	return s.Topic
}

// Atom is one piece of knowledge a source produced.
type Atom struct {
	Concept    string
	Content    string
	Confidence float64
	// Source is the producer and its identity, stored as the row's
	// provenance ("web/openai", "context7", "godoc/github.com/x/y@v1.2.3",
	// "repo/world").
	Source string
}

// Outcome statuses.
const (
	StatusAcquired    = "acquired"
	StatusEmpty       = "empty"
	StatusFailed      = "failed"
	StatusUnavailable = "unavailable"
)

// ErrUnavailable marks a source that cannot run here at all (no provider
// offers grounded search, no module pinned for the subject); wrap it.
var ErrUnavailable = errors.New("source unavailable")

// Outcome is one source's attempt on one subject.
type Outcome struct {
	Source  string
	Subject string
	Status  string
	Atoms   int
	Detail  string
}

func (o Outcome) String() string {
	s := fmt.Sprintf("%s on %s: %s", o.Source, o.Subject, o.Status)
	if o.Status == StatusAcquired {
		s += fmt.Sprintf(" (%d atom(s))", o.Atoms)
	}
	if o.Detail != "" {
		s += ": " + o.Detail
	}
	return s
}

// Source is one knowledge producer.
type Source interface {
	// Name is the source's provenance prefix ("web/openai", "context7").
	Name() string
	// Acquire returns what the source knows about the subject. An error
	// wrapping ErrUnavailable means the source cannot run for it; any other
	// error is a failure. No atoms and no error is an empty answer.
	Acquire(ctx context.Context, subject Subject) ([]Atom, error)
}

// Acquirer runs every source on a subject.
type Acquirer struct {
	Sources []Source
	// Now stamps the outcomes; time.Now when nil.
	Now func() time.Time
}

// Acquire runs each source on the subject and returns the atoms in source
// order with one outcome per source.
func (a Acquirer) Acquire(ctx context.Context, subject Subject) ([]Atom, []Outcome) {
	return a.acquire(ctx, subject, nil)
}

// acquire runs the sources done does not name.
func (a Acquirer) acquire(ctx context.Context, subject Subject, done map[string]bool) ([]Atom, []Outcome) {
	var atoms []Atom
	outcomes := make([]Outcome, 0, len(a.Sources))
	for _, src := range a.Sources {
		if done[src.Name()] {
			continue
		}
		if ctx.Err() != nil {
			outcomes = append(outcomes, Outcome{Source: src.Name(), Subject: subject.Key(), Status: StatusFailed, Detail: ctx.Err().Error()})
			continue
		}
		got, err := src.Acquire(ctx, subject)
		out := Outcome{Source: src.Name(), Subject: subject.Key()}
		switch {
		case errors.Is(err, ErrUnavailable):
			out.Status, out.Detail = StatusUnavailable, err.Error()
		case err != nil:
			out.Status, out.Detail = StatusFailed, err.Error()
		case len(got) == 0:
			out.Status = StatusEmpty
		default:
			out.Status, out.Atoms = StatusAcquired, len(got)
			atoms = append(atoms, got...)
		}
		outcomes = append(outcomes, out)
	}
	return atoms, outcomes
}

// acquisitionsSchema is the outcomes table every agent KB carries: the last
// attempt of each source on each subject.
const acquisitionsSchema = `CREATE TABLE IF NOT EXISTS knowledge_acquisitions (
	source TEXT NOT NULL,
	subject TEXT NOT NULL,
	status TEXT NOT NULL,
	atoms INTEGER NOT NULL,
	detail TEXT NOT NULL,
	attempted_at TEXT NOT NULL,
	PRIMARY KEY (source, subject)
)`

// Fill acquires knowledge for each subject and writes it into the KB: new
// atoms (deduplicated by concept and content, with their source and
// embeddings when the store has an engine) and every outcome. A source that
// already acquired a subject is not asked again (its recorded outcome
// stands): what is stale is decided by the refresh policy, which clears the
// outcome; a source that failed, answered nothing or could not run is asked
// again, since it may serve now. It returns how many atoms were added and
// the outcomes of the attempts made, in subject then source order.
func Fill(ctx context.Context, db *store.LocalStore, acq Acquirer, subjects []Subject) (int, []Outcome, error) {
	if db == nil {
		return 0, nil, fmt.Errorf("no knowledge base")
	}
	if _, err := db.GetDB().ExecContext(ctx, acquisitionsSchema); err != nil {
		return 0, nil, fmt.Errorf("acquisitions table: %w", err)
	}
	existing, err := db.GetAllKnowledgeAtoms()
	if err != nil {
		return 0, nil, fmt.Errorf("read knowledge: %w", err)
	}
	seen := make(map[string]bool, len(existing))
	for _, atom := range existing {
		seen[store.ComputeContentHash(atom.Concept, atom.Content)] = true
	}
	prior, err := Outcomes(ctx, db.GetDB())
	if err != nil {
		return 0, nil, fmt.Errorf("read outcomes: %w", err)
	}
	acquired := map[string]map[string]bool{}
	for _, o := range prior {
		if o.Status == StatusAcquired {
			if acquired[o.Subject] == nil {
				acquired[o.Subject] = map[string]bool{}
			}
			acquired[o.Subject][o.Source] = true
		}
	}
	now := time.Now
	if acq.Now != nil {
		now = acq.Now
	}
	added := 0
	var all []Outcome
	for _, subject := range subjects {
		atoms, outcomes := acq.acquire(ctx, subject, acquired[subject.Key()])
		for _, atom := range atoms {
			hash := store.ComputeContentHash(atom.Concept, atom.Content)
			if seen[hash] {
				continue
			}
			if err := db.StoreKnowledgeAtomWithEmbedding(ctx, atom.Concept, atom.Content, atom.Confidence, atom.Source); err != nil {
				return added, all, fmt.Errorf("store %s: %w", atom.Concept, err)
			}
			seen[hash] = true
			added++
		}
		at := now().UTC().Format(time.RFC3339)
		for _, out := range outcomes {
			if _, err := db.GetDB().ExecContext(ctx,
				`INSERT INTO knowledge_acquisitions (source, subject, status, atoms, detail, attempted_at) VALUES (?, ?, ?, ?, ?, ?)
				 ON CONFLICT(source, subject) DO UPDATE SET status = excluded.status, atoms = excluded.atoms, detail = excluded.detail, attempted_at = excluded.attempted_at`,
				out.Source, out.Subject, out.Status, out.Atoms, out.Detail, at); err != nil {
				return added, all, fmt.Errorf("record outcome: %w", err)
			}
		}
		all = append(all, outcomes...)
	}
	return added, all, nil
}

// Outcomes reads a KB's recorded acquisition outcomes, by source then subject.
func Outcomes(ctx context.Context, db *sql.DB) ([]Outcome, error) {
	rows, err := db.QueryContext(ctx, `SELECT source, subject, status, atoms, detail FROM knowledge_acquisitions ORDER BY source, subject`)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such table") {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	var out []Outcome
	for rows.Next() {
		var o Outcome
		if err := rows.Scan(&o.Source, &o.Subject, &o.Status, &o.Atoms, &o.Detail); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// SummarizeOutcomes counts outcomes by status, for one line in a report.
func SummarizeOutcomes(outcomes []Outcome) string {
	counts := map[string]int{}
	for _, o := range outcomes {
		counts[o.Status]++
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%d %s", counts[k], k))
	}
	return strings.Join(parts, ", ")
}
