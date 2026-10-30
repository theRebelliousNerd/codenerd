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
	// Ecosystem is the Module's package ecosystem ("go", "npm"); go doc
	// reads only Go modules.
	Ecosystem string
	// Version is the version the workspace's manifest pins for Module, when
	// it pins one: recorded with every acquisition, so the refresh policy
	// can tell knowledge acquired at an older pin (knowledge_acquired_version).
	Version string
	// Gap marks a question a consultation found no knowledge for (Topic is
	// the task's own words): web search is asked the question itself.
	Gap bool
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
	// Concepts are the knowledge rows an acquisition wrote, so the next
	// acquisition of the same subject by the same source replaces them.
	Concepts []string
	// Version is the subject's pinned version at the attempt.
	Version string
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

// availability is a source that can say, before any call, whether it can
// run here at all.
type availability interface {
	Available() bool
}

// Acquirer runs every source on a subject.
type Acquirer struct {
	Sources []Source
	// Now stamps the outcomes; time.Now when nil.
	Now func() time.Time
}

// Available names the sources that can run here: those that do not say they
// cannot.
func (a Acquirer) Available() []string {
	var out []string
	for _, src := range a.Sources {
		if av, ok := src.(availability); ok && !av.Available() {
			continue
		}
		out = append(out, src.Name())
	}
	return out
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
			outcomes = append(outcomes, Outcome{Source: src.Name(), Subject: subject.Key(), Version: subject.Version, Status: StatusFailed, Detail: ctx.Err().Error()})
			continue
		}
		got, err := src.Acquire(ctx, subject)
		out := Outcome{Source: src.Name(), Subject: subject.Key(), Version: subject.Version}
		switch {
		case errors.Is(err, ErrUnavailable):
			out.Status, out.Detail = StatusUnavailable, err.Error()
		case err != nil:
			out.Status, out.Detail = StatusFailed, err.Error()
		case len(got) == 0:
			out.Status = StatusEmpty
		default:
			out.Status, out.Atoms = StatusAcquired, len(got)
			for _, atom := range got {
				out.Concepts = append(out.Concepts, atom.Concept)
			}
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
	concepts TEXT NOT NULL,
	version TEXT NOT NULL,
	PRIMARY KEY (source, subject)
)`

// Fill acquires knowledge for each subject and writes it into the KB: new
// atoms (deduplicated by concept and content, with their source and
// embeddings when the store has an engine) and every outcome. A source that
// already acquired a subject is not asked again (its recorded outcome
// stands until the refresh policy calls it stale, Refresh); a source that
// failed, answered nothing or could not run is asked again, since it may
// serve now. It returns how many atoms were added and the outcomes of the
// attempts made, in subject then source order.
func Fill(ctx context.Context, db *store.LocalStore, acq Acquirer, subjects []Subject) (int, []Outcome, error) {
	return fill(ctx, db, acq, subjects, false)
}

// Refresh acquires each subject again from every source, whatever it holds:
// the subjects the refresh policy derived (stale, version changed, a gap, a
// newly available source). A source that acquires a subject again replaces
// the knowledge rows its previous acquisition wrote; one that fails leaves
// them in place.
func Refresh(ctx context.Context, db *store.LocalStore, acq Acquirer, subjects []Subject) (int, []Outcome, error) {
	return fill(ctx, db, acq, subjects, true)
}

func fill(ctx context.Context, db *store.LocalStore, acq Acquirer, subjects []Subject, again bool) (int, []Outcome, error) {
	if db == nil {
		return 0, nil, fmt.Errorf("no knowledge base")
	}
	if err := ensureAcquisitionsTable(ctx, db.GetDB()); err != nil {
		return 0, nil, err
	}
	existing, err := db.GetAllKnowledgeAtoms()
	if err != nil {
		return 0, nil, fmt.Errorf("read knowledge: %w", err)
	}
	seen := make(map[string]bool, len(existing))
	hashesByConcept := make(map[string][]string, len(existing))
	for _, atom := range existing {
		hash := store.ComputeContentHash(atom.Concept, atom.Content)
		seen[hash] = true
		hashesByConcept[atom.Concept] = append(hashesByConcept[atom.Concept], hash)
	}
	prior, err := Outcomes(ctx, db.GetDB())
	if err != nil {
		return 0, nil, fmt.Errorf("read outcomes: %w", err)
	}
	acquired := map[string]map[string]bool{}
	previous := map[[2]string][]string{}
	for _, o := range prior {
		previous[[2]string{o.Source, o.Subject}] = o.Concepts
		if o.Status == StatusAcquired && !again {
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
		// A source that acquired the subject again replaces what its last
		// acquisition wrote, the rows it writes again included (their
		// content may have changed under the same concept).
		var replaced []string
		for _, out := range outcomes {
			if out.Status != StatusAcquired {
				continue
			}
			for _, concept := range previous[[2]string{out.Source, out.Subject}] {
				if concept == "" {
					continue
				}
				replaced = append(replaced, concept)
				for _, hash := range hashesByConcept[concept] {
					delete(seen, hash)
				}
				delete(hashesByConcept, concept)
			}
		}
		if len(replaced) > 0 {
			if _, err := db.DeleteKnowledgeAtomsByConcept(replaced...); err != nil {
				return added, all, fmt.Errorf("drop replaced knowledge: %w", err)
			}
		}
		for _, atom := range atoms {
			hash := store.ComputeContentHash(atom.Concept, atom.Content)
			if seen[hash] {
				continue
			}
			if err := db.StoreKnowledgeAtomWithEmbedding(ctx, atom.Concept, atom.Content, atom.Confidence, atom.Source); err != nil {
				return added, all, fmt.Errorf("store %s: %w", atom.Concept, err)
			}
			seen[hash] = true
			hashesByConcept[atom.Concept] = append(hashesByConcept[atom.Concept], hash)
			added++
		}
		at := now().UTC().Format(time.RFC3339)
		for _, out := range outcomes {
			concepts := out.Concepts
			if out.Status != StatusAcquired {
				// A failed or empty attempt replaces nothing: the rows the
				// last acquisition wrote are still what the KB holds.
				concepts = previous[[2]string{out.Source, out.Subject}]
			}
			if _, err := db.GetDB().ExecContext(ctx,
				`INSERT INTO knowledge_acquisitions (source, subject, status, atoms, detail, attempted_at, concepts, version) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
				 ON CONFLICT(source, subject) DO UPDATE SET status = excluded.status, atoms = excluded.atoms, detail = excluded.detail, attempted_at = excluded.attempted_at, concepts = excluded.concepts, version = excluded.version`,
				out.Source, out.Subject, out.Status, out.Atoms, out.Detail, at, strings.Join(concepts, "\n"), out.Version); err != nil {
				return added, all, fmt.Errorf("record outcome: %w", err)
			}
		}
		all = append(all, outcomes...)
	}
	return added, all, nil
}

// acquisitionsColumns are the columns added to knowledge_acquisitions after
// its first release, with the definition each is added with.
var acquisitionsColumns = []struct{ name, def string }{
	{"concepts", "TEXT NOT NULL DEFAULT ''"},
	{"version", "TEXT NOT NULL DEFAULT ''"},
}

// ensureAcquisitionsTable creates the outcomes table, or adds to one an
// earlier build created the columns it lacks.
func ensureAcquisitionsTable(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, acquisitionsSchema); err != nil {
		return fmt.Errorf("acquisitions table: %w", err)
	}
	cols, err := tableColumnSet(ctx, db, "knowledge_acquisitions")
	if err != nil {
		return err
	}
	for _, col := range acquisitionsColumns {
		if cols[col.name] {
			continue
		}
		if _, err := db.ExecContext(ctx, `ALTER TABLE knowledge_acquisitions ADD COLUMN `+col.name+` `+col.def); err != nil {
			return fmt.Errorf("add acquisitions column %s: %w", col.name, err)
		}
	}
	return nil
}

// tableColumnSet names a table's columns; an absent table has none.
func tableColumnSet(ctx context.Context, db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return nil, fmt.Errorf("columns of %s: %w", table, err)
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

// Outcomes reads a KB's recorded acquisition outcomes, by source then subject.
func Outcomes(ctx context.Context, db *sql.DB) ([]Outcome, error) {
	cols, err := tableColumnSet(ctx, db, "knowledge_acquisitions")
	if err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return nil, nil
	}
	// A table an earlier build created may lack the later columns; reading
	// it (check-kb opens KBs read-only) reports them empty.
	concepts, version := "''", "''"
	if cols["concepts"] {
		concepts = "concepts"
	}
	if cols["version"] {
		version = "version"
	}
	rows, err := db.QueryContext(ctx, `SELECT source, subject, status, atoms, detail, `+concepts+`, `+version+` FROM knowledge_acquisitions ORDER BY source, subject`)
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
		var concepts string
		if err := rows.Scan(&o.Source, &o.Subject, &o.Status, &o.Atoms, &o.Detail, &concepts, &o.Version); err != nil {
			return nil, err
		}
		if concepts != "" {
			o.Concepts = strings.Split(concepts, "\n")
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
