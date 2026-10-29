package orient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"codenerd/internal/atomicfile"
	"codenerd/internal/config"
	"codenerd/internal/types"
	"codenerd/internal/workspace"
)

// TreatmentRefresh is what one Refresh did to .nerd/orientation/membership.json.
type TreatmentRefresh struct {
	// Written is true when the census ran and the artifact was replaced;
	// false when the recorded generation and policy were already current.
	Written     bool
	Generation  string
	Rows        int
	Conflicts   int
	HeaderReads int
}

// treatmentMu serializes treatment refreshes in this process. Two refreshes
// of one workspace would census the same generation and race on the write.
var treatmentMu sync.Mutex

// refreshTreatment brings the treatment artifact to the current membership
// generation and treatment policy. It reads the live orient section: an
// operator override takes effect at the next refresh, not at the next
// re-orientation. An unchanged generation and policy cost one member-list
// digest and one artifact read.
func refreshTreatment(ctx context.Context, root string) (TreatmentRefresh, error) {
	treatmentMu.Lock()
	defer treatmentMu.Unlock()
	root, err := absPath(root)
	if err != nil {
		return TreatmentRefresh{}, err
	}
	cfg, err := config.LoadOrientConfig(root)
	if err != nil {
		return TreatmentRefresh{}, fmt.Errorf("treatment configuration: %w", err)
	}
	m, err := workspace.For(root)
	if err != nil {
		return TreatmentRefresh{}, fmt.Errorf("treatment membership: %w", err)
	}
	policy := cfg.TreatmentPolicy()
	files := m.Files()
	generation := workspace.GenerationOf(files)
	policyDigest, err := treatmentPolicyDigest(policy)
	if err != nil {
		return TreatmentRefresh{}, err
	}
	if prev, err := workspace.ReadTreatmentArtifact(m.Root()); err == nil &&
		prev.SchemaVersion == workspace.TreatmentArtifactSchema && prev.Workspace == m.Root() &&
		prev.Generation == generation && prev.PolicyDigest == policyDigest {
		return TreatmentRefresh{Generation: generation, Rows: len(prev.Treatments), Conflicts: len(prev.Conflicts), HeaderReads: prev.Census.HeaderReads}, nil
	}
	census, err := measureTreatmentCensus(ctx, m.Root(), files, policy)
	if err != nil {
		return TreatmentRefresh{}, err
	}
	art, err := deriveTreatment(ctx, cfg, census)
	if err != nil {
		return TreatmentRefresh{}, err
	}
	art.Workspace, art.Generation, art.PolicyDigest = m.Root(), generation, policyDigest
	body, err := art.Encode()
	if err != nil {
		return TreatmentRefresh{}, err
	}
	target := workspace.TreatmentArtifactPath(m.Root())
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return TreatmentRefresh{}, err
	}
	if err := atomicfile.WriteFile(target, body, 0o644); err != nil {
		return TreatmentRefresh{}, err
	}
	return TreatmentRefresh{Written: true, Generation: generation, Rows: len(art.Treatments), Conflicts: len(art.Conflicts), HeaderReads: art.Census.HeaderReads}, nil
}

