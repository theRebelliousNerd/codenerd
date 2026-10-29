package mcp

import (
	"encoding/json"
	"sort"
	"strings"
	"unicode"
)

// A control plane has to describe a server it has never seen before with a
// vocabulary it fixed in advance. That is the whole trick: the facade tools are
// constant, so the only thing that can vary per server is which facets it fills
// and how many tools sit behind each one.
//
// Facets are therefore deliberately few and deliberately verb-shaped. Six is
// enough to route an intent ("I need to find something" -> /search) and small
// enough that the atlas for an entire fleet of servers fits in a couple of
// hundred tokens. Adding a seventh costs every atlas render forever, so the bar
// is that a facet must change which tool an agent reaches for, not merely
// describe the tool more precisely.

// Facet is the canonical verb a tool belongs to.
type Facet string

const (
	// FacetRead retrieves a named thing whose identity the caller already has.
	FacetRead Facet = "read"
	// FacetSearch finds things the caller cannot name yet.
	FacetSearch Facet = "search"
	// FacetAnalyze examines and reports without changing anything. Pure
	// transforms live here too: they compute, they do not persist.
	FacetAnalyze Facet = "analyze"
	// FacetWrite creates, modifies, or removes state.
	FacetWrite Facet = "write"
	// FacetExecute runs caller-supplied code or commands.
	FacetExecute Facet = "execute"
	// FacetManage changes the connection, session, or configuration itself
	// rather than the data behind it.
	FacetManage Facet = "manage"
)

// AllFacets lists every facet in atlas display order: cheapest and safest
// first, so a truncated atlas loses the dangerous rows rather than the useful
// ones.
var AllFacets = []Facet{FacetRead, FacetSearch, FacetAnalyze, FacetWrite, FacetExecute, FacetManage}

// Valid reports whether f is one of the canonical facets. An unknown facet from
// a config file or a cached row must not silently become an atlas section.
func (f Facet) Valid() bool {
	switch f {
	case FacetRead, FacetSearch, FacetAnalyze, FacetWrite, FacetExecute, FacetManage:
		return true
	}
	return false
}

// Atom renders the facet as a Mangle /name constant.
func (f Facet) Atom() string { return "/" + string(f) }

// RiskClass is the blast radius of one tool call.
//
// codeNERD's constitutional layer is default-deny: an action executes only if
// the kernel derives permitted(...). That rule needs something to reason over,
// and "this MCP server advertised 40 tools" is not it. Classifying blast radius
// at discovery time is what lets policy say "this shard may call /safe tools on
// any server but /arbitrary tools on none" without anyone hand-listing tools
// for a server nobody has connected yet.
type RiskClass string

const (
	// RiskSafe has no side effects the caller can observe later.
	RiskSafe RiskClass = "safe"
	// RiskMutating creates or modifies state that survives the call.
	RiskMutating RiskClass = "mutating"
	// RiskDestructive removes or overwrites state, with no in-band undo.
	RiskDestructive RiskClass = "destructive"
	// RiskArbitrary runs caller-supplied code or commands, so its blast radius
	// is whatever the server process can reach.
	RiskArbitrary RiskClass = "arbitrary"
)

// Valid reports whether r is one of the canonical risk classes.
func (r RiskClass) Valid() bool {
	switch r {
	case RiskSafe, RiskMutating, RiskDestructive, RiskArbitrary:
		return true
	}
	return false
}

// Atom renders the risk class as a Mangle /name constant.
func (r RiskClass) Atom() string { return "/" + string(r) }

// Rank orders risk classes so the maximum of two signals is well defined.
func (r RiskClass) Rank() int {
	switch r {
	case RiskSafe:
		return 0
	case RiskMutating:
		return 1
	case RiskDestructive:
		return 2
	case RiskArbitrary:
		return 3
	}
	return 1 // Unknown is treated as mutating, never as safe.
}

// ClassificationSource records which signal decided a classification. It is
// carried rather than discarded because the signals are not equally
// trustworthy: a server that declares readOnlyHint has told us the answer,
// while a name-prefix guess is an inference that policy may want to treat with
// more suspicion before granting a write.
type ClassificationSource string

