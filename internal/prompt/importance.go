package prompt

import (
	"bytes"
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
)

// ImportanceLevel is how much harm follows when the model ignores an atom's
// guidance: Jev's Score answer, given once per atom at corpus build time
// (cmd/tools/prompt_builder -score-importance, internal/atomjudge). It replaced
// the hand-set `priority` field (0..100, 1,246 of ~1,700 atoms at the default
// 30) as the atom's importance input. Every level has a written meaning, and
// those meanings are the Score's criteria, word for word.
//
// Importance orders and arbitrates; it never admits. Mandatory (skeleton)
// atoms stay rule-selected whatever their level.
type ImportanceLevel int

// The levels, lowest first. A level is the Score's level number plus one, so
// the zero value is unscored: an atom built in code carries no importance
// until a score gives it one.
const (
	// ImportanceUnscored: the atom has no current build-time score. It is
	// either not in the embedded corpus (project, expert, evolved, strategy
	// or runtime-injected atoms) or its content changed after it was scored.
	// No importance fact reaches the kernel for it, so no importance rule
	// decides anything about it; Go orderings place it after every scored
	// atom of equal standing.
	ImportanceUnscored    ImportanceLevel = 0
	ImportanceStyle       ImportanceLevel = 1
	ImportanceConvention  ImportanceLevel = 2
	ImportanceEfficiency  ImportanceLevel = 3
	ImportanceCorrectness ImportanceLevel = 4
	ImportanceCritical    ImportanceLevel = 5
)

// importanceMeanings are the Score levels, in level order. Each is a
// situation that reads on its own (docs.typesafe.ai/primitives/score: describe
// situations, not degrees). Changing one changes the scale's fingerprint
// (atomjudge.ImportanceScaleFingerprint), which makes every stored score stale.
var importanceMeanings = [...]string{
	ImportanceStyle - 1:       "Ignoring this guidance changes only style or wording; the work is still correct.",
	ImportanceConvention - 1:  "Ignoring this guidance breaks a project convention or makes the result harder to review, but the result still works.",
	ImportanceEfficiency - 1:  "Ignoring this guidance wastes turns, tool calls or tokens, or causes rework, but the end result can still be correct.",
	ImportanceCorrectness - 1: "Ignoring this guidance produces a wrong result: incorrect code, a failing test, a broken contract or a wrong answer.",
	ImportanceCritical - 1:    "Ignoring this guidance breaks the build, loses or corrupts data, or violates safety or permissions.",
}

var importanceNames = [...]string{"style", "convention", "efficiency", "correctness", "critical"}

// ImportanceMeanings returns the written meaning of every level, lowest
// first: the Score's criteria.
func ImportanceMeanings() []string {
	return append([]string(nil), importanceMeanings[:]...)
}

// Scored reports whether the level is one of the written levels.
func (l ImportanceLevel) Scored() bool {
	return l >= ImportanceStyle && l <= ImportanceCritical
}

// Meaning is the level's written meaning; unscored says so.
func (l ImportanceLevel) Meaning() string {
	if !l.Scored() {
		return "No current build-time importance score."
	}
	return importanceMeanings[l-1]
}

func (l ImportanceLevel) String() string {
	if !l.Scored() {
		return "unscored"
	}
	return importanceNames[l-1]
}

// ImportanceFromScore reads a Score position (0..4, possibly between level
// numbers) as the nearest level, which is how TypeSafe reads a Score by
// default.
func ImportanceFromScore(score float64) (ImportanceLevel, error) {
	top := float64(len(importanceMeanings) - 1)
	if score < 0 || score > top || score != score {
		return ImportanceUnscored, fmt.Errorf("importance score %v outside 0..%v", score, top)
	}
	return ImportanceLevel(int(score+0.5)) + ImportanceStyle, nil
}

// ImportanceRecord is one atom's build-time judgment, as Jev returned it.
type ImportanceRecord struct {
	// ContentHash is HashContent of the content Jev judged; a different hash
	// on the live atom makes the record stale.
	ContentHash string `json:"content_hash"`
	// Level is the nearest level to Score (Score's level number plus one).
	Level ImportanceLevel `json:"level"`
	// Score is Jev's position on the scale and Probabilities its distribution
	// over the levels, kept so a two-peaked answer can be audited.
	Score         float64   `json:"score"`
	Probabilities []float64 `json:"probabilities"`
}

// ImportanceLedger is the build-time importance of every embedded atom
// (atom_importance.json), written by `prompt_builder -score-importance` and
// embedded in the binary: runtime pays nothing for it.
type ImportanceLedger struct {
	// Scale fingerprints the question and level wording the scores answered.
	Scale string `json:"scale"`
	// Model is the Jev model ID that answered.
	Model string                      `json:"model"`
	Atoms map[string]ImportanceRecord `json:"atoms"`
}

