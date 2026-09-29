package orient

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"codenerd/internal/config"
	"codenerd/internal/embedding"
	"codenerd/internal/types"
)

// Report is the repository orientation view. Judgments come from the engine.
// Month labels are joined here for display; they are not a decision.
type Report struct {
	Workspace          string              `json:"workspace"`
	Shallow            bool                `json:"shallow"`
	Empty              bool                `json:"empty"`
	Commits            int                 `json:"commits"`
	Span               SpanView            `json:"span"`
	Eras               []EraView           `json:"eras"`
	Origins            []OriginView        `json:"origins"`
	Evolved            []EvolvedView       `json:"evolved"`
	Bursts             []string            `json:"bursts"`
	Cohorts            []string            `json:"cohorts"`
	ReadCandidates     []ReadView          `json:"read_candidates"`
	ReadOmitted        []ReadView          `json:"read_omitted"`
	ReadBudget         int64               `json:"read_budget"`
	Vision             []VisionView        `json:"vision"`
	VisionNote         string              `json:"vision_note"`
	SimilarityNote     string              `json:"similarity_note"`
	HistoryNote        string              `json:"history_note"`
	Unreadable         []string            `json:"unreadable"`
	CacheNote          string              `json:"cache_note,omitempty"`
	Embedded           int                 `json:"embedded"`
	EmbeddingCacheHits int                 `json:"embedding_cache_hits"`
	EmbeddingOmitted   []EmbeddingOmission `json:"embedding_omitted"`
	ReadMembers        []ReadMemberView    `json:"read_members"`
	CohortGroups       []CohortView        `json:"cohort_groups"`
}

// ReadMemberView exposes suppression separately from the read-budget cut.
type ReadMemberView struct {
	Path           string `json:"path"`
	Representative string `json:"representative"`
}

// CohortView is a policy-derived act, its representative and every member.
type CohortView struct {
	ID             string   `json:"id"`
	Representative string   `json:"representative"`
	Members        []string `json:"members"`
	Burst          bool     `json:"burst"`
}

// SpanView is repo_span.
type SpanView struct {
	FirstUnix int64 `json:"first_unix"`
	LastUnix  int64 `json:"last_unix"`
	Commits   int64 `json:"commits"`
	Shallow   bool  `json:"shallow"`
}

// EraView is one repo_era plus the month labels Go stored on repo_month.
type EraView struct {
	Index      int64  `json:"index"`
	StartMonth int64  `json:"start_month"`
	EndMonth   int64  `json:"end_month"`
	Kind       string `json:"kind"`
	StartLabel string `json:"start_label"`
	EndLabel   string `json:"end_label"`
}

// OriginView is one origin_source.
type OriginView struct {
	Path string `json:"path"`
	Why  string `json:"why"`
}

// EvolvedView is one doc_evolved_into edge. Superseded is true when the
// policy also derived doc_superseded for the same pair.
type EvolvedView struct {
	Old        string `json:"old"`
	New        string `json:"new"`
	Superseded bool   `json:"superseded"`
}

// ReadView is one document and the reasons the policy qualified it.
type ReadView struct {
	Path    string   `json:"path"`
	Reasons []string `json:"reasons"`
}

// VisionView is one vision_source. Empty until role claims are asserted.
type VisionView struct {
	Path      string `json:"path"`
	WeightPct int64  `json:"weight_pct"`
	Why       string `json:"why"`
}