// treatmentPolicyDigest names the rules and the configuration they ran with,
// so an edited override or threshold re-derives the rows.
func treatmentPolicyDigest(policy config.OrientTreatmentConfig) (string, error) {
	rules, err := policyFiles.ReadFile("treatment.mg")
	if err != nil {
		return "", fmt.Errorf("orient treatment rules: %w", err)
	}
	policy.Overrides = policy.SortedOverrides()
	cfg, err := json.Marshal(policy)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	h.Write(rules)
	h.Write([]byte{0})
	h.Write(cfg)
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// treatmentCensus is the measured EDB plus what measuring it cost.
type treatmentCensus struct {
	facts   []types.Fact
	summary workspace.TreatmentCensus
	digest  string
	policy  config.OrientTreatmentConfig
}

type censusDir struct {
	// [start, end) is the directory's subtree in the sorted member list;
	// a subtree is contiguous in byte order.
	start, end int
	files      int64
	bytes      int64
	exts       map[string]int64
}

// measureTreatmentCensus describes every member directory down to the
// census depth, plus each overridden member directory and its ancestors.
// files is the frozen member list (Membership.Files), so the census and the
// generation digest cover the same set. Bodies are not read except for the
// counted, budgeted header samples.
func measureTreatmentCensus(ctx context.Context, root string, files []string, policy config.OrientTreatmentConfig) (*treatmentCensus, error) {
	if !sort.StringsAreSorted(files) {
		files = append([]string(nil), files...)
		sort.Strings(files)
	}
	memberDirs := map[string]bool{}
	for _, f := range files {
		for d := treeParent(f); d != "" && !memberDirs[d]; d = treeParent(d) {
			memberDirs[d] = true
		}
	}
	dirs := map[string]*censusDir{}
	add := func(d string) {
		if dirs[d] == nil {
			dirs[d] = &censusDir{start: -1, exts: map[string]int64{}}
		}
	}
	for d := range memberDirs {
		if treeDepth(d) <= policy.CensusMaxDepth {
			add(d)
		}
	}
	for _, o := range policy.Overrides {
		if !memberDirs[o.Path] {
			continue
		}
		for d := o.Path; d != ""; d = treeParent(d) {
			add(d)
		}
	}
	out := &treatmentCensus{policy: policy}
	out.summary.MaxDepth = policy.CensusMaxDepth
	out.summary.HeaderReadBudget = policy.HeaderReadBudget
	for i, f := range files {
		if i%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(f)))
		if errors.Is(err, fs.ErrNotExist) {
			continue // removed since the member list was taken
		}
		if err != nil {
			return nil, fmt.Errorf("treatment census %s: %w", f, err)
		}
		var size int64
		if info.Mode().IsRegular() {
			size = info.Size()
		}
		ext := strings.ToLower(path.Ext(f))
		out.summary.Files++
		out.summary.Bytes += size
		for d := treeParent(f); d != ""; d = treeParent(d) {
			c := dirs[d]
			if c == nil {
				continue
			}
			if c.start < 0 {
				c.start = i
			}
			c.end = i + 1
			c.files++
			c.bytes += size
			if ext != "" {
				c.exts[ext]++
			}
		}
	}
	ordered := make([]string, 0, len(dirs))
	for d, c := range dirs {
		if c.start >= 0 {
			ordered = append(ordered, d)
		}
	}
	// Shallow directories first, so a spent budget leaves the deepest
	// directories unsampled rather than whole top-level trees.
	sort.Slice(ordered, func(i, j int) bool {
		di, dj := treeDepth(ordered[i]), treeDepth(ordered[j])
		if di != dj {
			return di < dj
		}
		return ordered[i] < ordered[j]
	})
	out.summary.Directories = len(ordered)
	budget := policy.HeaderReadBudget
	sampled := map[int]int{}
	for _, d := range ordered {
		c := dirs[d]
		depth := int64(treeDepth(d))
		out.facts = append(out.facts,
			types.Fact{Predicate: "tree_dir", Args: []any{d}},
			types.Fact{Predicate: "tree_depth", Args: []any{d, depth}},
			types.Fact{Predicate: "tree_name", Args: []any{d, path.Base(d)}},
			types.Fact{Predicate: "tree_stats", Args: []any{d, c.files, c.bytes}},
		)
		if p := treeParent(d); p != "" {
			out.facts = append(out.facts, types.Fact{Predicate: "tree_parent", Args: []any{d, p}})
		}
		for ext, n := range c.exts {
			out.facts = append(out.facts, types.Fact{Predicate: "tree_ext", Args: []any{d, ext, n}})
		}
		n := c.end - c.start
		k := min(policy.HeaderSamplesPerDir, n)
		var read, generated int64
		short := false
		for j := 0; j < k; j++ {
			idx := c.start + j*n/k
			marker, ok := sampled[idx]
			if !ok {
				if budget == 0 {
					short = true
					continue
				}
				budget--
				out.summary.HeaderReads++
				marker = headerMarker(root, files[idx], policy.HeaderBytes)
				sampled[idx] = marker
			}
			if marker < 0 {
				continue
			}
			read++
			if marker > 0 {
				generated++
			}
		}
		if short {
			out.summary.UnsampledDirectories++
		}
		out.facts = append(out.facts, types.Fact{Predicate: "tree_header_sample", Args: []any{d, read, generated}})
	}
	lines := make([]string, len(out.facts))
	for i, f := range out.facts {
		lines[i] = f.String()
	}
	sort.Strings(lines)
	h := sha256.New()
	for _, l := range lines {
		h.Write([]byte(l))
		h.Write([]byte{'\n'})
	}
	out.digest = "sha256:" + hex.EncodeToString(h.Sum(nil))
	return out, nil
}

var (
	goGeneratedMarker    = regexp.MustCompile(`(?m)^// Code generated .* DO NOT EDIT\.\r?$`)
	generatedByDoNotEdit = regexp.MustCompile(`(?mi)^\W{0,4}generated by .*do not edit`)
)

// headerMarker reads at most limit bytes of one member file: 1 when the
// header carries a generated-code marker, 0 when it does not, -1 when the
// file could not be read (it is then no sample).
func headerMarker(root, rel string, limit int) int {
	f, err := os.Open(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return -1
	}
	defer f.Close()
	head, err := io.ReadAll(io.LimitReader(f, int64(limit)))
	if err != nil {
		return -1
	}
	text := string(head)
	if goGeneratedMarker.MatchString(text) || generatedByDoNotEdit.MatchString(text) ||
		strings.Contains(text, "@generated") || strings.Contains(strings.ToLower(text), "<auto-generated") {
		return 1
	}
	return 0
}