const (
	// SourceAnnotation came from the server's own tool annotations.
	SourceAnnotation ClassificationSource = "annotation"
	// SourceCapability came from analyzer-extracted capabilities.
	SourceCapability ClassificationSource = "capability"
	// SourceName came from the tool's name.
	SourceName ClassificationSource = "name"
	// SourceSchema came from the shape of the tool's input schema.
	SourceSchema ClassificationSource = "schema"
	// SourceDefault means nothing matched and the conservative default applied.
	SourceDefault ClassificationSource = "default"
)

// FacetClassification is the derived control-plane classification of one tool.
type FacetClassification struct {
	Facet       Facet                `json:"facet"`
	Risk        RiskClass            `json:"risk"`
	FacetSource ClassificationSource `json:"facet_source"`
	RiskSource  ClassificationSource `json:"risk_source"`
}

// verbFacets maps a leading name token to a facet. A name token is a far
// stronger signal than a substring match anywhere in the description, which is
// why this is consulted before capabilities: "get_already_deleted_records"
// contains "delete" and is still a read.
var verbFacets = map[string]Facet{
	// read
	"get": FacetRead, "read": FacetRead, "list": FacetRead, "fetch": FacetRead,
	"show": FacetRead, "describe": FacetRead, "view": FacetRead, "load": FacetRead,
	"download": FacetRead, "peek": FacetRead, "head": FacetRead, "cat": FacetRead,
	"resolve": FacetRead, "lookup": FacetRead, "info": FacetRead, "status": FacetRead,

	// search
	"search": FacetSearch, "find": FacetSearch, "query": FacetSearch, "grep": FacetSearch,
	"scan": FacetSearch, "match": FacetSearch, "filter": FacetSearch, "browse": FacetSearch,
	"discover": FacetSearch, "locate": FacetSearch, "recall": FacetSearch, "retrieve": FacetSearch,

	// analyze
	"analyze": FacetAnalyze, "analyse": FacetAnalyze, "inspect": FacetAnalyze,
	"check": FacetAnalyze, "validate": FacetAnalyze, "verify": FacetAnalyze,
	"lint": FacetAnalyze, "diff": FacetAnalyze, "compare": FacetAnalyze,
	"explain": FacetAnalyze, "summarize": FacetAnalyze, "count": FacetAnalyze,
	"measure": FacetAnalyze, "estimate": FacetAnalyze, "predict": FacetAnalyze,
	"convert": FacetAnalyze, "format": FacetAnalyze, "render": FacetAnalyze,
	"parse": FacetAnalyze, "transform": FacetAnalyze, "encode": FacetAnalyze,
	"decode": FacetAnalyze, "diagnose": FacetAnalyze, "audit": FacetAnalyze,

	// write
	"create": FacetWrite, "write": FacetWrite, "update": FacetWrite, "set": FacetWrite,
	"add": FacetWrite, "put": FacetWrite, "patch": FacetWrite, "append": FacetWrite,
	"insert": FacetWrite, "upload": FacetWrite, "save": FacetWrite, "store": FacetWrite,
	"edit": FacetWrite, "modify": FacetWrite, "rename": FacetWrite, "move": FacetWrite,
	"copy": FacetWrite, "commit": FacetWrite, "push": FacetWrite, "merge": FacetWrite,
	"send": FacetWrite, "post": FacetWrite, "publish": FacetWrite, "apply": FacetWrite,
	"delete": FacetWrite, "remove": FacetWrite, "drop": FacetWrite, "purge": FacetWrite,
	"clear": FacetWrite, "truncate": FacetWrite, "destroy": FacetWrite, "reset": FacetWrite,
	"revert": FacetWrite, "rollback": FacetWrite, "prune": FacetWrite, "wipe": FacetWrite,

	// execute
	"run": FacetExecute, "exec": FacetExecute, "execute": FacetExecute,
	"invoke": FacetExecute, "eval": FacetExecute, "call": FacetExecute,
	"shell": FacetExecute, "spawn": FacetExecute, "launch": FacetExecute,
	"compile": FacetExecute, "build": FacetExecute, "deploy": FacetExecute,

	// manage
	"connect": FacetManage, "disconnect": FacetManage, "configure": FacetManage,
	"config": FacetManage, "auth": FacetManage, "login": FacetManage,
	"logout": FacetManage, "register": FacetManage, "unregister": FacetManage,
	"subscribe": FacetManage, "unsubscribe": FacetManage, "install": FacetManage,
	"uninstall": FacetManage, "restart": FacetManage, "start": FacetManage,
	"stop": FacetManage, "enable": FacetManage, "disable": FacetManage,
	"session": FacetManage, "ping": FacetManage,
}

