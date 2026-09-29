package orient

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"codenerd/internal/atomicfile"
	"codenerd/internal/config"
	"codenerd/internal/embedding"
	"codenerd/internal/types"
)

// Snapshot contains measurements, never cached executive judgments.
type Snapshot struct {
	Head    string              `json:"head"`
	Config  config.OrientConfig `json:"config"`
	Facts   []types.Fact        `json:"facts"`
	Bodies  map[string]string   `json:"bodies"`
	Sources []Source            `json:"sources"`
	Notes   []string            `json:"notes"`
}

var snapshotMu sync.Mutex

func number(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case json.Number:
		i, _ := n.Int64()
		return i
	case float64:
		return int64(n)
	}
	return 0
}

func documentDigest(body string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(body))) }

func Head(ctx context.Context, root string) (string, error) {
	var err error
	root, err = absPath(root)
	if err != nil {
		return "", err
	}
	if err := ensureWorkTree(ctx, root); err != nil {
		return "", err
	}
	out, err := gitCmd(ctx, root, "-c", "safe.directory=*", "rev-parse", "--verify", "--quiet", "HEAD").Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			return "", nil // An unborn work tree has no head.
		}
		return "", fmt.Errorf("orientation HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Measure supplies the one initial history, document and ecosystem census.
func Measure(ctx context.Context, root string, cfg config.OrientConfig, emb embedding.EmbeddingEngine) (*Snapshot, error) {
	before, err := Head(ctx, root)
	if err != nil {
		return nil, err
	}
	history, err := ScanHistory(ctx, root)
	if err != nil {
		return nil, err
	}
	docs, err := CollectDocs(ctx, root, cfg, emb)
	if err != nil {
		return nil, err
	}
	sources, discErr := Discover(root)
	s := &Snapshot{Head: before, Config: cfg.WithDefaults(), Bodies: docs.Bodies, Sources: sources}
	s.Facts = append(s.Facts, history.Facts...)
	s.Facts = append(s.Facts, docs.Facts...)
	s.Facts = append(s.Facts, Facts(sources)...)
	s.Notes = append(s.Notes, docs.SimilarityNote)
	if discErr != nil {
		s.Notes = append(s.Notes, "Agent sources were only partly read: "+discErr.Error())
	}
	for _, p := range docs.Unreadable {
		s.Notes = append(s.Notes, "Unreadable tracked document: "+p)
	}
	s.Facts = append(s.Facts, DocTieFacts(tiePaths(s.Facts))...)
	s.Facts = append(s.Facts, types.Fact{Predicate: "oriented_head", Args: []any{before}})
	for p, body := range s.Bodies {
		s.Facts = append(s.Facts, types.Fact{Predicate: "oriented_document", Args: []any{p, documentDigest(body)}})
	}
	after, err := Head(ctx, root)
	if err != nil {
		return nil, err
	}
	if before != after {
		return nil, fmt.Errorf("repository HEAD changed during orientation; retry on a stable revision")
	}
	return s, nil
}

func (s *Snapshot) Engine(ctx context.Context) (*Engine, error) {
	e, err := NewEngine(&s.Config)
	if err != nil {
		return nil, err
	}
	if err = e.Assert(s.Facts); err == nil {
		err = e.Evaluate(ctx)
	}
	if err != nil {
		_ = e.Close()
		return nil, err
	}
	return e, nil
}

func snapshotPath(root string) string {
	return filepath.Join(root, ".nerd", "orientation", "snapshot.json")
}

func LoadSnapshot(root string) (*Snapshot, error) {
	body, err := os.ReadFile(snapshotPath(root))
	if err != nil {
		return nil, err
	}
	var s Snapshot
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&s); err != nil {
		return nil, fmt.Errorf("orientation snapshot: %w", err)
	}
	for i := range s.Facts {
		for j, arg := range s.Facts[i].Args {
			if n, ok := arg.(json.Number); ok {
				value, err := n.Int64()
				if err != nil {
					return nil, err
				}
				s.Facts[i].Args[j] = value
			} else if text, ok := arg.(string); ok && strings.HasPrefix(text, "/") {
				s.Facts[i].Args[j] = types.MangleAtom(text)
			}
		}
	}
	return &s, nil
}

