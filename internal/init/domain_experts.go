package init

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"codenerd/internal/atomicfile"
	"codenerd/internal/northstar"
	"codenerd/internal/orient"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// Vision-derived domain experts.
//
// The stack catalog (orient/ecosystem_agents.mg) knows languages, frameworks
// and libraries; it cannot know what the project is trying to build. After
// orientation has derived the north star, read the strategic documents and
// measured the stack, the planner client proposes the domain experts that
// goal needs: each with a mission, a reason cited to the north star or an
// oriented document, the topics and failure modes it covers, and the research
// questions its knowledge base answers by grounded web search. The answer is
// validated here and asserted as vision_agent facts; orient/vision_agents.mg
// merges them into the roster with the stack experts.

// coreAgentNames are the experts the policy always considers and keeps on
// evidence; a proposal may support them but not redefine them.
var coreAgentNames = []string{"SecurityAuditor", "TestArchitect"}

var domainExpertName = regexp.MustCompile(`^[A-Z][A-Za-z0-9]{2,47}$`)

// domainProposal is the validated planner answer.
type domainProposal struct {
	Experts []domainExpert `json:"experts"`
	Core    []coreSupport  `json:"core,omitempty"`
}

type domainExpert struct {
	Name         string           `json:"name"`
	Mission      string           `json:"mission"`
	Why          string           `json:"why"`
	Cites        []string         `json:"cites"`
	Topics       []string         `json:"topics,omitempty"`
	FailureModes []string         `json:"failure_modes,omitempty"`
	Research     []domainQuestion `json:"research,omitempty"`
}

type domainQuestion struct {
	Question   string `json:"question"`
	FastMoving bool   `json:"fast_moving,omitempty"`
}

type coreSupport struct {
	Name  string   `json:"name"`
	Why   string   `json:"why"`
	Cites []string `json:"cites"`
}

// domainExpertsRecord is .nerd/orientation/domain_experts.json: the last
// validated proposal and the digest of the request it answered, so an init
// whose north star, documents, stack and bounds are unchanged reuses it.
type domainExpertsRecord struct {
	RequestDigest string         `json:"request_digest"`
	ChosenBy      string         `json:"chosen_by"`
	Proposal      domainProposal `json:"proposal"`
}

// domainBounds are the orient_param rows that bound a proposal.
type domainBounds struct {
	maxExperts int
	maxTopics  int
	staleDays  int
}

func readDomainBounds(eng *orient.Engine) (domainBounds, error) {
	rows, err := eng.Query("orient_param")
	if err != nil {
		return domainBounds{}, err
	}
	bounds := domainBounds{}
	for _, row := range rows {
		if len(row.Args) < 2 {
			continue
		}
		n, ok := factNumber(row.Args[1])
		if !ok {
			continue
		}
		switch types.ExtractString(row.Args[0]) {
		case "/orient_domain_agent_max":
			bounds.maxExperts = int(n)
		case "/orient_domain_agent_topic_max":
			bounds.maxTopics = int(n)
		case "/orient_research_stale_days":
			bounds.staleDays = int(n)
		}
	}
	return bounds, nil
}

// proposeDomainExperts asks the planner route for the domain-expert roster,
// reusing the last answer when the request is unchanged, and returns the
// facts to assert with notes for the init warnings. No client, no north star
// and no oriented documents all mean no proposal; the stack catalog stands.
func (i *Initializer) proposeDomainExperts(ctx context.Context, eng *orient.Engine, profile ProjectProfile, stack []RecommendedAgent) ([]types.Fact, []string, error) {
	bounds, err := readDomainBounds(eng)
	if err != nil {
		return nil, nil, err
	}
	if bounds.maxExperts <= 0 {
		return nil, nil, nil
	}
	sources, err := domainEvidence(eng, i.orientationSnapshot)
	if err != nil {
		return nil, nil, err
	}
	if i.orientedVision == nil && len(sources) == 0 {
		return nil, []string{"No north star and no oriented documents: the roster is the stack catalog alone"}, nil
	}
	task := domainExpertsTask(i.orientedVision, sources, profile, stack, bounds)
	digest := sha256.Sum256([]byte(task))
	requestDigest := hex.EncodeToString(digest[:])
	nerdDir := filepath.Join(i.config.Workspace, ".nerd")

	var notes []string
	previous, err := loadDomainExpertsRecord(nerdDir)
	if err != nil {
		notes = append(notes, err.Error())
	}
	var proposal domainProposal
	switch {
	case previous != nil && previous.RequestDigest == requestDigest:
		proposal = previous.Proposal
	case i.route(slotPlanner).client == nil:
		return nil, append(notes, "No LLM client to propose domain experts from the north star"), nil
	default:
		route := i.route(slotPlanner)
		response, err := i.withJITPrompt(ctx, initCall{slot: slotPlanner, phase: "domain_experts", task: task, profile: &profile},
			func(ctx context.Context, client perception.LLMClient, prompt string) (string, error) {
				return client.Complete(ctx, prompt)
			})
		if err != nil {
			return nil, append(notes, fmt.Sprintf("Domain experts were not proposed: %v", err)), nil
		}
		var rejected []string
		proposal, rejected, err = parseDomainProposal(response, sources, i.orientedVision != nil, stack, bounds)
		if err != nil {
			return nil, append(notes, fmt.Sprintf("Domain-expert proposal rejected: %v", err)), nil
		}
		notes = append(notes, rejected...)
		record := domainExpertsRecord{
			RequestDigest: requestDigest,
			ChosenBy:      string(route.slot) + " client " + formatLLMIdentity(route.provider, route.model),
			Proposal:      proposal,
		}
		if err := saveDomainExpertsRecord(nerdDir, record); err != nil {
			notes = append(notes, fmt.Sprintf("Domain experts were not recorded for the next init: %v", err))
		}
	}
	return proposal.facts(), notes, nil
}