// destructiveVerbs name tokens whose effect has no in-band undo.
var destructiveVerbs = map[string]struct{}{
	"delete": {}, "remove": {}, "drop": {}, "purge": {}, "clear": {},
	"truncate": {}, "destroy": {}, "wipe": {}, "prune": {}, "reset": {},
	"revert": {}, "rollback": {}, "uninstall": {}, "overwrite": {}, "kill": {},
	"terminate": {}, "revoke": {},
}

// arbitraryVerbs name tokens that hand caller-supplied code to the server.
var arbitraryVerbs = map[string]struct{}{
	"run": {}, "exec": {}, "execute": {}, "eval": {}, "shell": {},
	"spawn": {}, "command": {}, "bash": {}, "sh": {}, "python": {},
	"script": {}, "system": {},
}

// arbitraryParams are input-schema property names that mean the caller supplies
// code. A tool named "notebook_cell" is not obviously an execution surface;
// a "code" parameter is.
var arbitraryParams = map[string]struct{}{
	"command": {}, "cmd": {}, "script": {}, "code": {}, "shell": {},
	"expression": {}, "eval": {}, "sql": {}, "program": {}, "source": {},
}

// searchParams are input-schema property names that mean the caller is looking
// for something they cannot name.
var searchParams = map[string]struct{}{
	"query": {}, "q": {}, "pattern": {}, "search": {}, "keywords": {},
	"filter": {}, "regex": {}, "term": {}, "prompt": {},
}

// writeParams are input-schema property names that carry a payload to persist.
var writeParams = map[string]struct{}{
	"content": {}, "body": {}, "data": {}, "value": {}, "payload": {},
	"text": {}, "contents": {}, "document": {}, "record": {}, "patch": {},
}

// ClassifyTool derives the facet and risk class for one tool.
//
// The order of signals is the design. Server annotations win outright because
// the server is describing itself; everything below them is inference, and an
// inference that contradicts a declaration is a bug, not a refinement. Name
// tokens beat analyzer capabilities because capabilities are produced by
// substring matching over the description, which routinely tags a read tool
// with /write because its description mentions the word "update".
//
// analysis may be nil: classification must work on a server discovered while
// the LLM analyzer was unavailable, otherwise the control plane goes dark
// exactly when the agent is already degraded.
func ClassifyTool(schema MCPToolSchema, analysis *ToolAnalysis) FacetClassification {
	tokens := nameTokens(schema.Name)
	props := schemaPropertyNames(schema.InputSchema)

	facet, facetSrc := deriveFacet(schema, analysis, tokens, props)
	risk, riskSrc := deriveRisk(schema, analysis, tokens, props, facet, facetSrc)

	return FacetClassification{
		Facet:       facet,
		Risk:        risk,
		FacetSource: facetSrc,
		RiskSource:  riskSrc,
	}
}