// Lookup returns the atom's level when the ledger holds a record for exactly
// this content, and why not otherwise.
func (l *ImportanceLedger) Lookup(id, contentHash string) (ImportanceLevel, bool) {
	if l == nil {
		return ImportanceUnscored, false
	}
	rec, ok := l.Atoms[id]
	if !ok || rec.ContentHash != contentHash || !rec.Level.Scored() {
		return ImportanceUnscored, false
	}
	return rec.Level, true
}

// MarshalIndent writes the ledger with one atom record per line, in ID
// order, so a rescore diffs as the atoms it changed.
func (l *ImportanceLedger) MarshalIndent() ([]byte, error) {
	head, err := json.Marshal(struct {
		Scale string `json:"scale"`
		Model string `json:"model"`
	}{l.Scale, l.Model})
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	b.Write(head[:len(head)-1])
	b.WriteString(",\"atoms\":{")
	ids := make([]string, 0, len(l.Atoms))
	for id := range l.Atoms {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for i, id := range ids {
		key, err := json.Marshal(id)
		if err != nil {
			return nil, err
		}
		rec, err := json.Marshal(l.Atoms[id])
		if err != nil {
			return nil, err
		}
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString("\n")
		b.Write(key)
		b.WriteByte(':')
		b.Write(rec)
	}
	b.WriteString("\n}}\n")
	return b.Bytes(), nil
}

// ParseImportanceLedger reads a ledger and checks every record.
func ParseImportanceLedger(data []byte) (*ImportanceLedger, error) {
	var l ImportanceLedger
	if err := json.Unmarshal(data, &l); err != nil {
		return nil, fmt.Errorf("importance ledger: %w", err)
	}
	if l.Atoms == nil {
		l.Atoms = map[string]ImportanceRecord{}
	}
	ids := make([]string, 0, len(l.Atoms))
	for id := range l.Atoms {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		rec := l.Atoms[id]
		if !rec.Level.Scored() {
			return nil, fmt.Errorf("importance ledger: %s has level %d outside the scale", id, rec.Level)
		}
		if rec.ContentHash == "" {
			return nil, fmt.Errorf("importance ledger: %s has no content hash", id)
		}
		if len(rec.Probabilities) != len(importanceMeanings) {
			return nil, fmt.Errorf("importance ledger: %s has %d probabilities, the scale has %d levels", id, len(rec.Probabilities), len(importanceMeanings))
		}
	}
	return &l, nil
}

//go:embed atom_importance.json
var embeddedImportanceJSON []byte

var (
	embeddedImportanceOnce sync.Once
	embeddedImportance     *ImportanceLedger
	embeddedImportanceErr  error
)

// EmbeddedImportanceLedger is the ledger shipped in the binary.
func EmbeddedImportanceLedger() (*ImportanceLedger, error) {
	embeddedImportanceOnce.Do(func() {
		embeddedImportance, embeddedImportanceErr = ParseImportanceLedger(embeddedImportanceJSON)
	})
	return embeddedImportance, embeddedImportanceErr
}

// applyImportance sets each atom's level from the ledger and returns how many
// atoms it left unscored (absent or stale), so the loader can say so.
func applyImportance(atoms []*PromptAtom, ledger *ImportanceLedger) (unscored []string) {
	for _, a := range atoms {
		if a == nil {
			continue
		}
		level, ok := ledger.Lookup(a.ID, a.ContentHash)
		a.Importance = level
		if !ok {
			unscored = append(unscored, a.ID)
		}
	}
	return unscored
}

// importanceColumn is the level as the prompt_atoms.importance column stores
// it: NULL for unscored.
func importanceColumn(l ImportanceLevel) any {
	if !l.Scored() {
		return nil
	}
	return int64(l)
}

// importanceFromColumn reads the column back; NULL or a value off the scale
// is unscored.
func importanceFromColumn(v sql.NullInt64) ImportanceLevel {
	if !v.Valid {
		return ImportanceUnscored
	}
	if l := ImportanceLevel(v.Int64); l.Scored() {
		return l
	}
	return ImportanceUnscored
}

// migrateImportanceColumn moves a corpus written before importance replaced
// priority: it adds the importance column and drops priority, whose hand-set
// values must not be read as levels. Idempotent.
func migrateImportanceColumn(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, "SELECT name FROM pragma_table_info('prompt_atoms')")
	if err != nil {
		return fmt.Errorf("read prompt_atoms columns: %w", err)
	}
	cols := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return fmt.Errorf("scan prompt_atoms column: %w", err)
		}
		cols[name] = true
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if !cols["importance"] {
		if _, err := db.ExecContext(ctx, "ALTER TABLE prompt_atoms ADD COLUMN importance INTEGER"); err != nil {
			return fmt.Errorf("add prompt_atoms.importance: %w", err)
		}
	}
	if cols["priority"] {
		if _, err := db.ExecContext(ctx, "ALTER TABLE prompt_atoms DROP COLUMN priority"); err != nil {
			return fmt.Errorf("drop prompt_atoms.priority: %w", err)
		}
	}
	return nil
}
