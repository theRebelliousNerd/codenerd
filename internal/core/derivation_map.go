package core

import (
	"fmt"
	"path"
	"strings"

	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/parse"
)

// Static derivation map: which shards can derive which predicates and which
// shared facts each shard's rules actually consume. It ports the reference
// analysis in .claude/skills/codenerd-dogfood/scripts/shard_join_audit.py:
// presence lattice per predicate (ALL for program facts and shared
// predicates, owner-or-catchAll for runtime facts, derived fixpoint over
// positive body atoms) plus split-join and blind-negation findings.

// ShardSet is a set of shard domain names.
type ShardSet map[string]struct{}

// Presence describes where a predicate's facts can exist. All means every
// shard; otherwise Shards holds the exact set (empty means nowhere).
type Presence struct {
	All    bool
	Shards map[string]struct{}
}

// AllPresence returns the ALL presence (every shard).
func AllPresence() Presence {
	return Presence{All: true}
}

// EmptyPresence returns the empty presence (nowhere).
func EmptyPresence() Presence {
	return Presence{}
}

// SingleShardPresence returns the presence containing exactly one shard.
func SingleShardPresence(shard string) Presence {
	return Presence{Shards: map[string]struct{}{shard: {}}}
}

// Clone returns a deep copy of the presence.
func (p Presence) Clone() Presence {
	if p.All {
		return Presence{All: true}
	}
	if len(p.Shards) == 0 {
		return Presence{}
	}
	out := make(map[string]struct{}, len(p.Shards))
	for s := range p.Shards {
		out[s] = struct{}{}
	}
	return Presence{Shards: out}
}

// IsEmpty reports whether the presence is the empty set (not ALL).
func (p Presence) IsEmpty() bool {
	return !p.All && len(p.Shards) == 0
}

// Equals reports whether two presences denote the same shard set.
func (p Presence) Equals(o Presence) bool {
	if p.All != o.All {
		return false
	}
	if p.All {
		return true
	}
	if len(p.Shards) != len(o.Shards) {
		return false
	}
	for s := range p.Shards {
		if _, ok := o.Shards[s]; !ok {
			return false
		}
	}
	return true
}

// Meet is lattice intersection: ALL meets X as X.
func (p Presence) Meet(o Presence) Presence {
	if p.All {
		return o.Clone()
	}
	if o.All {
		return p.Clone()
	}
	return intersectShards(p.Shards, o.Shards)
}

// Join is lattice union: anything joined with ALL is ALL.
func (p Presence) Join(o Presence) Presence {
	if p.All || o.All {
		return Presence{All: true}
	}
	return unionShards(p.Shards, o.Shards)
}

// SubsetOf reports whether p is a subset of o.
func (p Presence) SubsetOf(o Presence) bool {
	if p.All {
		return o.All
	}
	if o.All {
		return true
	}
	for s := range p.Shards {
		if _, ok := o.Shards[s]; !ok {
			return false
		}
	}
	return true
}

// Contains reports whether the shard is in the presence.
func (p Presence) Contains(shard string) bool {
	if p.All {
		return true
	}
	_, ok := p.Shards[shard]
	return ok
}

func intersectShards(a, b map[string]struct{}) Presence {
	if len(a) == 0 || len(b) == 0 {
		return Presence{}
	}
	small, large := a, b
	if len(large) < len(small) {
		small, large = large, small
	}
	out := make(map[string]struct{})
	for s := range small {
		if _, ok := large[s]; ok {
			out[s] = struct{}{}
		}
	}
	return Presence{Shards: out}
}

func unionShards(a, b map[string]struct{}) Presence {
	if len(a) == 0 && len(b) == 0 {
		return Presence{}
	}
	out := make(map[string]struct{}, len(a)+len(b))
	for s := range a {
		out[s] = struct{}{}
	}
	for s := range b {
		out[s] = struct{}{}
	}
	return Presence{Shards: out}
}