// deriveFacet picks the verb bucket.
func deriveFacet(schema MCPToolSchema, analysis *ToolAnalysis, tokens []string, props map[string]struct{}) (Facet, ClassificationSource) {
	readOnly := schema.Annotations.readOnly()

	// 1. Leading name token. A tool called "delete_branch" is a write no matter
	//    what else its description says.
	if len(tokens) > 0 {
		if f, ok := verbFacets[tokens[0]]; ok {
			if readOnly && (f == FacetWrite || f == FacetExecute) {
				// The server says read-only and the name says otherwise. Trust
				// the declaration for the side-effect question but keep the
				// name's shape: a read-only "run_report" is an analysis.
				return FacetAnalyze, SourceAnnotation
			}
			return f, SourceName
		}
	}

	// 2. Any other name token, so "repo_search_files" still lands in /search.
	for _, tok := range tokens[1:] {
		if f, ok := verbFacets[tok]; ok {
			if readOnly && (f == FacetWrite || f == FacetExecute) {
				continue
			}
			return f, SourceName
		}
	}

	// 3. Analyzer capabilities, narrowest first. /search before /read because a
	//    search tool almost always also reads.
	if analysis != nil {
		if hasCap(analysis.Capabilities, "/execute") && !readOnly {
			return FacetExecute, SourceCapability
		}
		if hasCap(analysis.Capabilities, "/search") {
			return FacetSearch, SourceCapability
		}
		if !readOnly && (hasCap(analysis.Capabilities, "/write") || hasCap(analysis.Capabilities, "/delete")) {
			return FacetWrite, SourceCapability
		}
		if hasCap(analysis.Capabilities, "/analyze") || hasCap(analysis.Capabilities, "/validate") ||
			hasCap(analysis.Capabilities, "/transform") {
			return FacetAnalyze, SourceCapability
		}
		if hasCap(analysis.Capabilities, "/read") {
			return FacetRead, SourceCapability
		}
	}

	// 4. Input schema shape.
	if anyKey(props, arbitraryParams) && !readOnly {
		return FacetExecute, SourceSchema
	}
	if anyKey(props, searchParams) {
		return FacetSearch, SourceSchema
	}
	if anyKey(props, writeParams) && !readOnly {
		return FacetWrite, SourceSchema
	}

	// 5. Nothing matched. Read is the conservative default for the facet
	//    because it is the tier the atlas discloses most cheaply; the risk
	//    class below is where conservatism actually costs something, and it
	//    defaults the other way.
	return FacetRead, SourceDefault
}

// deriveRisk picks the blast radius.
//
// facetSource is carried in because risk evidence must never be weaker than the
// evidence that produced the facet. Two cases depend on it, and both were found
// by test rather than by reading: a tool the name places firmly in /read should
// not be dragged to mutating by the analyzer's capability list, which is
// substring matching over a description and tags half the read tools /write for
// containing the word "updated"; and a tool where NOTHING matched must not
// inherit /read's benign default, because the classifier ran out of evidence
// rather than finding none.
func deriveRisk(schema MCPToolSchema, analysis *ToolAnalysis, tokens []string, props map[string]struct{}, facet Facet, facetSource ClassificationSource) (RiskClass, ClassificationSource) {
	// 1. Server declarations. destructiveHint is only meaningful on a tool that
	//    is not read-only, per the MCP spec, so read-only is checked first.
	if schema.Annotations.readOnly() {
		return RiskSafe, SourceAnnotation
	}
	if schema.Annotations.destructive() {
		return RiskDestructive, SourceAnnotation
	}

	// 2. Arbitrary code is the top of the ladder and can be signalled by either
	//    the name or a parameter, so both are checked before anything lower.
	for _, tok := range tokens {
		if _, ok := arbitraryVerbs[tok]; ok {
			return RiskArbitrary, SourceName
		}
	}
	if anyKey(props, arbitraryParams) {
		return RiskArbitrary, SourceSchema
	}

	// 3. Destructive verbs.
	for _, tok := range tokens {
		if _, ok := destructiveVerbs[tok]; ok {
			return RiskDestructive, SourceName
		}
	}
	if analysis != nil && hasCap(analysis.Capabilities, "/delete") {
		return RiskDestructive, SourceCapability
	}

	// 4. Facet implies the floor. An /execute tool that dodged every arbitrary
	//    signal above is still running something.
	switch facet {
	case FacetExecute:
		return RiskArbitrary, SourceName
	case FacetWrite:
		return RiskMutating, SourceName
	case FacetRead, FacetSearch, FacetAnalyze:
		// Nothing at all matched — the facet is a fallback, not a finding — so
		// there is no evidence this tool is read-only, only an absence of
		// evidence that it is not.
		if facetSource == SourceDefault {
			return RiskMutating, SourceDefault
		}
		// The capability list is consulted only when it is not being overruled
		// by a stronger signal. If the NAME put the tool in /read, a /write
		// capability inferred from prose is the weaker witness and loses.
		if facetSource != SourceName && analysis != nil && hasCap(analysis.Capabilities, "/write") {
			return RiskMutating, SourceCapability
		}
		return RiskSafe, facetSource
	case FacetManage:
		// Managing a connection changes state that outlives the call.
		return RiskMutating, SourceDefault
	}

	// 5. Unclassifiable. Default deny is the house rule, and the honest default
	//    for an unknown side effect is that there is one.
	return RiskMutating, SourceDefault
}

