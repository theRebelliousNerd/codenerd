package init

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"codenerd/internal/config"
	"codenerd/internal/orient"
	"codenerd/internal/prompt"
	"codenerd/internal/store"
	"codenerd/internal/types"

	"gopkg.in/yaml.v3"
)

// The ecosystem seed is how phase 6 hands derived knowledge to phase 7a
// without a new field on Initializer (that struct is shared with another
// lane). A missing workspace entry means "this init did not derive a set",
// so knowledge-base creation keeps the agent's own topics. A present entry,
// even with an empty topic list, means the policy named every topic that
// still needs research.
type ecosystemAtom struct {
	Concept string
	Content string
	Path    string
	Tool    string
	Kind    string
}

type ecosystemWorkspace struct {
	researchSet map[string]bool
	research    map[string][]string
	knowledge   map[string][]ecosystemAtom
	prompts     map[string]string
}

var (
	ecosystemMu    sync.Mutex
	ecosystemSeeds = map[string]*ecosystemWorkspace{}
)

func ecosystemWorkspaceKey(ws string) string {
	if strings.TrimSpace(ws) == "" {
		return ""
	}
	return filepath.Clean(ws)
}

func clearEcosystemSeed(ws string) {
	key := ecosystemWorkspaceKey(ws)
	if key == "" {
		return
	}
	ecosystemMu.Lock()
	delete(ecosystemSeeds, key)
	ecosystemMu.Unlock()
}

func publishEcosystemSeed(ws string, seed *ecosystemWorkspace) {
	key := ecosystemWorkspaceKey(ws)
	if key == "" || seed == nil {
		return
	}
	ecosystemMu.Lock()
	ecosystemSeeds[key] = seed
	ecosystemMu.Unlock()
}

func ecosystemResearchTopics(ws, name string) ([]string, bool) {
	key := ecosystemWorkspaceKey(ws)
	ecosystemMu.Lock()
	defer ecosystemMu.Unlock()
	seed := ecosystemSeeds[key]
	if seed == nil || !seed.researchSet[name] {
		return nil, false
	}
	return append([]string{}, seed.research[name]...), true
}

func ecosystemKnowledgeChunks(ws, name string) []ecosystemAtom {
	key := ecosystemWorkspaceKey(ws)
	ecosystemMu.Lock()
	defer ecosystemMu.Unlock()
	seed := ecosystemSeeds[key]
	if seed == nil {
		return nil
	}
	return append([]ecosystemAtom{}, seed.knowledge[name]...)
}

func ecosystemImportedPrompt(ws, name string) string {
	key := ecosystemWorkspaceKey(ws)
	ecosystemMu.Lock()
	defer ecosystemMu.Unlock()
	seed := ecosystemSeeds[key]
	if seed == nil {
		return ""
	}
	return seed.prompts[name]
}