// DerivationMap is the static cross-shard derivation analysis.
type DerivationMap struct {
	// Arities comes from loaded declarations, so an unloaded policy module
	// cannot satisfy a runtime integration contract by merely existing on disk.
	Arities map[string]int
	// Presence: where a predicate's facts can exist.
	Presence map[string]Presence
	// QueryTargets: where a query for a derived predicate must look (a rule
	// that fires everywhere contributes the catch-all, plus every shard
	// where its premises' facts are local).
	QueryTargets map[string]Presence
	// Local: per derived predicate, the shards whose own facts can add to
	// the copy every shard derives alike -- where a read must look beyond
	// the catch-all.
	Local map[string]Presence
	// Consumes: shard -> shared predicates referenced by a rule that can
	// fire there.
	Consumes       map[string]map[string]struct{}
	SplitJoins     []RuleFinding
	BlindNegations []RuleFinding
	CatchAll       string
	// Rules: every analysed rule with the shard set it fires in, for
	// diagnostics ("why does a query for X visit shard Y").
	Rules []RuleSummary
}

// RuleSummary is one rule's static firing set.
type RuleSummary struct {
	File  string
	Head  string
	Pos   []string
	Neg   []string
	Fires Presence
}

// RuleFinding describes one split join or blind negation rule.
type RuleFinding struct {
	File    string
	Head    string
	Clause  string
	Homes   map[string]Presence
	Negated string
}

var scopeEvaluatedFiles = map[string]struct{}{
	"jit_compiler.mg":  {},
	"jit_selection.mg": {},
	"jit_logic.mg":     {},
}

type parsedRule struct {
	file   string
	head   string
	pos    []string
	neg    []string
	clause string
}

type fileChunk struct {
	file string
	text string
}