// Inspect scans root, asserts the measurements and any extra rows (agent
// sources, role claims), and evaluates. extra's doc_tie rows are ignored:
// this function assigns the single ordinal space. emb may be nil; similarity
// is then skipped and the report says so.
//
// Init retains its engine between transduction and materialization; Inspect
// is the read-only library view over one measurement pass.
func Inspect(ctx context.Context, root string, cfg *config.OrientConfig, emb embedding.EmbeddingEngine, extra []types.Fact) (*Report, error) {
	if cfg == nil {
		return nil, fmt.Errorf("orient config is nil")
	}
	root, err := absPath(root)
	if err != nil {
		return nil, err
	}
	hist, err := ScanHistory(ctx, root)
	if err != nil {
		return nil, err
	}
	docs, err := CollectDocs(ctx, root, *cfg, emb)
	if err != nil {
		return nil, err
	}
	facts := make([]types.Fact, 0, len(hist.Facts)+len(docs.Facts)+len(extra))
	facts = append(facts, hist.Facts...)
	facts = append(facts, docs.Facts...)
	for _, f := range extra {
		if f.Predicate == "doc_tie" {
			continue
		}
		facts = append(facts, f)
	}
	facts = append(facts, DocTieFacts(tiePaths(facts))...)

	eng, err := NewEngine(cfg)
	if err != nil {
		return nil, err
	}
	defer eng.Close()
	if err := eng.Assert(facts); err != nil {
		return nil, err
	}
	if err := eng.Evaluate(ctx); err != nil {
		return nil, err
	}
	rep, err := buildReport(eng)
	if err != nil {
		return nil, err
	}
	rep.Workspace = root
	rep.Shallow = hist.Shallow
	rep.Empty = hist.Empty
	rep.Commits = hist.Commits
	rep.SimilarityNote = docs.SimilarityNote
	rep.Unreadable = docs.Unreadable
	if docs.CacheWriteError != "" {
		rep.CacheNote = docs.CacheWriteError
	}
	if hist.Shallow {
		rep.HistoryNote = "History is shallow (git rev-parse --is-shallow-repository). Eras, generations, bursts, cohorts, evolution and origin sources were not drawn, because first-commit times in a truncated clone are not the repository's."
	} else if hist.Empty {
		rep.HistoryNote = "The repository has no commits. There is no timeline to draw."
	}
	if len(rep.Vision) == 0 {
		rep.VisionNote = "none derivable (no doc_role_claim)"
	}
	return rep, nil
}