var declarationPattern = regexp.MustCompile(`(?m)^Decl ([a-zA-Z0-9_]+)\([^\n]*\) bound \[[^\n]*\]\.`)

var projectedPredicates = []string{
	"repo_file_history", "repo_file_day", "repo_month", "repo_month_span", "repo_span",
	"doc_file", "doc_link", "doc_similar", "doc_cluster", "doc_tie", "doc_role_claim", "doc_theme",
	"doc_body_digest", "doc_subtree", "doc_embedding_status", "doc_embedding_omitted",
	"agent_source", "agent_source_digest", "agent_source_topic", "agent_source_scope",
	"repo_era", "doc_generation", "doc_burst", "doc_evolved_into", "doc_superseded", "doc_live",
	"origin_source", "vision_source", "orient_read_candidate", "orient_read_omitted",
	"agent_source_duplicate", "agent_source_winner", "agent_source_loser",
	"orient_agent", "orient_agent_knowledge", "orient_research_topic",
	"orient_agent_topic", "orient_agent_description", "orient_agent_permission", "orient_agent_priority",
	"orient_agent_prompt",
	"oriented_head", "oriented_document", "orient_role_pending",
}

// Projection carries public judgments and measurements, not the isolated
// engine's policy or config_param rows, into the action kernel.
func Projection(e *Engine) (string, error) {
	src, err := policySource()
	if err != nil {
		return "", err
	}
	decls := map[string]string{}
	for _, m := range declarationPattern.FindAllStringSubmatch(src, -1) {
		decls[m[1]] = m[0]
	}
	var b strings.Builder
	b.WriteString("# Generated orientation projection.\n")
	for _, pred := range projectedPredicates {
		decl, ok := decls[pred]
		if !ok {
			return "", fmt.Errorf("orientation projection: missing Decl %s", pred)
		}
		b.WriteString(decl + "\n")
	}
	var rows []string
	for _, pred := range projectedPredicates {
		facts, err := e.Query(pred)
		if err != nil {
			return "", err
		}
		for _, fact := range facts {
			rows = append(rows, fact.String())
		}
	}
	sort.Strings(rows)
	for _, row := range rows {
		b.WriteString(row + "\n")
	}
	return b.String(), nil
}

func projectionFacts(e *Engine) ([]types.Fact, error) {
	var out []types.Fact
	for _, predicate := range projectedPredicates {
		rows, err := e.Query(predicate)
		if err != nil {
			return nil, err
		}
		out = append(out, rows...)
	}
	return out, nil
}

func (s *Snapshot) Save(root string, e *Engine) error {
	projection, err := Projection(e)
	if err != nil {
		return err
	}
	body, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(snapshotPath(root))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := atomicfile.WriteFile(filepath.Join(dir, "orientation.mg"), []byte(projection), 0o644); err != nil {
		return err
	}
	return atomicfile.WriteFile(snapshotPath(root), body, 0o644)
}

type RefreshResult struct {
	Stale     []types.Fact
	Changed   []types.Fact
	Pending   []types.Fact
	Refreshed bool
	Added     []types.Fact
	Removed   []types.Fact
}

// Apply publishes only the changed public projection in one kernel transaction.
// Boot loads the file; a running incremental scanner uses this transaction.
func (r *RefreshResult) Apply(kernel types.KernelTransactor) error {
	if r == nil || !r.Refreshed {
		return nil
	}
	if kernel == nil {
		return fmt.Errorf("orientation refresh requires a kernel transactor")
	}
	tx := kernel.Transaction()
	if tx == nil {
		return fmt.Errorf("orientation refresh received a nil transaction")
	}
	for _, row := range r.Removed {
		tx.RetractExactFact(row)
	}
	for _, row := range r.Added {
		tx.Assert(row)
	}
	return tx.Commit()
}