// integrateEcosystem consumes the already measured orientation and profile
// signals, and remembers winning sources for knowledge-base creation.
// An empty workspace is left alone: callers that only ask which profile
// agents a language needs must not walk the process's current directory.
// A failure here keeps the profile recommendation; it does not invent a
// second list in Go.
func (i *Initializer) integrateEcosystem(ctx context.Context, result *InitResult, profile ProjectProfile, recommended []RecommendedAgent) []RecommendedAgent {
	ws := ecosystemWorkspaceKey(i.config.Workspace)
	if ws == "" {
		return recommended
	}
	clearEcosystemSeed(ws)

	eng := i.orientation
	if eng == nil || i.orientationSnapshot == nil {
		result.Failures = append(result.Failures, "agent derivation requires the orientation phase")
		return recommended
	}
	sources := i.orientationSnapshot.Sources
	signals := profileFacts(profile)
	if err := eng.Assert(signals); err != nil {
		result.Failures = append(result.Failures, "profile orientation facts: "+err.Error())
		return recommended
	}
	i.orientationSnapshot.Facts = append(i.orientationSnapshot.Facts, signals...)
	if err := eng.Evaluate(ctx); err != nil {
		result.Failures = append(result.Failures, "profile orientation evaluation: "+err.Error())
		return recommended
	}
	if err := i.orientationSnapshot.Save(ws, eng); err != nil {
		result.Failures = append(result.Failures, "profile orientation persistence: "+err.Error())
	}
	warnEcosystemThresholds(eng, result)

	derived, err := assembleAgents(eng)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("agent ecosystem results were not read: %v", err))
		return recommended
	}
	winners, knowledge, losers, researchRows, paramRows, err := ecosystemRows(eng)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("agent ecosystem results were not read: %v", err))
		return recommended
	}

	researchSet := map[string]bool{}
	research := map[string][]string{}
	for _, agent := range derived {
		researchSet[agent.Name] = true
	}
	for _, fact := range researchRows {
		if len(fact.Args) < 2 {
			continue
		}
		name := types.ExtractString(fact.Args[0])
		topic := types.ExtractString(fact.Args[1])
		if name == "" || topic == "" {
			continue
		}
		research[name] = append(research[name], topic)
	}
	for name := range researchSet {
		research[name] = uniqueSorted(research[name])
	}

	limit := knowledgeChunkRunes(paramRows)
	byID := map[string]orient.Source{}
	for _, src := range sources {
		byID[src.ID] = src
	}
	knowledgeByAgent := map[string][]ecosystemAtom{}
	for _, fact := range knowledge {
		if len(fact.Args) < 2 {
			continue
		}
		name := types.ExtractString(fact.Args[0])
		id := types.ExtractString(fact.Args[1])
		src, ok := byID[id]
		if !ok {
			continue
		}
		knowledgeByAgent[name] = append(knowledgeByAgent[name], ecosystemKnowledgeAtoms(src, limit)...)
	}
	promptRows, err := eng.Query("orient_agent_prompt")
	if err != nil {
		result.Failures = append(result.Failures, "orientation prompt selection: "+err.Error())
		return recommended
	}
	prompts := importedPrompts(sources, promptRows)

	merged := mergeDerivedAgents(recommended, derived, profile.Language)
	atoms := ecosystemPromptAtoms(sources, winners)
	reportPath := filepath.Join(ws, ".nerd", "orientation", "agents.md")
	if err := writeOrientationAgents(reportPath, merged, research, sources, winners, knowledge, losers, atoms); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("orientation agent report was not written: %v", err))
	} else {
		result.FilesCreated = append(result.FilesCreated, reportPath)
	}
	i.persistEcosystemAtoms(ctx, atoms, result)
	publishEcosystemSeed(ws, &ecosystemWorkspace{
		researchSet: researchSet,
		research:    research,
		knowledge:   knowledgeByAgent,
		prompts:     prompts,
	})
	return merged
}

func warnEcosystemThresholds(eng *orient.Engine, result *InitResult) {
	missing, err := eng.Query("config_param_missing")
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("orient missing-threshold rows were not read: %v", err))
		return
	}
	for _, fact := range missing {
		if len(fact.Args) < 2 {
			continue
		}
		result.Warnings = append(result.Warnings, fmt.Sprintf("config_param_missing(%s, %s)", types.ExtractString(fact.Args[0]), types.ExtractString(fact.Args[1])))
	}
}

// knowledgeChunkRunes is the optional /orient_knowledge_chunk_runes row.
// Absent or non-positive means a paragraph is stored whole: there is no
// hidden chunk size in Go.
func knowledgeChunkRunes(rows []types.Fact) int {
	for _, fact := range rows {
		if len(fact.Args) < 2 || types.ExtractString(fact.Args[0]) != "/orient_knowledge_chunk_runes" {
			continue
		}
		n, ok := factNumber(fact.Args[1])
		if !ok || n <= 0 {
			return 0
		}
		return int(n)
	}
	return 0
}

func factNumber(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case float64:
		return int64(n), true
	case float32:
		return int64(n), true
	default:
		return 0, false
	}
}

func ecosystemRows(eng *orient.Engine) (winners map[string]string, knowledge, losers, research, params []types.Fact, err error) {
	winnerRows, err := eng.Query("agent_source_winner")
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	winners = map[string]string{}
	for _, fact := range winnerRows {
		if len(fact.Args) < 2 {
			continue
		}
		winners[types.ExtractString(fact.Args[0])] = types.ExtractString(fact.Args[1])
	}
	if knowledge, err = eng.Query("orient_agent_knowledge"); err != nil {
		return nil, nil, nil, nil, nil, err
	}
	if losers, err = eng.Query("agent_source_loser"); err != nil {
		return nil, nil, nil, nil, nil, err
	}
	if research, err = eng.Query("orient_research_topic"); err != nil {
		return nil, nil, nil, nil, nil, err
	}
	if params, err = eng.Query("config_param"); err != nil {
		return nil, nil, nil, nil, nil, err
	}
	return winners, knowledge, losers, research, params, nil
}