func buildReport(e *Engine) (*Report, error) {
	rep := &Report{
		Eras:             []EraView{},
		Origins:          []OriginView{},
		Evolved:          []EvolvedView{},
		Bursts:           []string{},
		Cohorts:          []string{},
		ReadCandidates:   []ReadView{},
		ReadOmitted:      []ReadView{},
		Vision:           []VisionView{},
		Unreadable:       []string{},
		EmbeddingOmitted: []EmbeddingOmission{},
		ReadMembers:      []ReadMemberView{},
		CohortGroups:     []CohortView{},
	}
	statuses, err := e.Query("doc_embedding_status")
	if err != nil {
		return nil, err
	}
	for _, f := range statuses {
		rep.Embedded++
		if types.ExtractString(f.Args[1]) == "/cache" {
			rep.EmbeddingCacheHits++
		}
	}
	omissions, err := e.Query("doc_embedding_omitted")
	if err != nil {
		return nil, err
	}
	for _, f := range omissions {
		rep.EmbeddingOmitted = append(rep.EmbeddingOmitted, EmbeddingOmission{types.ExtractString(f.Args[0]), types.ExtractString(f.Args[1]), types.ExtractString(f.Args[2])})
	}
	sort.Slice(rep.EmbeddingOmitted, func(i, j int) bool { return rep.EmbeddingOmitted[i].Path < rep.EmbeddingOmitted[j].Path })
	members, err := e.Query("orient_read_member")
	if err != nil {
		return nil, err
	}
	for _, f := range members {
		rep.ReadMembers = append(rep.ReadMembers, ReadMemberView{types.ExtractString(f.Args[0]), types.ExtractString(f.Args[1])})
	}
	sort.Slice(rep.ReadMembers, func(i, j int) bool { return rep.ReadMembers[i].Path < rep.ReadMembers[j].Path })
	if rep.CohortGroups, err = cohortViews(e); err != nil {
		return nil, err
	}
	span, err := e.Query("repo_span")
	if err != nil {
		return nil, err
	}
	if len(span) > 0 && len(span[0].Args) == 4 {
		rep.Span.FirstUnix, _ = types.ExtractInt64(span[0].Args[0])
		rep.Span.LastUnix, _ = types.ExtractInt64(span[0].Args[1])
		rep.Span.Commits, _ = types.ExtractInt64(span[0].Args[2])
		rep.Span.Shallow = types.ExtractString(span[0].Args[3]) == "/yes"
		rep.Shallow = rep.Span.Shallow
	}
	labels := map[int64]string{}
	months, err := e.Query("repo_month")
	if err != nil {
		return nil, err
	}
	for _, f := range months {
		if len(f.Args) < 2 {
			continue
		}
		idx, ok := types.ExtractInt64(f.Args[0])
		if !ok {
			continue
		}
		labels[idx] = types.ExtractString(f.Args[1])
	}
	eras, err := e.Query("repo_era")
	if err != nil {
		return nil, err
	}
	for _, f := range eras {
		if len(f.Args) != 4 {
			continue
		}
		idx, _ := types.ExtractInt64(f.Args[0])
		start, _ := types.ExtractInt64(f.Args[1])
		end, _ := types.ExtractInt64(f.Args[2])
		rep.Eras = append(rep.Eras, EraView{
			Index: idx, StartMonth: start, EndMonth: end,
			Kind:       types.ExtractString(f.Args[3]),
			StartLabel: labels[start], EndLabel: labels[end],
		})
	}
	sort.Slice(rep.Eras, func(i, j int) bool { return rep.Eras[i].Index < rep.Eras[j].Index })

	origins, err := e.Query("origin_source")
	if err != nil {
		return nil, err
	}
	for _, f := range origins {
		if len(f.Args) != 2 {
			continue
		}
		rep.Origins = append(rep.Origins, OriginView{
			Path: types.ExtractString(f.Args[0]),
			Why:  types.ExtractString(f.Args[1]),
		})
	}
	sort.Slice(rep.Origins, func(i, j int) bool { return rep.Origins[i].Path < rep.Origins[j].Path })

	superseded := map[string]bool{}
	sup, err := e.Query("doc_superseded")
	if err != nil {
		return nil, err
	}
	for _, f := range sup {
		if len(f.Args) == 2 {
			superseded[types.ExtractString(f.Args[0])+"\x00"+types.ExtractString(f.Args[1])] = true
		}
	}
	evolved, err := e.Query("doc_evolved_into")
	if err != nil {
		return nil, err
	}
	for _, f := range evolved {
		if len(f.Args) != 2 {
			continue
		}
		old, neu := types.ExtractString(f.Args[0]), types.ExtractString(f.Args[1])
		rep.Evolved = append(rep.Evolved, EvolvedView{
			Old: old, New: neu, Superseded: superseded[old+"\x00"+neu],
		})
	}
	sort.Slice(rep.Evolved, func(i, j int) bool {
		if rep.Evolved[i].Old != rep.Evolved[j].Old {
			return rep.Evolved[i].Old < rep.Evolved[j].Old
		}
		return rep.Evolved[i].New < rep.Evolved[j].New
	})

	if rep.Bursts, err = onePath(e, "doc_burst"); err != nil {
		return nil, err
	}
	if rep.Cohorts, err = onePath(e, "doc_cohort"); err != nil {
		return nil, err
	}
	if rep.ReadCandidates, err = readViews(e, "orient_read_candidate"); err != nil {
		return nil, err
	}
	if rep.ReadOmitted, err = readViews(e, "orient_read_omitted"); err != nil {
		return nil, err
	}
	params, err := e.Query(config.ConfigParamPredicate)
	if err != nil {
		return nil, err
	}
	for _, f := range params {
		if len(f.Args) == 2 && types.ExtractString(f.Args[0]) == "/orient_read_candidate_budget" {
			rep.ReadBudget, _ = types.ExtractInt64(f.Args[1])
		}
	}
	vision, err := e.Query("vision_source")
	if err != nil {
		return nil, err
	}
	for _, f := range vision {
		if len(f.Args) != 3 {
			continue
		}
		w, _ := types.ExtractInt64(f.Args[1])
		rep.Vision = append(rep.Vision, VisionView{
			Path: types.ExtractString(f.Args[0]), WeightPct: w, Why: types.ExtractString(f.Args[2]),
		})
	}
	sort.Slice(rep.Vision, func(i, j int) bool {
		if rep.Vision[i].WeightPct != rep.Vision[j].WeightPct {
			return rep.Vision[i].WeightPct > rep.Vision[j].WeightPct
		}
		return rep.Vision[i].Path < rep.Vision[j].Path
	})
	return rep, nil
}