func treeParent(p string) string {
	i := strings.LastIndexByte(p, '/')
	if i < 0 {
		return ""
	}
	return p[:i]
}

func treeDepth(d string) int { return strings.Count(d, "/") + 1 }

// overrideFacts are the operator's overrides as policy rows.
func overrideFacts(policy config.OrientTreatmentConfig) []types.Fact {
	out := make([]types.Fact, 0, len(policy.Overrides))
	for _, o := range policy.Overrides {
		out = append(out, types.Fact{Predicate: "treatment_override", Args: []any{
			o.Path, types.MangleAtom("/" + o.Treatment), types.MangleAtom("/" + o.Basis),
		}})
	}
	return out
}

// deriveTreatment evaluates treatment.mg over the census and the overrides
// and shapes the derived rows into the artifact. The program is schema.mg
// plus treatment.mg alone: treatment needs none of the history or document
// census, and loading it would cost the whole orientation fixpoint.
func deriveTreatment(ctx context.Context, cfg config.OrientConfig, census *treatmentCensus) (*workspace.TreatmentArtifact, error) {
	e, err := isolatedEngine("schema.mg", "treatment.mg")
	if err != nil {
		return nil, err
	}
	defer e.Close()
	if err := e.Assert(config.ParamFacts(engineParams(cfg))); err != nil {
		return nil, err
	}
	if err := e.Assert(census.facts); err != nil {
		return nil, err
	}
	if err := e.Assert(overrideFacts(census.policy)); err != nil {
		return nil, err
	}
	if err := e.Evaluate(ctx); err != nil {
		return nil, err
	}
	if missing, err := e.Query("config_param_missing"); err != nil {
		return nil, err
	} else if len(missing) > 0 {
		return nil, fmt.Errorf("treatment policy is missing %d config_param rows", len(missing))
	}
	art := &workspace.TreatmentArtifact{
		SchemaVersion:     workspace.TreatmentArtifactSchema,
		MeasurementDigest: census.digest,
		Census:            census.summary,
	}
	rows := map[string]*workspace.TreatmentRow{}
	treatments, err := e.Query("tree_treatment")
	if err != nil {
		return nil, err
	}
	for _, f := range treatments {
		dir, t, why := types.ExtractString(f.Args[0]), workspace.Treatment(types.ExtractString(f.Args[1])), types.ExtractString(f.Args[2])
		r := rows[dir]
		if r == nil {
			r = &workspace.TreatmentRow{Dir: dir, Treatment: t}
			rows[dir] = r
		} else if r.Treatment != t {
			return nil, fmt.Errorf("treatment policy derived %s and %s for %q", r.Treatment, t, dir)
		}
		r.Reasons = append(r.Reasons, why)
	}
	sources, err := e.Query("tree_treatment_source")
	if err != nil {
		return nil, err
	}
	for _, f := range sources {
		dir, scope, authority := types.ExtractString(f.Args[0]), types.ExtractString(f.Args[1]), types.ExtractString(f.Args[2])
		r := rows[dir]
		if r == nil {
			return nil, fmt.Errorf("treatment policy named a source for %q without a treatment", dir)
		}
		if r.Scope != "" && (r.Scope != scope || r.Authority != authority) {
			return nil, fmt.Errorf("treatment policy derived two governing scopes for %q", dir)
		}
		r.Scope, r.Authority = scope, authority
	}
	for _, r := range rows {
		if r.Scope == "" {
			return nil, fmt.Errorf("treatment policy derived %s for %q without a governing scope", r.Treatment, r.Dir)
		}
		art.Treatments = append(art.Treatments, *r)
	}
	conflicts, err := e.Query("tree_treatment_conflict")
	if err != nil {
		return nil, err
	}
	byDir := map[string]*workspace.TreatmentConflict{}
	for _, f := range conflicts {
		dir, scope, t := types.ExtractString(f.Args[0]), types.ExtractString(f.Args[1]), workspace.Treatment(types.ExtractString(f.Args[2]))
		if rows[dir] != nil {
			return nil, fmt.Errorf("treatment policy derived both a treatment and a conflict for %q", dir)
		}
		c := byDir[dir]
		if c == nil {
			c = &workspace.TreatmentConflict{Dir: dir, Scope: scope}
			byDir[dir] = c
		}
		c.Treatments = append(c.Treatments, t)
	}
	for _, c := range byDir {
		art.Conflicts = append(art.Conflicts, *c)
	}
	unmatched, err := e.Query("treatment_override_unmatched")
	if err != nil {
		return nil, err
	}
	for _, f := range unmatched {
		art.UnmatchedOverrides = append(art.UnmatchedOverrides, types.ExtractString(f.Args[0]))
	}
	return art, nil
}