// agentsFromProfile is the profile half of determineRequiredAgents. The
// language, framework and dependency table lives in the policy; this only
// asserts the signals and reads the agents back.
func agentsFromProfile(ctx context.Context, profile ProjectProfile) ([]RecommendedAgent, error) {
	cfg := config.DefaultOrientConfig()
	eng, err := orient.NewEngine(&cfg)
	if err != nil {
		return nil, err
	}
	defer eng.Close()
	if err := eng.Assert(profileFacts(profile)); err != nil {
		return nil, err
	}
	if err := eng.Evaluate(ctx); err != nil {
		return nil, err
	}
	return assembleAgents(eng)
}

func assembleAgents(eng *orient.Engine) ([]RecommendedAgent, error) {
	agents, err := eng.Query("orient_agent")
	if err != nil {
		return nil, err
	}
	topics, err := eng.Query("orient_agent_topic")
	if err != nil {
		return nil, err
	}
	descs, err := eng.Query("orient_agent_description")
	if err != nil {
		return nil, err
	}
	perms, err := eng.Query("orient_agent_permission")
	if err != nil {
		return nil, err
	}
	pris, err := eng.Query("orient_agent_priority")
	if err != nil {
		return nil, err
	}
	return buildAgents(agents, topics, descs, perms, pris), nil
}

type agentBag struct {
	whys, topics, descs, perms []string
	priority                   int
	havePriority               bool
}