func onePath(e *Engine, pred string) ([]string, error) {
	rows, err := e.Query(pred)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, f := range rows {
		if len(f.Args) >= 1 {
			out = append(out, types.ExtractString(f.Args[0]))
		}
	}
	sort.Strings(out)
	return out, nil
}

func cohortViews(e *Engine) ([]CohortView, error) {
	by := map[string]*CohortView{}
	members, err := e.Query("cohort_member")
	if err != nil {
		return nil, err
	}
	for _, f := range members {
		p, root := types.ExtractString(f.Args[0]), types.ExtractString(f.Args[1])
		if by[root] == nil {
			by[root] = &CohortView{ID: root}
		}
		by[root].Members = append(by[root].Members, p)
	}
	representatives, err := e.Query("cohort_rep")
	if err != nil {
		return nil, err
	}
	for _, f := range representatives {
		if view := by[types.ExtractString(f.Args[0])]; view != nil {
			view.Representative = types.ExtractString(f.Args[1])
		}
	}
	bursts, err := e.Query("cohort_burst")
	if err != nil {
		return nil, err
	}
	for _, f := range bursts {
		if view := by[types.ExtractString(f.Args[0])]; view != nil {
			view.Burst = true
		}
	}
	out := make([]CohortView, 0, len(by))
	for _, view := range by {
		sort.Strings(view.Members)
		out = append(out, *view)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func readViews(e *Engine, pred string) ([]ReadView, error) {
	rows, err := e.Query(pred)
	if err != nil {
		return nil, err
	}
	by := map[string]map[string]struct{}{}
	var order []string
	for _, f := range rows {
		if len(f.Args) != 2 {
			continue
		}
		p := types.ExtractString(f.Args[0])
		if _, ok := by[p]; !ok {
			by[p] = map[string]struct{}{}
			order = append(order, p)
		}
		by[p][types.ExtractString(f.Args[1])] = struct{}{}
	}
	sort.Strings(order)
	out := make([]ReadView, 0, len(order))
	for _, p := range order {
		reasons := make([]string, 0, len(by[p]))
		for r := range by[p] {
			reasons = append(reasons, r)
		}
		sort.Strings(reasons)
		out = append(out, ReadView{Path: p, Reasons: reasons})
	}
	return out, nil
}

// JSON is the report, indented. Every omitted candidate and every
// unreadable path is included; nothing is trimmed to fit a screen.
func (r *Report) JSON() ([]byte, error) {
	if r.Unreadable == nil {
		r.Unreadable = []string{}
	}
	if r.Bursts == nil {
		r.Bursts = []string{}
	}
	if r.Cohorts == nil {
		r.Cohorts = []string{}
	}
	return json.MarshalIndent(r, "", "  ")
}

// Text is the human inspection. The read budget says how many qualifying
// documents were left out and names them.
func (r *Report) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "workspace: %s\n", r.Workspace)
	if r.SimilarityNote != "" {
		fmt.Fprintf(&b, "%s\n", r.SimilarityNote)
	}
	if len(r.EmbeddingOmitted) > 0 {
		fmt.Fprintf(&b, "similarity exclusions: %d (retained %d, cache hits %d)\n", len(r.EmbeddingOmitted), r.Embedded, r.EmbeddingCacheHits)
		for _, omitted := range r.EmbeddingOmitted {
			fmt.Fprintf(&b, "  %s %s: %s\n", omitted.Path, omitted.Kind, omitted.Detail)
		}
	}
	if r.HistoryNote != "" {
		fmt.Fprintf(&b, "%s\n", r.HistoryNote)
	}
	fmt.Fprintf(&b, "span: first %d last %d commits %d shallow %t\n",
		r.Span.FirstUnix, r.Span.LastUnix, r.Span.Commits, r.Span.Shallow)
	fmt.Fprintf(&b, "eras: %d\n", len(r.Eras))
	for _, e := range r.Eras {
		fmt.Fprintf(&b, "  %d %s %s..%s (months %d..%d)\n",
			e.Index, e.Kind, e.StartLabel, e.EndLabel, e.StartMonth, e.EndMonth)
	}
	fmt.Fprintf(&b, "origin sources: %d\n", len(r.Origins))
	for _, o := range r.Origins {
		fmt.Fprintf(&b, "  %s %s\n", o.Path, o.Why)
	}
	fmt.Fprintf(&b, "evolution: %d\n", len(r.Evolved))
	for _, e := range r.Evolved {
		mark := ""
		if e.Superseded {
			mark = " superseded"
		}
		fmt.Fprintf(&b, "  %s -> %s%s\n", e.Old, e.New, mark)
	}
	fmt.Fprintf(&b, "burst documents: %d\n", len(r.Bursts))
	for _, p := range r.Bursts {
		fmt.Fprintf(&b, "  %s\n", p)
	}
	fmt.Fprintf(&b, "cohort documents: %d\n", len(r.Cohorts))
	for _, p := range r.Cohorts {
		fmt.Fprintf(&b, "  %s\n", p)
	}
	for _, group := range r.CohortGroups {
		fmt.Fprintf(&b, "  cohort %s representative %s burst %t members %s\n", group.ID, group.Representative, group.Burst, strings.Join(group.Members, ","))
	}
	qualifying := len(r.ReadCandidates) + len(r.ReadOmitted)
	fmt.Fprintf(&b, "read candidates (budget %d): %d kept of %d qualifying, %d omitted\n",
		r.ReadBudget, len(r.ReadCandidates), qualifying, len(r.ReadOmitted))
	if len(r.ReadOmitted) > 0 {
		fmt.Fprintf(&b, "omitted documents are listed; raising orient.read_candidate_budget admits more representatives; unit members share their representative's slot\n")
	}
	for _, c := range r.ReadCandidates {
		fmt.Fprintf(&b, "  keep %s %s\n", c.Path, strings.Join(c.Reasons, ","))
	}
	for _, c := range r.ReadOmitted {
		fmt.Fprintf(&b, "  omit %s %s\n", c.Path, strings.Join(c.Reasons, ","))
	}
	for _, member := range r.ReadMembers {
		fmt.Fprintf(&b, "  member %s representative %s\n", member.Path, member.Representative)
	}
	if len(r.Vision) == 0 {
		fmt.Fprintf(&b, "vision sources: %s\n", r.VisionNote)
	} else {
		fmt.Fprintf(&b, "vision sources: %d\n", len(r.Vision))
		for _, v := range r.Vision {
			fmt.Fprintf(&b, "  %s %d %s\n", v.Path, v.WeightPct, v.Why)
		}
	}
	if len(r.Unreadable) > 0 {
		fmt.Fprintf(&b, "unreadable documents (not asserted as doc_file): %d\n", len(r.Unreadable))
		for _, p := range r.Unreadable {
			fmt.Fprintf(&b, "  %s\n", p)
		}
	}
	return b.String()
}