// Refresh observes HEAD and tracked document contents, asks Mangle what is
// stale, and replaces affected measurements. It never opens answers.json.
// An incremental scan can call this same API before consuming orientation.
func Refresh(ctx context.Context, root string, emb embedding.EmbeddingEngine) (*RefreshResult, error) {
	snapshotMu.Lock()
	defer snapshotMu.Unlock()
	s, err := LoadSnapshot(root)
	if err != nil {
		return nil, err
	}
	head, err := Head(ctx, root)
	if err != nil {
		return nil, err
	}
	paths, err := trackedDocs(ctx, root)
	if err != nil {
		return nil, err
	}
	bodies := map[string]string{}
	observed := []types.Fact{{Predicate: "current_repo_head", Args: []any{head}}}
	for _, p := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
		if err != nil {
			return nil, fmt.Errorf("observe tracked document %s: %w", p, err)
		}
		bodies[p] = string(body)
		observed = append(observed, types.Fact{Predicate: "current_document", Args: []any{p, documentDigest(string(body))}})
	}
	e, err := s.Engine(ctx)
	if err != nil {
		return nil, err
	}
	defer e.Close()
	before, err := projectionFacts(e)
	if err != nil {
		return nil, err
	}
	if err := e.Assert(observed); err != nil {
		return nil, err
	}
	if err := e.Evaluate(ctx); err != nil {
		return nil, err
	}
	out := &RefreshResult{}
	if out.Stale, err = e.Query("orient_stale"); err != nil {
		return nil, err
	}
	if out.Changed, err = e.Query("orient_refresh_document"); err != nil {
		return nil, err
	}
	if len(out.Stale) == 0 {
		out.Pending, err = e.Query("orient_role_pending")
		return out, err
	}
	changed := map[string]bool{}
	for _, row := range out.Changed {
		changed[types.ExtractString(row.Args[0])] = true
	}
	var historyRows []types.Fact
	for _, row := range s.Facts {
		if strings.HasPrefix(row.Predicate, "repo_") {
			historyRows = append(historyRows, row)
		}
	}
	if head != s.Head {
		ancestor := gitCmd(ctx, root, "-c", "safe.directory=*", "merge-base", "--is-ancestor", s.Head, head).Run() == nil
		diff, diffErr := gitCmd(ctx, root, "-c", "safe.directory=*", "diff", "--name-status", "--find-renames", s.Head, head).Output()
		renames := strings.HasPrefix(string(diff), "R") || strings.Contains(string(diff), "\nR")
		if s.Head == "" || head == "" || !ancestor || diffErr != nil || renames || !hasDayWitness(historyRows) {
			h, err := ScanHistory(ctx, root)
			if err != nil {
				return nil, err
			}
			historyRows = h.Facts
			s.Notes = append(s.Notes, "History remeasured: rewritten, renamed, unborn or pre-delta snapshot.")
		} else {
			shallow, err := isShallow(ctx, root)
			if err != nil {
				return nil, err
			}
			delta, err := historyDelta(ctx, root, s.Head, head, shallow)
			if err != nil {
				return nil, err
			}
			historyRows = mergeHistory(historyRows, delta)
		}
	}
	var kept []types.Fact
	for _, row := range s.Facts {
		if strings.HasPrefix(row.Predicate, "repo_") || row.Predicate == "oriented_head" || row.Predicate == "oriented_document" || row.Predicate == "doc_tie" {
			continue
		}
		if (row.Predicate == "doc_role_claim" || row.Predicate == "doc_theme" || row.Predicate == "orient_role_pending") && changed[types.ExtractString(row.Args[0])] {
			continue
		}
		if len(changed) > 0 && documentMeasurement(row.Predicate) {
			continue
		}
		kept = append(kept, row)
	}
	if len(changed) > 0 {
		sources, err := Discover(root)
		if err != nil {
			return nil, fmt.Errorf("refresh agent sources: %w", err)
		}
		s.Sources = sources
		var retained []types.Fact
		for _, row := range kept {
			if !strings.HasPrefix(row.Predicate, "agent_source") {
				retained = append(retained, row)
			}
		}
		kept = append(retained, Facts(sources)...)
		// Bodies are already observed; unchanged vectors hit the content cache.
		scan := &DocScan{}
		for p, body := range bodies {
			dir := filepath.ToSlash(filepath.Dir(p))
			if dir == "." {
				dir = ""
			}
			scan.Facts = append(scan.Facts, types.Fact{Predicate: "doc_file", Args: []any{p, dir, int64(len(body)), int64(headingCount(body))}})
		}
		scan, err = collectDocBodies(ctx, root, s.Config, emb, paths, bodies, scan)
		if err != nil {
			return nil, err
		}
		kept = append(kept, scan.Facts...)
		if emb == nil {
			var pairs []simPair
			for _, row := range s.Facts {
				if (row.Predicate == "doc_embedding_status" || row.Predicate == "doc_embedding_omitted") && !changed[types.ExtractString(row.Args[0])] {
					kept = append(kept, row)
				}
				if row.Predicate != "doc_similar" {
					continue
				}
				a, b := types.ExtractString(row.Args[0]), types.ExtractString(row.Args[1])
				if changed[a] || changed[b] {
					continue
				}
				kept = append(kept, row)
				pairs = append(pairs, simPair{a: a, b: b, permille: int(number(row.Args[2]))})
			}
			for p, id := range clusterIDs(pairs) {
				kept = append(kept, types.Fact{Predicate: "doc_cluster", Args: []any{p, id}})
			}
			for p := range changed {
				if _, ok := bodies[p]; ok {
					kept = append(kept, types.Fact{Predicate: "doc_embedding_omitted", Args: []any{p, types.MangleAtom("/unavailable"), "Refresh has no embedding client; affected similarity witnesses were invalidated."}})
				}
			}
		}
		s.Notes = append(s.Notes, scan.SimilarityNote)
		for p := range changed {
			if _, ok := bodies[p]; ok {
				kept = append(kept, types.Fact{Predicate: "orient_role_pending", Args: []any{p}})
			}
		}
	}
	s.Head, s.Bodies = head, bodies
	s.Facts = append(kept, historyRows...)
	s.Facts = append(s.Facts, types.Fact{Predicate: "oriented_head", Args: []any{head}})
	for p, body := range bodies {
		s.Facts = append(s.Facts, types.Fact{Predicate: "oriented_document", Args: []any{p, documentDigest(body)}})
	}
	s.Facts = append(s.Facts, DocTieFacts(tiePaths(s.Facts))...)
	if after, err := Head(ctx, root); err != nil || after != head {
		return nil, fmt.Errorf("HEAD changed during orientation refresh")
	}
	fresh, err := s.Engine(ctx)
	if err != nil {
		return nil, err
	}
	defer fresh.Close()
	if out.Pending, err = fresh.Query("orient_role_pending"); err != nil {
		return nil, err
	}
	after, err := projectionFacts(fresh)
	if err != nil {
		return nil, err
	}
	oldRows, newRows := map[string]types.Fact{}, map[string]types.Fact{}
	for _, row := range before {
		oldRows[row.String()] = row
	}
	for _, row := range after {
		newRows[row.String()] = row
	}
	for key, row := range oldRows {
		if _, exists := newRows[key]; !exists {
			out.Removed = append(out.Removed, row)
		}
	}
	for key, row := range newRows {
		if _, exists := oldRows[key]; !exists {
			out.Added = append(out.Added, row)
		}
	}
	if err := s.Save(root, fresh); err != nil {
		return nil, err
	}
	report, err := buildReport(fresh)
	if err != nil {
		return nil, err
	}
	report.Workspace = root
	report.HistoryNote = "Orientation refreshed from the recorded head; changed document roles require transduction. The canonical north star remains in .nerd/northstar.json."
	for _, row := range out.Pending {
		report.HistoryNote += " Pending: " + types.ExtractString(row.Args[0]) + "."
	}
	if err := atomicfile.WriteFile(filepath.Join(root, ".nerd", "orientation", "README.md"), []byte(report.Text()), 0o644); err != nil {
		return nil, err
	}
	out.Refreshed = true
	return out, nil
}

func documentMeasurement(predicate string) bool {
	switch predicate {
	case "doc_file", "doc_link", "doc_similar", "doc_cluster", "doc_body_digest", "doc_subtree", "doc_embedding_status", "doc_embedding_omitted":
		return true
	}
	return false
}

func hasDayWitness(rows []types.Fact) bool {
	for _, row := range rows {
		if row.Predicate == "repo_file_day" {
			return true
		}
	}
	return false
}