func buildAgents(agentRows, topicRows, descRows, permRows, priRows []types.Fact) []RecommendedAgent {
	bags := map[string]*agentBag{}
	ensure := func(name string) *agentBag {
		bag := bags[name]
		if bag == nil {
			bag = &agentBag{}
			bags[name] = bag
		}
		return bag
	}
	for _, fact := range agentRows {
		if len(fact.Args) < 2 {
			continue
		}
		name := types.ExtractString(fact.Args[0])
		if name == "" {
			continue
		}
		ensure(name).whys = append(ensure(name).whys, types.ExtractString(fact.Args[1]))
	}
	for _, fact := range topicRows {
		if len(fact.Args) < 2 {
			continue
		}
		ensure(types.ExtractString(fact.Args[0])).topics = append(ensure(types.ExtractString(fact.Args[0])).topics, types.ExtractString(fact.Args[1]))
	}
	for _, fact := range descRows {
		if len(fact.Args) < 2 {
			continue
		}
		ensure(types.ExtractString(fact.Args[0])).descs = append(ensure(types.ExtractString(fact.Args[0])).descs, types.ExtractString(fact.Args[1]))
	}
	for _, fact := range permRows {
		if len(fact.Args) < 2 {
			continue
		}
		ensure(types.ExtractString(fact.Args[0])).perms = append(ensure(types.ExtractString(fact.Args[0])).perms, types.ExtractString(fact.Args[1]))
	}
	for _, fact := range priRows {
		if len(fact.Args) < 2 {
			continue
		}
		n, ok := factNumber(fact.Args[1])
		if !ok {
			continue
		}
		bag := ensure(types.ExtractString(fact.Args[0]))
		if !bag.havePriority || n > int64(bag.priority) {
			bag.priority = int(n)
			bag.havePriority = true
		}
	}

	names := make([]string, 0, len(bags))
	for name := range bags {
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	agents := make([]RecommendedAgent, 0, len(names))
	for _, name := range names {
		bag := bags[name]
		whys := uniqueSorted(bag.whys)
		agents = append(agents, RecommendedAgent{
			Name:        name,
			Type:        "persistent",
			Description: agentDescription(name, bag.descs, whys),
			Topics:      uniqueSorted(bag.topics),
			Permissions: uniqueSorted(bag.perms),
			Priority:    bag.priority,
			Reason:      strings.Join(whys, "; "),
		})
	}
	sort.Slice(agents, func(i, j int) bool {
		if agents[i].Priority != agents[j].Priority {
			return agents[i].Priority > agents[j].Priority
		}
		return agents[i].Name < agents[j].Name
	})
	return agents
}

func agentDescription(name string, descs, whys []string) string {
	descs = uniqueSorted(descs)
	if len(descs) > 0 {
		return strings.Join(descs, "; ")
	}
	if len(whys) == 1 && whys[0] == "skill cluster" {
		return "Skill cluster for " + name
	}
	return strings.Join(whys, "; ")
}

func mergeDerivedAgents(recommended, derived []RecommendedAgent, language string) []RecommendedAgent {
	have := map[string]bool{}
	for _, agent := range recommended {
		have[agent.Name] = true
	}
	for _, agent := range derived {
		if have[agent.Name] {
			continue
		}
		tools, prefs := GetToolsForAgentType(agent.Name, language)
		agent.Tools = tools
		agent.ToolPreferences = prefs
		recommended = append(recommended, agent)
		have[agent.Name] = true
	}
	return recommended
}

// profileFacts is the measurement the catalog joins. Keys are lowercased
// for language and framework because the catalog's keys are. Dependency
// names stay as declared: the catalog matches the module name, not a
// folded guess, and not a module path. Empty and repeated names are not
// signals. Per-module languages are not signals; the flat profile is.
func profileFacts(profile ProjectProfile) []types.Fact {
	var facts []types.Fact
	if lang := strings.ToLower(strings.TrimSpace(profile.Language)); lang != "" {
		facts = append(facts, types.Fact{Predicate: "profile_signal", Args: []any{types.MangleAtom("/language"), lang}})
	}
	if framework := strings.ToLower(strings.TrimSpace(profile.Framework)); framework != "" {
		facts = append(facts, types.Fact{Predicate: "profile_signal", Args: []any{types.MangleAtom("/framework"), framework}})
	}
	seen := map[string]bool{}
	for _, dep := range profile.Dependencies {
		name := strings.TrimSpace(dep.Name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		facts = append(facts, types.Fact{Predicate: "profile_signal", Args: []any{types.MangleAtom("/dependency"), name}})
	}
	return facts
}

func importedPrompts(sources []orient.Source, rows []types.Fact) map[string]string {
	byID := map[string]orient.Source{}
	for _, source := range sources {
		byID[source.ID] = source
	}
	out := map[string]string{}
	for _, row := range rows {
		if len(row.Args) != 2 {
			continue
		}
		source, ok := byID[types.ExtractString(row.Args[1])]
		if ok {
			out[types.ExtractString(row.Args[0])] = source.Prompt
		}
	}
	return out
}

func ecosystemKnowledgeAtoms(src orient.Source, runeLimit int) []ecosystemAtom {
	var out []ecosystemAtom
	if y := frontmatterYAML(src.Frontmatter); y != "" {
		out = append(out, ecosystemAtom{
			Concept: "ecosystem frontmatter",
			Content: y,
			Path:    src.Path,
			Tool:    src.Tool,
			Kind:    src.Kind,
		})
	}
	parts := ecosystemBodyParts(src.Body, runeLimit)
	for i, part := range parts {
		var concept string
		if runeLimit <= 0 {
			concept = fmt.Sprintf("ecosystem paragraph %d/%d tool=%s path=%s bytes=%d", i+1, len(parts), src.Tool, src.Path, len(part))
		} else {
			concept = fmt.Sprintf("ecosystem page %d/%d runes=%d tool=%s path=%s bytes=%d", i+1, len(parts), runeLimit, src.Tool, src.Path, len(part))
		}
		out = append(out, ecosystemAtom{
			Concept: concept,
			Content: part,
			Path:    src.Path,
			Tool:    src.Tool,
			Kind:    src.Kind,
		})
	}
	return out
}

// ecosystemBodyParts splits a body for storage. A non-positive limit keeps
// every paragraph, including empty ones, so joining with a blank line
// returns the body. A positive limit pages by runes and the pages
// concatenate back to the body. Nothing is dropped.
func ecosystemBodyParts(body string, runeLimit int) []string {
	if body == "" {
		return nil
	}
	if runeLimit <= 0 {
		return strings.Split(body, "\n\n")
	}
	runes := []rune(body)
	parts := make([]string, 0, (len(runes)+runeLimit-1)/runeLimit)
	for start := 0; start < len(runes); start += runeLimit {
		end := start + runeLimit
		if end > len(runes) {
			end = len(runes)
		}
		parts = append(parts, string(runes[start:end]))
	}
	return parts
}

func ecosystemPromptAtoms(sources []orient.Source, winners map[string]string) []*store.PromptAtom {
	used := map[string]struct{}{}
	var atoms []*store.PromptAtom
	ordered := append([]orient.Source{}, sources...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	for _, src := range ordered {
		if _, ok := winners[src.ID]; !ok {
			continue
		}
		if src.Kind != "rule" && src.Kind != "instructions" && src.Kind != "command" {
			continue
		}
		if src.Body == "" && len(src.Frontmatter) == 0 {
			continue
		}
		id := ecosystemAtomID(src, used)
		content := ecosystemAtomContent(src)
		atoms = append(atoms, &store.PromptAtom{
			AtomID:      id,
			Version:     1,
			Content:     content,
			TokenCount:  estimateTokens(content),
			ContentHash: computeContentHash(id, content),
			Category:    string(prompt.CategoryDomain),
			Subcategory: scopeLabel(src),
			Priority:    60,
			IsMandatory: false,
		})
	}
	return atoms
}

func ecosystemAtomID(src orient.Source, used map[string]struct{}) string {
	base := fmt.Sprintf("project/ecosystem/%s/%s/%s", sanitizeForMangle(src.Tool), sanitizeForMangle(src.Kind), sanitizeForMangle(src.Name))
	id := base
	if _, ok := used[id]; ok {
		id = base + "/" + sanitizeForMangle(src.Path)
	}
	if _, ok := used[id]; ok {
		for n := 2; ; n++ {
			next := fmt.Sprintf("%s/%d", id, n)
			if _, taken := used[next]; !taken {
				id = next
				break
			}
		}
	}
	used[id] = struct{}{}
	return id
}

func scopeLabel(src orient.Source) string {
	scopes := src.Scopes
	if len(scopes) == 0 && src.ScopeDir != "" {
		scopes = []string{src.ScopeDir}
	}
	scopes = uniqueSorted(scopes)
	if len(scopes) == 0 || (len(scopes) == 1 && scopes[0] == ".") {
		return "repository"
	}
	return strings.Join(scopes, ",")
}

func ecosystemAtomContent(src orient.Source) string {
	var b strings.Builder
	fmt.Fprintf(&b, "source: %s\ntool: %s\nkind: %s\nscope: %s\n", src.Path, src.Tool, src.Kind, scopeLabel(src))
	if y := frontmatterYAML(src.Frontmatter); y != "" {
		b.WriteString(y)
		if !strings.HasSuffix(y, "\n") {
			b.WriteByte('\n')
		}
	}
	b.WriteString(src.Body)
	return b.String()
}

// frontmatterYAML writes every key, sorted, so an unknown key is not
// dropped on the way into a prompt atom or a knowledge atom. A value that
// cannot be encoded is written with its Go representation rather than
// omitted.
func frontmatterYAML(fm map[string]any) string {
	if len(fm) == 0 {
		return ""
	}
	keys := make([]string, 0, len(fm))
	for key := range fm {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	node := &yaml.Node{Kind: yaml.MappingNode}
	for _, key := range keys {
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
		var val yaml.Node
		if err := val.Encode(fm[key]); err != nil {
			val = yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%#v", fm[key])}
		}
		node.Content = append(node.Content, keyNode, &val)
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(node); err != nil {
		return fmt.Sprintf("%#v\n", fm)
	}
	_ = enc.Close()
	return buf.String()
}

func (i *Initializer) persistEcosystemAtoms(ctx context.Context, atoms []*store.PromptAtom, result *InitResult) {
	if len(atoms) == 0 {
		return
	}
	i.mu.Lock()
	i.projectAtoms = append(i.projectAtoms, atoms...)
	i.mu.Unlock()
	if i.localDB != nil {
		for _, atom := range atoms {
			if err := i.localDB.StorePromptAtom(atom); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("prompt atom %s was not stored: %v", atom.AtomID, err))
			}
		}
	}
	// Phase 5c ingests project atoms before phase 6 runs, so a corpus that
	// already exists will not see these unless they are stored here too.
	corpus := filepath.Join(i.config.Workspace, ".nerd", "prompts", "corpus.db")
	if _, err := os.Stat(corpus); err != nil {
		return
	}
	db, err := sql.Open("sqlite3", corpus)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("prompt corpus was not opened: %v", err))
		return
	}
	defer db.Close()
	loader := prompt.NewAtomLoader(i.embedEngine)
	for _, atom := range atoms {
		if err := loader.StoreAtom(ctx, db, projectAtomToPromptAtom(atom)); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("prompt atom %s was not ingested: %v", atom.AtomID, err))
		}
	}
}