// nameTokens splits a tool name into lowercase word tokens, handling the three
// conventions MCP servers use interchangeably: snake_case, kebab-case, and
// camelCase. A server that names a tool "listPullRequests" must classify the
// same as one that names it "list_pull_requests".
func nameTokens(name string) []string {
	if name == "" {
		return nil
	}

	var out []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, strings.ToLower(cur.String()))
			cur.Reset()
		}
	}

	runes := []rune(name)
	for i, r := range runes {
		switch {
		case r == '_' || r == '-' || r == '.' || r == '/' || r == ':' || r == ' ':
			flush()
		case unicode.IsUpper(r):
			// Split before an uppercase run that starts a new word, and before
			// the last capital of an acronym followed by a lowercase word
			// ("HTTPRequest" -> "http", "request").
			prevLower := i > 0 && (unicode.IsLower(runes[i-1]) || unicode.IsDigit(runes[i-1]))
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			prevUpper := i > 0 && unicode.IsUpper(runes[i-1])
			if prevLower || (prevUpper && nextLower) {
				flush()
			}
			cur.WriteRune(unicode.ToLower(r))
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return out
}

// schemaPropertyNames extracts the top-level property names of a JSON Schema
// object. A malformed or absent schema yields an empty set rather than an
// error: schema shape is one signal among several, and a server that ships a
// broken schema should still get classified.
func schemaPropertyNames(raw json.RawMessage) map[string]struct{} {
	out := make(map[string]struct{})
	if len(raw) == 0 {
		return out
	}
	var doc struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return out
	}
	for name := range doc.Properties {
		out[strings.ToLower(name)] = struct{}{}
	}
	return out
}

func anyKey(have map[string]struct{}, want map[string]struct{}) bool {
	for k := range want {
		if _, ok := have[k]; ok {
			return true
		}
	}
	return false
}

func hasCap(caps []string, want string) bool {
	for _, c := range caps {
		if strings.EqualFold(strings.TrimSpace(c), want) {
			return true
		}
	}
	return false
}

// FacetCensus counts tools per facet for one server. It is the row the atlas
// renders, and the reason a fleet of servers costs a few hundred tokens to
// describe rather than a few thousand.
type FacetCensus struct {
	Facet Facet     `json:"facet"`
	Count int       `json:"count"`
	Risk  RiskClass `json:"max_risk"`
	// Sample names a few representative tools so the atlas is navigable
	// without a second call. Bounded hard — see atlasSampleSize.
	Sample []string `json:"sample,omitzero"`
}

// atlasSampleSize bounds how many tool names one atlas row names. Three is
// enough to tell a filesystem server from a Jira server at a glance and small
// enough that a 40-tool facet costs the same as a 4-tool one.
const atlasSampleSize = 3

// CensusFor buckets tools into facet rows in AllFacets order. Empty facets are
// omitted: an atlas that lists what a server cannot do is paying tokens for
// nothing.
func CensusFor(tools []*MCPTool) []FacetCensus {
	byFacet := make(map[Facet][]*MCPTool, len(AllFacets))
	for _, t := range tools {
		if t == nil {
			continue
		}
		f := t.Facet
		if !f.Valid() {
			f = FacetRead
		}
		byFacet[f] = append(byFacet[f], t)
	}

	out := make([]FacetCensus, 0, len(AllFacets))
	for _, f := range AllFacets {
		bucket := byFacet[f]
		if len(bucket) == 0 {
			continue
		}
		// Stable order so the atlas does not churn between turns; an atlas that
		// reorders itself looks like new information and is not.
		sort.Slice(bucket, func(i, j int) bool { return bucket[i].Name < bucket[j].Name })

		row := FacetCensus{Facet: f, Count: len(bucket), Risk: RiskSafe}
		for i, t := range bucket {
			if t.Risk.Rank() > row.Risk.Rank() {
				row.Risk = t.Risk
			}
			if i < atlasSampleSize {
				row.Sample = append(row.Sample, t.Name)
			}
		}
		out = append(out, row)
	}
	return out
}
