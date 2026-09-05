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
	// Presence: where a predicate's facts can exist.
	Presence map[string]Presence
	// QueryTargets: where a query for a derived predicate must look (rules
	// that fire everywhere contribute only the catch-all).
	QueryTargets map[string]Presence
	// Consumes: shard -> shared predicates referenced by a rule that can
	// fire there.
	Consumes       map[string]map[string]struct{}
	SplitJoins     []RuleFinding
	BlindNegations []RuleFinding
	CatchAll       string
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
	owners     map[string]string
	shared     map[string]struct{}
	catchAll   string
	programEDB map[string]struct{}
	declSet    map[string]struct{}
	rules      []parsedRule
	derived    map[string]struct{}
	presence   map[string]Presence
	leafMemo   map[string]map[string]struct{}
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
	cur := positiveIntersection(b, r.pos)
	if cur.All {
		// A rule that fires everywhere derives the same facts in every
		// shard, and queries read those from the catch-all (queryTargets),
		// so only the catch-all needs this rule's shared inputs.
		m, ok := out[b.catchAll]
		if !ok {
			m = make(map[string]struct{})
			out[b.catchAll] = m
		}
		for q := range sharedInRule {
			m[q] = struct{}{}
		}
		return
	}
	for s := range cur.Shards {
		m, ok := out[s]
		if !ok {
			m = make(map[string]struct{})
			out[s] = m
		}
		for q := range sharedInRule {
			m[q] = struct{}{}
		}
	}
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
	splits, blinds := b.findings()
	return &DerivationMap{
		Presence:       b.presence,
		QueryTargets:   b.queryTargets(),
		Consumes:       b.consumes(),
		SplitJoins:     splits,
		BlindNegations: blinds,
		CatchAll:       b.catchAll,
	}, nil
}

// queryTargets computes, per derived predicate, the smallest shard set a
// query must visit to see every fact: the union over its rules of each
// rule's firing set, where a rule that fires everywhere (only shared and
// program facts in its body) derives the same facts in every shard and so
// contributes the catch-all alone. Presence answers "where can it exist";
// QueryTargets answers "where must I look". delegate_task is the live case:
// one rule needs only user_intent, so Presence is All and a fan-out paid a
// world-shard evaluation on every step for facts the catch-all already had.
func (b *derivationBuilder) queryTargets() map[string]Presence {
	out := make(map[string]Presence, len(b.derived))
	for _, r := range b.rules {
		cur := positiveIntersection(b, r.pos)
		var contrib Presence
		if cur.All {
			contrib = SingleShardPresence(b.catchAll)
		} else {
			contrib = cur
		}
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
	if !ok || pr.All {
		return all()
	}
	return pick(pr.Shards)
}