func splitPolicyByFile(policyText string) []fileChunk {
	var chunks []fileChunk
	curFile := ""
	var sb strings.Builder
	flush := func() {
		txt := sb.String()
		sb.Reset()
		if strings.TrimSpace(txt) == "" {
			return
		}
		chunks = append(chunks, fileChunk{file: curFile, text: txt})
	}
	for _, line := range strings.Split(policyText, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# Policy Module:") {
			flush()
			name := strings.TrimSpace(strings.TrimPrefix(trimmed, "# Policy Module:"))
			if name != "" {
				name = path.Base(name)
			}
			curFile = name
			sb.WriteString(line)
			sb.WriteString("\n")
			continue
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	flush()
	return chunks
}

func isBuiltinBodyPred(sym string) bool {
	return sym == "" || strings.HasPrefix(sym, ":") || strings.HasPrefix(sym, "fn:")
}

func bodyTermPred(term ast.Term) (sym string, negated bool, ok bool) {
	switch p := term.(type) {
	case ast.Atom:
		sym = p.Predicate.Symbol
	case *ast.Atom:
		if p == nil {
			return "", false, false
		}
		sym = p.Predicate.Symbol
	case ast.NegAtom:
		sym = p.Atom.Predicate.Symbol
		negated = true
	case *ast.NegAtom:
		if p == nil {
			return "", false, false
		}
		sym = p.Atom.Predicate.Symbol
		negated = true
	default:
		return "", false, false
	}
	if isBuiltinBodyPred(sym) {
		return "", false, false
	}
	return sym, negated, true
}

func clauseBodyPreds(clause ast.Clause) (pos []string, neg []string) {
	for _, term := range clause.Premises {
		sym, isNeg, ok := bodyTermPred(term)
		if !ok {
			continue
		}
		if isNeg {
			neg = append(neg, sym)
		} else {
			pos = append(pos, sym)
		}
	}
	return pos, neg
}

type derivationBuilder struct {
	arities    map[string]int
	owners     map[string]string
	shared     map[string]struct{}
	catchAll   string
	programEDB map[string]struct{}
	declSet    map[string]struct{}
	rules      []parsedRule
	derived    map[string]struct{}
	presence   map[string]Presence
	leafMemo   map[string]map[string]struct{}
	// adds / cuts: per derived predicate, the shards whose own facts can add
	// facts to, or remove facts from, the copy every shard derives alike
	// (localFixpoint). Presence cannot say this: All means "can exist in
	// every shard", not "is the same in every shard".
	adds map[string]map[string]struct{}
	cuts map[string]map[string]struct{}
}

func newDerivationBuilder(owners map[string]string, shared map[string]struct{}, catchAll string) *derivationBuilder {
	if owners == nil {
		owners = map[string]string{}
	}
	if shared == nil {
		shared = map[string]struct{}{}
	}
	if catchAll == "" {
		catchAll = "cortex"
	}
	return &derivationBuilder{
		arities:    make(map[string]int),
		owners:     owners,
		shared:     shared,
		catchAll:   catchAll,
		programEDB: make(map[string]struct{}),
		declSet:    make(map[string]struct{}),
		presence:   make(map[string]Presence),
		leafMemo:   make(map[string]map[string]struct{}),
	}
}

func (b *derivationBuilder) edbPresence(p string) Presence {
	if _, ok := b.programEDB[p]; ok {
		if _, isDerived := b.derived[p]; !isDerived {
			return AllPresence()
		}
	}
	if _, ok := b.shared[p]; ok {
		return AllPresence()
	}
	if owner, ok := b.owners[p]; ok {
		return SingleShardPresence(owner)
	}
	return SingleShardPresence(b.catchAll)
}

func (b *derivationBuilder) getPresence(p string) Presence {
	if pr, ok := b.presence[p]; ok {
		return pr
	}
	return b.edbPresence(p)
}

func (b *derivationBuilder) parse(policyText string, programFacts map[string]struct{}) error {
	for p := range programFacts {
		if p != "" {
			b.programEDB[p] = struct{}{}
		}
	}
	for _, chunk := range splitPolicyByFile(policyText) {
		if strings.TrimSpace(chunk.text) == "" {
			continue
		}
		unit, err := parseUnit(strings.NewReader(chunk.text))
		if err != nil {
			return b.parseError(chunk.file, err)
		}
		b.collectUnit(chunk.file, unit)
	}
	b.derived = make(map[string]struct{}, len(b.rules))
	for _, r := range b.rules {
		b.derived[r.head] = struct{}{}
	}
	return nil
}

func (b *derivationBuilder) parseError(file string, err error) error {
	if file == "" {
		return fmt.Errorf("derivation map: failed to parse policy text: %w", err)
	}
	return fmt.Errorf("derivation map: failed to parse %q: %w", file, err)
}

// collectUnit records one file's declarations, program facts and rules.
// Rules from the scope-evaluated JIT files are skipped: they run inside a
// per-compilation scope cloned from the catch-all, outside the shard model.
func (b *derivationBuilder) collectUnit(file string, unit parse.SourceUnit) {
	for _, decl := range unit.Decls {
		b.declSet[decl.DeclaredAtom.Predicate.Symbol] = struct{}{}
		b.arities[decl.DeclaredAtom.Predicate.Symbol] = decl.DeclaredAtom.Predicate.Arity
	}
	_, scopeEvaluated := scopeEvaluatedFiles[file]
	for _, clause := range unit.Clauses {
		head := clause.Head.Predicate.Symbol
		if head == "" {
			continue
		}
		if len(clause.Premises) == 0 && clause.Transform == nil {
			b.programEDB[head] = struct{}{}
			continue
		}
		if scopeEvaluated {
			continue
		}
		pos, neg := clauseBodyPreds(clause)
		b.rules = append(b.rules, parsedRule{
			file:   file,
			head:   head,
			pos:    pos,
			neg:    neg,
			clause: clause.String(),
		})
	}
}

func (b *derivationBuilder) seed() {
	for p := range b.declSet {
		if _, isDerived := b.derived[p]; isDerived {
			continue
		}
		b.presence[p] = b.edbPresence(p)
	}
	b.seedSet(b.programEDB)
	b.seedSetOwners()
	for p := range b.shared {
		if _, isDerived := b.derived[p]; isDerived {
			continue
		}
		if _, ok := b.presence[p]; !ok {
			b.presence[p] = AllPresence()
		}
	}
	for p := range b.derived {
		if _, ok := b.programEDB[p]; ok {
			b.presence[p] = AllPresence()
		} else {
			b.presence[p] = EmptyPresence()
		}
	}
}

func (b *derivationBuilder) seedSet(set map[string]struct{}) {
	for p := range set {
		if _, isDerived := b.derived[p]; isDerived {
			continue
		}
		if _, ok := b.presence[p]; !ok {
			b.presence[p] = b.edbPresence(p)
		}
	}
}

func (b *derivationBuilder) seedSetOwners() {
	for p := range b.owners {
		if _, isDerived := b.derived[p]; isDerived {
			continue
		}
		if _, ok := b.presence[p]; !ok {
			b.presence[p] = b.edbPresence(p)
		}
	}
}

func (b *derivationBuilder) fixpoint() {
	for changed := true; changed; {
		changed = false
		for _, r := range b.rules {
			cur := AllPresence()
			for _, p := range r.pos {
				cur = cur.Meet(b.getPresence(p))
			}
			joined := b.presence[r.head].Join(cur)
			if !joined.Equals(b.presence[r.head]) {
				b.presence[r.head] = joined
				changed = true
			}
		}
	}
}

// Uniform versus local facts.
//
// Every shard holds the program facts and the replicated shared facts, and
// evaluates the whole program, so what those alone derive is the same in every
// shard: the uniform copy, which the catch-all answers for. Everything else a
// shard derives depends on facts only it holds -- its owned predicates, or for
// the catch-all the unowned runtime ones -- and is local to it.
//
// A rule whose positive body is present everywhere (Presence All) fires in
// every shard, but it derives the same facts everywhere only if its whole body
// is uniform. context_relevant is the live case: one rule reads shared
// user_intent, so its Presence is All, and another reads world-owned modified.
// should_include_context(F, P) :- context_relevant(F, P) therefore fires
// everywhere, and its facts for a modified file exist only in the world shard.
// queryTargets used to send that query to the catch-all alone, so no modified
// file reached the context window through the kernel.

// edbLocal is where a non-derived predicate's facts are shard-specific: its
// owner, the catch-all for an unowned runtime predicate, nowhere for a program
// or shared one (every shard holds the same copy).
func (b *derivationBuilder) edbLocal(p string) map[string]struct{} {
	if _, ok := b.shared[p]; ok {
		return nil
	}
	if owner, ok := b.owners[p]; ok {
		return map[string]struct{}{owner: {}}
	}
	if _, ok := b.programEDB[p]; ok {
		return nil
	}
	if _, isDerived := b.derived[p]; isDerived {
		return nil
	}
	return map[string]struct{}{b.catchAll: {}}
}

// Local facts have a direction. A shard's own facts reaching a positive
// premise can ADD head facts there that no other shard derives; reaching a
// negated premise they can only REMOVE head facts there, so that shard holds
// fewer than the others. A read is a union across the shards it visits, so
// only a shard that can add facts is worth visiting: one that can only cut
// returns a subset of what the catch-all already returns. Through a negation
// the two swap (a cut under a negation is an add). delegate_task is why this
// matters: it reads missing_tool_for, which is tools- and world-local only
// through !has_capability, so those shards hold fewer delegations, never more,
// and visiting them would cost a world-shard evaluation on every step for
// nothing.

// localityOf returns the shards whose own facts can add p facts to, and remove
// p facts from, the copy every shard derives alike.
func (b *derivationBuilder) localityOf(p string) (adds, cuts map[string]struct{}) {
	if _, isDerived := b.derived[p]; !isDerived {
		return b.edbLocal(p), nil
	}
	adds, cuts = b.adds[p], b.cuts[p]
	// A derived predicate Go also asserts into its owner has facts of its own
	// there.
	if owner, ok := b.owners[p]; ok {
		if _, has := adds[owner]; !has {
			merged := make(map[string]struct{}, len(adds)+1)
			for s := range adds {
				merged[s] = struct{}{}
			}
			merged[owner] = struct{}{}
			adds = merged
		}
	}
	return adds, cuts
}

// ruleLocality is where a rule's facts differ from the uniform copy, in each
// direction. A rule that fires in only some shards adds its facts there and
// nowhere else. A rule that fires everywhere adds where a positive premise
// adds or a negated one cuts, and cuts where a positive premise cuts or a
// negated one adds.
func (b *derivationBuilder) ruleLocality(r parsedRule) (adds, cuts map[string]struct{}) {
	adds = make(map[string]struct{})
	cuts = make(map[string]struct{})
	union := func(into, from map[string]struct{}) {
		for s := range from {
			into[s] = struct{}{}
		}
	}
	for _, p := range r.pos {
		a, c := b.localityOf(p)
		union(adds, a)
		union(cuts, c)
	}
	for _, p := range r.neg {
		a, c := b.localityOf(p)
		union(adds, c)
		union(cuts, a)
	}
	if cur := positiveIntersection(b, r.pos); !cur.All {
		adds = make(map[string]struct{}, len(cur.Shards))
		union(adds, cur.Shards)
	}
	return adds, cuts
}

// localFixpoint computes adds and cuts for every derived predicate: the union
// of its rules' ruleLocality, iterated to a fixpoint (recursive rules).
func (b *derivationBuilder) localFixpoint() {
	b.adds = make(map[string]map[string]struct{}, len(b.derived))
	b.cuts = make(map[string]map[string]struct{}, len(b.derived))
	for p := range b.derived {
		b.adds[p] = map[string]struct{}{}
		b.cuts[p] = map[string]struct{}{}
	}
	grow := func(into, from map[string]struct{}) bool {
		grew := false
		for s := range from {
			if _, ok := into[s]; !ok {
				into[s] = struct{}{}
				grew = true
			}
		}
		return grew
	}
	for changed := true; changed; {
		changed = false
		for _, r := range b.rules {
			adds, cuts := b.ruleLocality(r)
			if grow(b.adds[r.head], adds) {
				changed = true
			}
			if grow(b.cuts[r.head], cuts) {
				changed = true
			}
		}
	}
}

// ruleReadShards is the smallest shard set whose facts cover everything a
// rule derives: the shards it fires in when it fires in only some; else the
// catch-all for the uniform copy plus every shard where its facts can be
// added to it. A query reads a derived predicate there (queryTargets), so
// those shards also need the rule's shared inputs (consumes).
func (b *derivationBuilder) ruleReadShards(r parsedRule) map[string]struct{} {
	adds, _ := b.ruleLocality(r)
	if cur := positiveIntersection(b, r.pos); cur.All {
		adds[b.catchAll] = struct{}{}
	}
	return adds
}

func positiveIntersection(b *derivationBuilder, pos []string) Presence {
	cur := AllPresence()
	for _, p := range pos {
		cur = cur.Meet(b.getPresence(p))
	}
	return cur
}

func (b *derivationBuilder) findings() ([]RuleFinding, []RuleFinding) {
	var splits []RuleFinding
	var blinds []RuleFinding
	for _, r := range b.rules {
		homes := b.homesFor(r.pos)
		cur := positiveIntersection(b, r.pos)
		if cur.IsEmpty() {
			splits = append(splits, RuleFinding{File: r.file, Head: r.head, Clause: r.clause, Homes: homes})
			continue
		}
		blinds = append(blinds, b.blindsFor(r, homes, cur)...)
	}
	return splits, blinds
}

func (b *derivationBuilder) homesFor(pos []string) map[string]Presence {
	homes := make(map[string]Presence, len(pos))
	for _, p := range pos {
		if _, ok := homes[p]; !ok {
			homes[p] = b.getPresence(p).Clone()
		}
	}
	return homes
}

func (b *derivationBuilder) blindsFor(r parsedRule, homes map[string]Presence, cur Presence) []RuleFinding {
	var out []RuleFinding
	for _, np := range r.neg {
		pr := b.getPresence(np)
		if pr.All {
			continue
		}
		if cur.All || !cur.SubsetOf(pr) {
			out = append(out, RuleFinding{File: r.file, Head: r.head, Clause: r.clause, Homes: homes, Negated: np})
		}
	}
	return out
}

func (b *derivationBuilder) consumes() map[string]map[string]struct{} {
	allShards := make(map[string]struct{})
	for _, o := range b.owners {
		allShards[o] = struct{}{}
	}
	allShards[b.catchAll] = struct{}{}
	out := make(map[string]map[string]struct{}, len(allShards))
	for s := range allShards {
		out[s] = make(map[string]struct{})
	}
	for _, r := range b.rules {
		b.consumeRule(out, allShards, r)
	}
	return out
}

func (b *derivationBuilder) consumeRule(out map[string]map[string]struct{}, allShards map[string]struct{}, r parsedRule) {
	sharedInRule := b.sharedInRule(r)
	if len(sharedInRule) == 0 {
		return
	}
	add := func(s string, preds map[string]struct{}) {
		m, ok := out[s]
		if !ok {
			m = make(map[string]struct{})
			out[s] = m
		}
		for q := range preds {
			m[q] = struct{}{}
		}
	}
	cur := positiveIntersection(b, r.pos)
	if !cur.All {
		// The rule fires only in these shards: each needs every shared
		// input of the whole derivation chain.
		for s := range cur.Shards {
			add(s, sharedInRule)
		}
		return
	}
	// The catch-all answers for the copy every shard derives alike.
	add(b.catchAll, sharedInRule)
	// A shard read for its local additions needs only what those additions
	// join with (sharedForLocal), not the inputs of the uniform copy the
	// catch-all already derives.
	adds, _ := b.ruleLocality(r)
	for s := range adds {
		if s != b.catchAll {
			add(s, b.sharedForLocal(r, s))
		}
	}
}

// sharedForLocal returns the shared predicates shard s must hold for rule r's
// facts local to s to be derived there. When exactly one positive premise
// carries local facts in s, what s adds is that premise's local facts joined
// with the rest of the body in full: every other premise needs all its shared
// inputs (a negated one most of all, or the join over-derives), while the
// local premise's own derivation is registered by the rules that make it local
// in s. Any other shape (several local premises, or locality through a
// negation) needs every shared input of the whole rule.
func (b *derivationBuilder) sharedForLocal(r parsedRule, s string) map[string]struct{} {
	var localPos []string
	for _, p := range r.pos {
		if adds, _ := b.localityOf(p); adds != nil {
			if _, ok := adds[s]; ok {
				localPos = append(localPos, p)
			}
		}
	}
	skip := ""
	if len(localPos) == 1 {
		skip = localPos[0]
	}
	out := make(map[string]struct{})
	for _, p := range r.pos {
		if p != skip {
			b.sharedLeaves(p, out, map[string]struct{}{})
		}
	}
	for _, p := range r.neg {
		b.sharedLeaves(p, out, map[string]struct{}{})
	}
	return out
}

// sharedInRule returns every shared predicate the rule depends on, directly
// or through the derived predicates in its body (positive or negated): a
// shard that fires the rule must hold all of them for the derivation chain
// to exist there.
func (b *derivationBuilder) sharedInRule(r parsedRule) map[string]struct{} {
	found := make(map[string]struct{})
	for _, p := range r.pos {
		b.sharedLeaves(p, found, map[string]struct{}{})
	}
	for _, p := range r.neg {
		b.sharedLeaves(p, found, map[string]struct{}{})
	}
	return found
}

// sharedLeaves adds to out the shared predicates reachable from p through
// rule bodies. Memoised per builder; cycles are cut by the stack.
func (b *derivationBuilder) sharedLeaves(p string, out map[string]struct{}, stack map[string]struct{}) {
	if _, ok := b.shared[p]; ok {
		out[p] = struct{}{}
		return
	}
	if _, isDerived := b.derived[p]; !isDerived {
		return
	}
	if _, cycling := stack[p]; cycling {
		return
	}
	if cached, ok := b.leafMemo[p]; ok {
		for q := range cached {
			out[q] = struct{}{}
		}
		return
	}
	stack[p] = struct{}{}
	local := make(map[string]struct{})
	for _, r := range b.rules {
		if r.head != p {
			continue
		}
		for _, q := range r.pos {
			b.sharedLeaves(q, local, stack)
		}
		for _, q := range r.neg {
			b.sharedLeaves(q, local, stack)
		}
	}
	delete(stack, p)
	b.leafMemo[p] = local
	for q := range local {
		out[q] = struct{}{}
	}
}

// BuildDerivationMap analyzes concatenated policy text with the real Mangle
// parser and computes the static cross-shard derivation map.
func BuildDerivationMap(policyText string, programFacts map[string]struct{}, owners map[string]string, shared map[string]struct{}, catchAll string) (*DerivationMap, error) {
	b := newDerivationBuilder(owners, shared, catchAll)
	if err := b.parse(policyText, programFacts); err != nil {
		return nil, err
	}
	b.seed()
	b.fixpoint()
	b.localFixpoint()
	splits, blinds := b.findings()
	summaries := make([]RuleSummary, 0, len(b.rules))
	for _, r := range b.rules {
		summaries = append(summaries, RuleSummary{
			File: r.file, Head: r.head, Pos: r.pos, Neg: r.neg,
			Fires: positiveIntersection(b, r.pos),
		})
	}
	local := make(map[string]Presence, len(b.adds))
	for p, shards := range b.adds {
		local[p] = Presence{Shards: shards}
	}
	return &DerivationMap{
		Arities:        b.arities,
		Presence:       b.presence,
		QueryTargets:   b.queryTargets(),
		Local:          local,
		Consumes:       b.consumes(),
		SplitJoins:     splits,
		BlindNegations: blinds,
		CatchAll:       b.catchAll,
		Rules:          summaries,
	}, nil
}

// queryTargets computes, per derived predicate, the smallest shard set a
// query must visit to see every fact: the union over its rules of
// ruleReadShards. A rule that fires everywhere contributes the catch-all for
// the copy every shard derives alike, and a shard beyond it only when a
// premise's facts are local there. Presence answers "where can it exist";
// QueryTargets answers "where must I look". delegate_task is why the catch-all
// stands in for the uniform copy: one rule needs only user_intent, so Presence
// is All, and a fan-out paid a world-shard evaluation on every step for facts
// the catch-all already had. should_include_context is why that is not the
// whole answer (see the note on uniform versus local facts above).
func (b *derivationBuilder) queryTargets() map[string]Presence {
	out := make(map[string]Presence, len(b.derived))
	for _, r := range b.rules {
		contrib := Presence{Shards: b.ruleReadShards(r)}
		if prev, ok := out[r.head]; ok {
			out[r.head] = unionShards(prev.Shards, contrib.Shards)
		} else {
			out[r.head] = contrib.Clone()
		}
	}
	// A derived predicate that is also a program fact exists everywhere as
	// EDB; the catch-all sees that copy too, so no change is needed.
	return out
}

// ShardsFor returns the shards a query for pred must visit, preserving
// allShards order: the QueryTargets set for a derived predicate, the owner
// for an owned one, the catch-all for a shared one; unknown predicates and
// a nil map yield all shards.
func (m *DerivationMap) ShardsFor(pred string, allShards []string) []string {
	all := func() []string {
		out := make([]string, len(allShards))
		copy(out, allShards)
		return out
	}
	if m == nil || m.Presence == nil {
		return all()
	}
	pick := func(set map[string]struct{}) []string {
		var out []string
		for _, s := range allShards {
			if _, ok := set[s]; ok {
				out = append(out, s)
			}
		}
		return out
	}
	if qt, ok := m.QueryTargets[pred]; ok && !qt.All && len(qt.Shards) > 0 {
		if got := pick(qt.Shards); len(got) > 0 {
			return got
		}
	}
	pr, ok := m.Presence[pred]
	if !ok {
		return all()
	}
	if pr.All {
		// Present everywhere and identical everywhere (a program table or a
		// shared fact): the catch-all answers for all of them.
		if m.CatchAll != "" {
			if got := pick(map[string]struct{}{m.CatchAll: {}}); len(got) == 1 {
				return got
			}
		}
		return all()
	}
	return pick(pr.Shards)
}
