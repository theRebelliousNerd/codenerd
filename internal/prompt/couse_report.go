package prompt

import (
	"sort"
)

// Analysis defaults. They are parameters rather than constants because the
// right thresholds depend on how many turns have been observed, and a reader
// comparing two runs needs to know the report says which values produced it.
const (
	// DefaultMinSupport is the smallest number of successful selections a pair
	// must appear in before its lift is reported. Lift over two or three
	// selections is noise wearing a decimal point.
	DefaultMinSupport = 5

	// DefaultMinLift is the edge threshold for clustering. 1.0 is independence,
	// so anything at or below it is evidence *against* an association.
	DefaultMinLift = 1.5

	// DefaultUbiquityThreshold is the share of successful selections above which
	// an atom is treated as skeleton rather than signal.
	DefaultUbiquityThreshold = 0.95
)

// CoUseParams are the thresholds an analysis ran with.
type CoUseParams struct {
	MinSupport        int     `json:"min_support"`
	MinLift           float64 `json:"min_lift"`
	UbiquityThreshold float64 `json:"ubiquity_threshold"`
}

// DefaultCoUseParams returns the documented defaults.
func DefaultCoUseParams() CoUseParams {
	return CoUseParams{
		MinSupport:        DefaultMinSupport,
		MinLift:           DefaultMinLift,
		UbiquityThreshold: DefaultUbiquityThreshold,
	}
}

// AtomStat is one atom's marginal frequency.
type AtomStat struct {
	ID       string  `json:"id"`
	Category string  `json:"category,omitempty"`
	Success  int     `json:"success_selections"`
	Failure  int     `json:"failure_selections"`
	Share    float64 `json:"share"` // fraction of successful selections containing it
	// Ubiquitous marks an atom present in nearly every selection -- a skeleton atom
	// rather than a selected one. Lift already handles these correctly: an atom
	// in every selection scores a lift of almost exactly 1 against everything, which
	// is independence, so it never forms an edge. The explicit exclusion is a
	// second line of defence for small samples and for anyone who lowers
	// MinLift, and it keeps skeleton atoms out of a readout where they would be
	// the loudest and least informative rows.
	Ubiquitous bool `json:"ubiquitous"`
}

// PairStat is one atom pair's association.
type PairStat struct {
	A string `json:"a"`
	B string `json:"b"`
	// Joint is the number of successful selections containing both.
	Joint int `json:"joint"`
	// Lift is P(A,B) / (P(A)P(B)). Above 1 means the two appear together more
	// often than independent selection would produce; 1 means independence,
	// which is exactly what a ubiquitous atom scores against everything and
	// why raw co-occurrence counts are the wrong statistic here.
	Lift float64 `json:"lift"`
	// Jaccard is |A and B| / |A or B| -- how much of their combined use is
	// shared. Lift says whether the association is real; Jaccard says whether
	// it is large. A pair can have huge lift and tiny Jaccard if both atoms are
	// rare, and grouping on that is how a taxonomy gets built out of noise.
	Jaccard float64 `json:"jaccard"`
	// SameCategory records whether both atoms carry the same category, which is
	// what the alignment verdict is computed from.
	SameCategory bool `json:"same_category"`
}

// Cluster is a connected component of the co-use graph.
type Cluster struct {
	Atoms []string `json:"atoms"`
	// ModalCategory is the most common category among the cluster's atoms, and
	// Purity the share of atoms carrying it. Purity is the number that answers
	// Q2: a cluster of mixed categories means information does not cluster the
	// way the taxonomy says it does.
	ModalCategory string  `json:"modal_category"`
	Purity        float64 `json:"purity"`
}