// domainEvidence lists what a citation may name: every oriented document,
// with the role claims and themes orientation gave it.
func domainEvidence(eng *orient.Engine, snapshot *orient.Snapshot) (map[string][]string, error) {
	evidence := map[string][]string{}
	if snapshot == nil {
		return evidence, nil
	}
	rows, err := eng.Query("orient_read_candidate")
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		path := types.ExtractString(row.Args[0])
		if _, ok := snapshot.Bodies[path]; ok {
			evidence[path] = nil
		}
	}
	for _, predicate := range []string{"doc_role_claim", "doc_theme"} {
		rows, err := eng.Query(predicate)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if len(row.Args) < 2 {
				continue
			}
			path := types.ExtractString(row.Args[0])
			if _, ok := evidence[path]; !ok {
				continue
			}
			value := strings.TrimPrefix(types.ExtractString(row.Args[1]), "/")
			if predicate == "doc_role_claim" {
				value = "role " + value
			}
			evidence[path] = append(evidence[path], value)
		}
	}
	return evidence, nil
}

// visionLines renders the north star of record for the proposal request.
func visionLines(vision *northstar.Vision) []string {
	if vision == nil {
		return nil
	}
	var lines []string
	add := func(label, value string) {
		if value = strings.TrimSpace(value); value != "" {
			lines = append(lines, label+": "+value)
		}
	}
	add("Mission", vision.Mission)
	add("Problem", vision.Problem)
	add("Vision", vision.VisionStmt)
	for _, capability := range vision.Capabilities {
		add("Capability", capability.Description)
	}
	for _, risk := range vision.Risks {
		add("Risk", risk.Description)
	}
	for _, requirement := range vision.Requirements {
		add("Requirement", requirement.Description)
	}
	for _, constraint := range vision.Constraints {
		add("Constraint", constraint)
	}
	return lines
}