func writeOrientationAgents(path string, agents []RecommendedAgent, research map[string][]string, sources []orient.Source, winners map[string]string, knowledge, losers []types.Fact, atoms []*store.PromptAtom) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	byID := map[string]orient.Source{}
	for _, src := range sources {
		byID[src.ID] = src
	}
	linked := map[string][]string{}
	linkedAny := map[string]bool{}
	for _, fact := range knowledge {
		if len(fact.Args) < 2 {
			continue
		}
		name := types.ExtractString(fact.Args[0])
		id := types.ExtractString(fact.Args[1])
		linked[name] = append(linked[name], id)
		linkedAny[id] = true
	}
	for name := range linked {
		sort.Strings(linked[name])
	}

	var b strings.Builder
	b.WriteString("# Orientation agents\n\n")
	b.WriteString("Shard agents derived from the repository's agent corpus and project profile. Source bodies are stored in the knowledge bases and prompt atoms; this file records the decision.\n\n")
	b.WriteString("## Agents\n\n")
	if len(agents) == 0 {
		b.WriteString("None.\n\n")
	}
	for _, agent := range agents {
		fmt.Fprintf(&b, "### %s\n\n", agent.Name)
		fmt.Fprintf(&b, "- Why: %s\n", agent.Reason)
		fmt.Fprintf(&b, "- Priority: %d\n", agent.Priority)
		topics := research[agent.Name]
		if len(topics) == 0 {
			b.WriteString("- Research topics: none\n")
		} else {
			fmt.Fprintf(&b, "- Research topics: %s\n", strings.Join(topics, ", "))
		}
		b.WriteString("- Sources:\n")
		ids := linked[agent.Name]
		if len(ids) == 0 {
			b.WriteString("  - none\n")
		}
		for _, id := range ids {
			src, ok := byID[id]
			if !ok {
				fmt.Fprintf(&b, "  - %s\n", id)
				continue
			}
			fmt.Fprintf(&b, "  - `%s` tool=%s kind=%s winner=%s\n", src.Path, src.Tool, src.Kind, winners[id])
		}
		b.WriteString("\n")
	}

	b.WriteString("## Duplicates that lost\n\n")
	if len(losers) == 0 {
		b.WriteString("None.\n\n")
	} else {
		lines := make([]string, 0, len(losers))
		for _, fact := range losers {
			if len(fact.Args) < 3 {
				continue
			}
			loserID := types.ExtractString(fact.Args[0])
			winnerID := types.ExtractString(fact.Args[1])
			why := types.ExtractString(fact.Args[2])
			loser := byID[loserID]
			winner := byID[winnerID]
			loserPath := loserID
			winnerPath := winnerID
			if loser.Path != "" {
				loserPath = loser.Path
			}
			if winner.Path != "" {
				winnerPath = winner.Path
			}
			lines = append(lines, fmt.Sprintf("- `%s` lost to `%s` because %s", loserPath, winnerPath, why))
		}
		sort.Strings(lines)
		for _, line := range lines {
			b.WriteString(line)
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}

	b.WriteString("## Prompt atoms\n\n")
	if len(atoms) == 0 {
		b.WriteString("None.\n\n")
	}
	for _, atom := range atoms {
		fmt.Fprintf(&b, "- `%s`\n", atom.AtomID)
		if atom.Content != "" {
			if line, ok := strings.CutPrefix(atom.Content, "source: "); ok {
				pathLine, _, _ := strings.Cut(line, "\n")
				fmt.Fprintf(&b, "  - source: `%s`\n", pathLine)
			}
		}
	}
	if len(atoms) > 0 {
		b.WriteByte('\n')
	}

	b.WriteString("## Sources not attached\n\n")
	var unattached []string
	for _, src := range sources {
		switch src.Kind {
		case "journal":
			unattached = append(unattached, fmt.Sprintf("- `%s` tool=%s kind=%s", src.Path, src.Tool, src.Kind))
		case "skill", "memory", "rule":
			if _, win := winners[src.ID]; win && !linkedAny[src.ID] {
				unattached = append(unattached, fmt.Sprintf("- `%s` tool=%s kind=%s winner=%s", src.Path, src.Tool, src.Kind, winners[src.ID]))
			}
		}
	}
	sort.Strings(unattached)
	if len(unattached) == 0 {
		b.WriteString("None.\n")
	} else {
		for _, line := range unattached {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func uniqueSorted(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