// CoUseReport is the Q2 answer.
type CoUseReport struct {
	Params CoUseParams `json:"params"`

	// SuccessSelections and FailureSelections count prompt compilations, not
	// turns: one turn can compile several prompts, and each is an independent
	// observation of what gets selected together.
	SuccessSelections int `json:"success_selections"`
	FailureSelections int `json:"failure_selections"`
	// PendingTurns and DroppedSelections describe evidence not in this report:
	// turns still awaiting an outcome, and selections evicted before one
	// arrived. A report that does not disclose its own gaps invites its sample
	// size to be read as larger than it is.
	PendingTurns      int `json:"pending_turns"`
	DroppedSelections int `json:"dropped_selections"`
	// UnattributedSelections counts compilations observed with no turn
	// identity. They can never be settled, so they are not evidence -- but a
	// large number means a whole class of compilations is invisible here, which
	// is a wiring gap rather than an absence of clustering.
	UnattributedSelections int `json:"unattributed_selections"`

	DistinctAtoms int `json:"distinct_atoms"`
	// Truncated means the pair matrix hit its cap and stopped admitting new
	// pairs. Associations among late-appearing atoms are missing.
	Truncated bool `json:"truncated"`

	Atoms      []AtomStat `json:"atoms"`
	Ubiquitous []string   `json:"ubiquitous"`
	Pairs      []PairStat `json:"pairs"`
	Clusters   []Cluster  `json:"clusters"`

	// CategoryAlignment is the atom-weighted mean cluster purity: the share of
	// clustered atoms sitting in a cluster whose modal category is their own.
	// High means the existing category taxonomy already describes how atoms
	// are used together. Low means it does not, and a lane taxonomy copied
	// from an org chart would describe it even worse.
	CategoryAlignment float64 `json:"category_alignment"`
	// CrossCategoryPairs is how many reported pairs join two different
	// categories. These are the concrete counterexamples to the taxonomy, and
	// the most useful thing in the report to read by hand.
	CrossCategoryPairs int `json:"cross_category_pairs"`
	// ClusteredAtoms counts atoms that landed in a cluster of two or more.
	ClusteredAtoms int `json:"clustered_atoms"`
	// IsolatedAtoms counts non-ubiquitous atoms with no qualifying edge. A high
	// count means atom selection is close to independent, which would make any
	// lane taxonomy -- this one or a measured replacement -- unfounded.
	IsolatedAtoms int `json:"isolated_atoms"`
}

// CategoryLookup maps an atom id to its category. Passing it in keeps the
// recorder decoupled from the corpus: the recorder only ever sees ids.
type CategoryLookup func(atomID string) string

// Report computes the co-use analysis. A nil lookup means categories are
// unknown, in which case alignment is reported as zero and the pair and cluster
// statistics still stand on their own.
func (r *CoUseRecorder) Report(params CoUseParams, lookup CategoryLookup) CoUseReport {
	if r == nil {
		return CoUseReport{Params: params}
	}
	if params.MinSupport < 1 {
		params.MinSupport = 1
	}

	r.mu.Lock()
	successSelections := r.selections.success
	failureSelections := r.selections.failure
	atoms := make(map[string]counts, len(r.atoms))
	for id, c := range r.atoms {
		atoms[id] = *c
	}
	pairs := make(map[pairKey]counts, len(r.pairs))
	for k, c := range r.pairs {
		pairs[k] = *c
	}
	pending := len(r.pending)
	dropped := r.dropped
	unattributed := r.unattributed
	truncated := r.truncated
	r.mu.Unlock()

	rep := CoUseReport{
		Params:            params,
		SuccessSelections: successSelections,
		FailureSelections: failureSelections,
		PendingTurns:      pending,
		DroppedSelections: dropped,

		UnattributedSelections: unattributed,
		DistinctAtoms:          len(atoms),
		Truncated:              truncated,
	}

	if successSelections == 0 {
		return rep
	}

	category := func(id string) string {
		if lookup == nil {
			return ""
		}
		return lookup(id)
	}

	// Marginals.
	ubiquitous := make(map[string]bool)
	rep.Atoms = make([]AtomStat, 0, len(atoms))
	for id, c := range atoms {
		share := float64(c.success) / float64(successSelections)
		st := AtomStat{
			ID:       id,
			Category: category(id),
			Success:  c.success,
			Failure:  c.failure,
			Share:    share,
		}
		if share >= params.UbiquityThreshold {
			st.Ubiquitous = true
			ubiquitous[id] = true
			rep.Ubiquitous = append(rep.Ubiquitous, id)
		}
		rep.Atoms = append(rep.Atoms, st)
	}
	sort.Slice(rep.Atoms, func(i, j int) bool {
		if rep.Atoms[i].Success != rep.Atoms[j].Success {
			return rep.Atoms[i].Success > rep.Atoms[j].Success
		}
		return rep.Atoms[i].ID < rep.Atoms[j].ID
	})
	sort.Strings(rep.Ubiquitous)

	// Pairs, excluding anything anchored on a ubiquitous atom.
	total := float64(successSelections)
	uf := newUnionFind()
	for key, c := range pairs {
		if c.success < params.MinSupport {
			continue
		}
		if ubiquitous[key.a] || ubiquitous[key.b] {
			continue
		}
		ca, cb := atoms[key.a], atoms[key.b]
		if ca.success < params.MinSupport || cb.success < params.MinSupport {
			continue
		}

		pA := float64(ca.success) / total
		pB := float64(cb.success) / total
		pAB := float64(c.success) / total
		if pA == 0 || pB == 0 {
			continue
		}
		lift := pAB / (pA * pB)
		union := ca.success + cb.success - c.success
		jaccard := 0.0
		if union > 0 {
			jaccard = float64(c.success) / float64(union)
		}

		catA, catB := category(key.a), category(key.b)
		ps := PairStat{
			A:            key.a,
			B:            key.b,
			Joint:        c.success,
			Lift:         lift,
			Jaccard:      jaccard,
			SameCategory: catA != "" && catA == catB,
		}
		if lift < params.MinLift {
			continue
		}
		rep.Pairs = append(rep.Pairs, ps)
		if !ps.SameCategory {
			rep.CrossCategoryPairs++
		}
		uf.union(key.a, key.b)
	}

	sort.Slice(rep.Pairs, func(i, j int) bool {
		if rep.Pairs[i].Lift != rep.Pairs[j].Lift {
			return rep.Pairs[i].Lift > rep.Pairs[j].Lift
		}
		if rep.Pairs[i].A != rep.Pairs[j].A {
			return rep.Pairs[i].A < rep.Pairs[j].A
		}
		return rep.Pairs[i].B < rep.Pairs[j].B
	})

	// Clusters.
	groups := uf.groups()
	rep.Clusters = make([]Cluster, 0, len(groups))
	var weighted, weight float64
	for _, members := range groups {
		if len(members) < 2 {
			continue
		}
		sort.Strings(members)
		modal, purity := modalCategory(members, category)
		rep.Clusters = append(rep.Clusters, Cluster{
			Atoms:         members,
			ModalCategory: modal,
			Purity:        purity,
		})
		rep.ClusteredAtoms += len(members)
		weighted += purity * float64(len(members))
		weight += float64(len(members))
	}
	sort.Slice(rep.Clusters, func(i, j int) bool {
		if len(rep.Clusters[i].Atoms) != len(rep.Clusters[j].Atoms) {
			return len(rep.Clusters[i].Atoms) > len(rep.Clusters[j].Atoms)
		}
		return rep.Clusters[i].Atoms[0] < rep.Clusters[j].Atoms[0]
	})

	if weight > 0 && lookup != nil {
		rep.CategoryAlignment = weighted / weight
	}

	clustered := make(map[string]bool, rep.ClusteredAtoms)
	for _, cl := range rep.Clusters {
		for _, id := range cl.Atoms {
			clustered[id] = true
		}
	}
	for id, c := range atoms {
		if ubiquitous[id] || clustered[id] {
			continue
		}
		if c.success < params.MinSupport {
			// Too rare to have been given a chance to cluster. Counting it as
			// isolated would blame the data for a sample-size problem.
			continue
		}
		rep.IsolatedAtoms++
	}

	return rep
}