func domainExpertsTask(vision *northstar.Vision, sources map[string][]string, profile ProjectProfile, stack []RecommendedAgent, bounds domainBounds) string {
	var b strings.Builder
	b.WriteString("Propose the domain experts this project needs for what it is trying to build.\n\n")
	b.WriteString("A domain expert is a specialist agent with its own knowledge base. The stack experts below already cover the languages and libraries; propose experts on the problem domain, its failure modes, the risks the north star names, and the practices that keep this kind of system working.\n\n")
	b.WriteString("## North star\n")
	if lines := visionLines(vision); len(lines) > 0 {
		for _, line := range lines {
			b.WriteString(line + "\n")
		}
	} else {
		b.WriteString("(none recorded)\n")
	}
	b.WriteString("\n## Oriented documents (cite these paths)\n")
	paths := make([]string, 0, len(sources))
	for path := range sources {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		claims := append([]string(nil), sources[path]...)
		sort.Strings(claims)
		fmt.Fprintf(&b, "- %s", path)
		if len(claims) > 0 {
			fmt.Fprintf(&b, ": %s", strings.Join(claims, "; "))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n## Measured stack\n")
	fmt.Fprintf(&b, "language %s; framework %s; build system %s; test files %d\n",
		profileValue(profile.Language), profileValue(profile.Framework), profileValue(profile.BuildSystem), profile.TestFileCount)
	var deps []string
	for _, dep := range profile.Dependencies {
		if dep.Type != "transitive" {
			deps = append(deps, dep.Name)
		}
	}
	sort.Strings(deps)
	if len(deps) > 0 {
		fmt.Fprintf(&b, "direct dependencies: %s\n", strings.Join(deps, ", "))
	}
	b.WriteString("\n## Experts already on the roster\n")
	for _, agent := range stack {
		fmt.Fprintf(&b, "- %s: %s\n", agent.Name, strings.Join(agent.Topics, ", "))
	}
	fmt.Fprintf(&b, `
## Rules
- Propose at most %d experts, each with at most %d topics plus failure modes and at most %d research questions.
- Name each expert in PascalCase (letters and digits). Reusing the exact name of an expert above extends that expert instead of adding one.
- Every expert needs a "why" grounded in the north star or a document above, and "cites" naming its evidence: "north_star" or a document path exactly as listed. An expert with no valid citation is discarded.
- Research questions are answered by grounded web search: make each one a concrete, searchable question. Set "fast_moving" to true when the answer changes within months (model releases, agent techniques, provider capabilities).
- "core" may support SecurityAuditor or TestArchitect when the north star or a document calls for them, with the same why and cites.

Respond with JSON only:
{"experts":[{"name":"","mission":"","why":"","cites":[""],"topics":[""],"failure_modes":[""],"research":[{"question":"","fast_moving":false}]}],"core":[{"name":"","why":"","cites":[""]}]}
`, bounds.maxExperts, bounds.maxTopics, bounds.maxTopics)
	return b.String()
}

// parseDomainProposal validates the planner's answer strictly: names, a
// mission, a why with at least one citation of known evidence, and bounded
// topics and questions. An expert named like a stack expert keeps the stack
// expert's name. It returns notes naming what it discarded.
func parseDomainProposal(response string, evidence map[string][]string, hasVision bool, stack []RecommendedAgent, bounds domainBounds) (domainProposal, []string, error) {
	var raw domainProposal
	if err := json.Unmarshal([]byte(extractJSON(response)), &raw); err != nil {
		return domainProposal{}, nil, fmt.Errorf("reply is not the requested JSON: %w", err)
	}
	stackNames := map[string]string{}
	for _, agent := range stack {
		stackNames[normalizeAgentName(agent.Name)] = agent.Name
	}
	validCites := func(cites []string) []string {
		var kept []string
		for _, cite := range cites {
			cite = strings.TrimSpace(cite)
			switch {
			case cite == "north_star" || strings.HasPrefix(cite, "north_star:"):
				if hasVision {
					kept = append(kept, cite)
				}
			default:
				if _, ok := evidence[cite]; ok {
					kept = append(kept, cite)
				}
			}
		}
		return uniqueSorted(kept)
	}
	clean := func(values []string, limit int) []string {
		var out []string
		for _, value := range values {
			value = strings.Join(strings.Fields(value), " ")
			if value == "" {
				continue
			}
			out = append(out, value)
		}
		out = uniqueSorted(out)
		if limit > 0 && len(out) > limit {
			out = out[:limit]
		}
		return out
	}

	var out domainProposal
	var notes []string
	seen := map[string]bool{}
	for _, expert := range raw.Experts {
		name := strings.TrimSpace(expert.Name)
		if !domainExpertName.MatchString(name) {
			notes = append(notes, fmt.Sprintf("Domain expert %q discarded: not a PascalCase name", name))
			continue
		}
		if canonical, ok := stackNames[normalizeAgentName(name)]; ok {
			name = canonical
		}
		key := normalizeAgentName(name)
		if seen[key] {
			continue
		}
		expert.Name = name
		expert.Mission = strings.TrimSpace(expert.Mission)
		expert.Why = strings.Join(strings.Fields(expert.Why), " ")
		expert.Cites = validCites(expert.Cites)
		if expert.Mission == "" || expert.Why == "" {
			notes = append(notes, fmt.Sprintf("Domain expert %s discarded: no mission or why", name))
			continue
		}
		if len(expert.Cites) == 0 {
			notes = append(notes, fmt.Sprintf("Domain expert %s discarded: no citation of the north star or an oriented document", name))
			continue
		}
		expert.Topics = clean(expert.Topics, bounds.maxTopics)
		expert.FailureModes = clean(expert.FailureModes, bounds.maxTopics)
		var research []domainQuestion
		asked := map[string]bool{}
		for _, question := range expert.Research {
			text := strings.Join(strings.Fields(question.Question), " ")
			if text == "" || asked[strings.ToLower(text)] {
				continue
			}
			asked[strings.ToLower(text)] = true
			research = append(research, domainQuestion{Question: text, FastMoving: question.FastMoving})
		}
		if bounds.maxTopics > 0 && len(research) > bounds.maxTopics {
			research = research[:bounds.maxTopics]
		}
		expert.Research = research
		if len(expert.Topics)+len(expert.FailureModes) == 0 && len(expert.Research) == 0 {
			notes = append(notes, fmt.Sprintf("Domain expert %s discarded: no topics, failure modes or research questions", name))
			continue
		}
		if len(out.Experts) >= bounds.maxExperts {
			notes = append(notes, fmt.Sprintf("Domain expert %s discarded: the roster bound is %d (orient domain_agent_max)", name, bounds.maxExperts))
			continue
		}
		seen[key] = true
		out.Experts = append(out.Experts, expert)
	}
	for _, support := range raw.Core {
		name := strings.TrimSpace(support.Name)
		if !containsString(coreAgentNames, name) {
			continue
		}
		support.Name = name
		support.Why = strings.Join(strings.Fields(support.Why), " ")
		support.Cites = validCites(support.Cites)
		if support.Why == "" || len(support.Cites) == 0 {
			notes = append(notes, fmt.Sprintf("Support for %s discarded: no why or no valid citation", name))
			continue
		}
		out.Core = append(out.Core, support)
	}
	return out, notes, nil
}

// facts renders the proposal as the vision facts orient/vision_agents.mg reads.
func (p domainProposal) facts() []types.Fact {
	var facts []types.Fact
	for _, expert := range p.Experts {
		why := fmt.Sprintf("north star: %s (cites %s)", expert.Why, strings.Join(expert.Cites, ", "))
		facts = append(facts,
			types.Fact{Predicate: "vision_agent", Args: []any{expert.Name, why}},
			types.Fact{Predicate: "vision_agent_description", Args: []any{expert.Name, expert.Mission}},
		)
		for _, topic := range expert.Topics {
			facts = append(facts, types.Fact{Predicate: "vision_agent_topic", Args: []any{expert.Name, topic}})
		}
		for _, mode := range expert.FailureModes {
			facts = append(facts, types.Fact{Predicate: "vision_agent_topic", Args: []any{expert.Name, "failure mode: " + mode}})
		}
		for _, question := range expert.Research {
			facts = append(facts, types.Fact{Predicate: "vision_agent_question", Args: []any{expert.Name, question.Question}})
			if question.FastMoving {
				facts = append(facts, types.Fact{Predicate: "vision_agent_fast_question", Args: []any{expert.Name, question.Question}})
			}
		}
	}
	for _, support := range p.Core {
		facts = append(facts, types.Fact{Predicate: "vision_core_support", Args: []any{
			support.Name, fmt.Sprintf("north star: %s (cites %s)", support.Why, strings.Join(support.Cites, ", ")),
		}})
	}
	return facts
}

// domainQuestionsByAgent reads the research questions the engine holds for
// vision experts, fast-moving ones marked.
func domainQuestionsByAgent(eng *orient.Engine) (map[string][]domainQuestion, error) {
	questions, err := eng.Query("vision_agent_question")
	if err != nil {
		return nil, err
	}
	fast, err := eng.Query("vision_agent_fast_question")
	if err != nil {
		return nil, err
	}
	isFast := map[string]bool{}
	for _, row := range fast {
		if len(row.Args) >= 2 {
			isFast[types.ExtractString(row.Args[0])+"\x00"+types.ExtractString(row.Args[1])] = true
		}
	}
	out := map[string][]domainQuestion{}
	for _, row := range questions {
		if len(row.Args) < 2 {
			continue
		}
		name, question := types.ExtractString(row.Args[0]), types.ExtractString(row.Args[1])
		out[name] = append(out[name], domainQuestion{Question: question, FastMoving: isFast[name+"\x00"+question]})
	}
	for name := range out {
		sort.Slice(out[name], func(a, b int) bool { return out[name][a].Question < out[name][b].Question })
	}
	return out, nil
}

// normalizeAgentName folds a name to lower-case letters and digits, so
// "code-reviewer", "code_reviewer" and "CodeReviewer" are one agent.
func normalizeAgentName(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func loadDomainExpertsRecord(nerdDir string) (*domainExpertsRecord, error) {
	data, err := os.ReadFile(filepath.Join(nerdDir, "orientation", "domain_experts.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read the last domain-expert proposal: %w", err)
	}
	var record domainExpertsRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("the last domain-expert proposal is unreadable and is proposed again: %w", err)
	}
	return &record, nil
}

func saveDomainExpertsRecord(nerdDir string, record domainExpertsRecord) error {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(nerdDir, "orientation", "domain_experts.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return atomicfile.WriteFile(path, append(data, '\n'), 0o644)
}