// modalCategory returns the most common category among members and the share of
// members carrying it. Ties break on the lexically first category so the same
// input always produces the same report.
func modalCategory(members []string, category func(string) string) (string, float64) {
	tally := make(map[string]int, len(members))
	known := 0
	for _, id := range members {
		cat := category(id)
		if cat == "" {
			continue
		}
		tally[cat]++
		known++
	}
	if known == 0 {
		return "", 0
	}

	best, bestN := "", 0
	names := make([]string, 0, len(tally))
	for cat := range tally {
		names = append(names, cat)
	}
	sort.Strings(names)
	for _, cat := range names {
		if tally[cat] > bestN {
			best, bestN = cat, tally[cat]
		}
	}
	// Purity is over all members, not only the categorized ones: an uncategorized
	// atom inside a cluster is evidence the taxonomy does not cover it, and
	// dividing it out would score that gap as agreement.
	return best, float64(bestN) / float64(len(members))
}

// ---------------------------------------------------------------------------
// union-find
// ---------------------------------------------------------------------------

type unionFind struct{ parent map[string]string }

func newUnionFind() *unionFind { return &unionFind{parent: make(map[string]string)} }

func (u *unionFind) find(x string) string {
	root, ok := u.parent[x]
	if !ok {
		u.parent[x] = x
		return x
	}
	if root == x {
		return x
	}
	r := u.find(root)
	u.parent[x] = r // path compression
	return r
}

func (u *unionFind) union(a, b string) {
	ra, rb := u.find(a), u.find(b)
	if ra == rb {
		return
	}
	// Union by lexical order rather than by rank: the tree here is tiny, and a
	// deterministic root makes the grouped output stable across runs.
	if ra < rb {
		u.parent[rb] = ra
	} else {
		u.parent[ra] = rb
	}
}

func (u *unionFind) groups() map[string][]string {
	out := make(map[string][]string)
	for x := range u.parent {
		r := u.find(x)
		out[r] = append(out[r], x)
	}
	return out
}
