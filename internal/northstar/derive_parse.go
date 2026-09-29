package northstar

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// draftWord matches "draft" or "wip" as a whole token. "drafty" and "swipe"
// are not a work-in-progress mark; "work in progress" is matched separately
// because it is three words.
var draftWord = regexp.MustCompile(`(?i)(?:\A|[^A-Za-z0-9_])(?:draft|wip)(?:\z|[^A-Za-z0-9_])`)

func textMarksDraft(s string) bool {
	if draftWord.MatchString(s) {
		return true
	}
	low := strings.ToLower(s)
	return strings.Contains(low, "work in progress") || strings.Contains(low, "work-in-progress")
}

// finalJSON returns the JSON object in a model response. The full response
// is included when it is not an object, so a refusal is visible rather than
// cut down to a prefix.
func finalJSON(resp string) (json.RawMessage, error) {
	extracted := strings.TrimSpace(resp)
	if strings.HasPrefix(extracted, "```") && strings.HasSuffix(extracted, "```") {
		label, body, ok := strings.Cut(extracted, "\n")
		label = strings.TrimSpace(label)
		if ok && (label == "```json" || label == "```") {
			extracted = strings.TrimSpace(strings.TrimSuffix(body, "```"))
		}
	}
	if extracted == "" || extracted[0] != '{' || !json.Valid([]byte(extracted)) {
		return nil, fmt.Errorf("response is not JSON: %s", resp)
	}
	return json.RawMessage(extracted), nil
}

func parseSummary(resp string) (string, error) {
	raw, err := finalJSON(resp)
	if err != nil {
		return "", err
	}
	var body struct {
		Summary string `json:"summary"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		return "", fmt.Errorf("summary response is not JSON: %s", resp)
	}
	return body.Summary, nil
}

var roleTokens = map[string]struct{}{
	"vision":           {},
	"north_star_draft": {},
	"origin_design":    {},
	"spec":             {},
	"plan":             {},
	"report":           {},
	"standard":         {},
	"guide":            {},
	"journal":          {},
	"readme":           {},
	"instructions":     {},
	"reference":        {},
	"archive":          {},
	"generated":        {},
}

// parseConfidence accepts a JSON integer in 0..100. 80.5, "80", and 1e2 are
// rejected rather than scaled or truncated: a confidence the model did not
// state as an integer is not a confidence.
func parseConfidence(raw json.RawMessage) (int, error) {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return 0, fmt.Errorf("missing confidence_pct")
	}
	if !confidenceToken.MatchString(s) {
		return 0, fmt.Errorf("confidence_pct %s is not an integer", s)
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("confidence_pct %s is not an integer", s)
	}
	if n < 0 || n > 100 {
		return 0, fmt.Errorf("confidence_pct %d is outside 0..100", n)
	}
	return n, nil
}

var confidenceToken = regexp.MustCompile(`^(?:0|[1-9][0-9]*)$`)

func normalizeTheme(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

type rawRole struct {
	Role       string          `json:"role"`
	Confidence json.RawMessage `json:"confidence_pct"`
	Evidence   string          `json:"evidence"`
}

type rawDocument struct {
	Path   string    `json:"path"`
	Roles  []rawRole `json:"roles"`
	Themes []string  `json:"themes"`
}

func applyClassification(class *Classification, requested []SourceDocument, raw json.RawMessage, resp string) error {
	var body struct {
		Documents *[]rawDocument `json:"documents"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return fmt.Errorf("classification JSON: %v; body: %s", err, resp)
	}
	if body.Documents == nil {
		return fmt.Errorf("classification JSON lacks documents: %s", resp)
	}
	seen := make(map[string]struct{}, len(*body.Documents))
	allowed := make(map[string]map[string]bool, len(requested))
	for _, doc := range requested {
		lines := make(map[string]bool)
		for _, line := range strings.Split(doc.Body, "\n") {
			lines[strings.TrimSpace(line)] = true
		}
		allowed[doc.Path] = lines
	}
	for _, doc := range *body.Documents {
		path := doc.Path
		if _, ok := allowed[path]; !ok {
			class.Rejected = append(class.Rejected, RejectedClaim{
				Path: path, Reason: "path was not in the request",
			})
			continue
		}
		if _, ok := seen[path]; ok {
			class.Rejected = append(class.Rejected, RejectedClaim{Path: path, Reason: "duplicate document result"})
			continue
		}
		seen[path] = struct{}{}
		if len(doc.Roles) == 0 {
			class.Rejected = append(class.Rejected, RejectedClaim{Path: path, Reason: "no role claims returned"})
		}
		seenRole := make(map[string]bool)
		for _, role := range doc.Roles {
			token := strings.TrimSpace(role.Role)
			conf, confErr := parseConfidence(role.Confidence)
			evidence := strings.TrimSpace(role.Evidence)
			switch {
			case token == "":
				class.Rejected = append(class.Rejected, RejectedClaim{Path: path, Reason: "missing role"})
			case !knownRole(token):
				class.Rejected = append(class.Rejected, RejectedClaim{
					Path: path, Reason: fmt.Sprintf("unknown role %q", token),
				})
			case confErr != nil:
				class.Rejected = append(class.Rejected, RejectedClaim{Path: path, Reason: confErr.Error()})
			case evidence == "" || strings.ContainsAny(evidence, "\r\n") || !allowed[path][evidence]:
				class.Rejected = append(class.Rejected, RejectedClaim{Path: path, Reason: "evidence is not one line in the document"})
			case seenRole[token]:
				class.Rejected = append(class.Rejected, RejectedClaim{Path: path, Reason: "duplicate role " + token})
			default:
				seenRole[token] = true
				class.Claims = append(class.Claims, RoleClaim{
					Path: path, Role: token, Confidence: conf, Evidence: evidence,
				})
			}
		}
		seenTheme := make(map[string]struct{})
		for _, theme := range doc.Themes {
			norm := normalizeTheme(theme)
			if norm == "" {
				class.Rejected = append(class.Rejected, RejectedClaim{Path: path, Reason: "empty theme"})
				continue
			}
			if _, ok := seenTheme[norm]; ok {
				continue
			}
			seenTheme[norm] = struct{}{}
			class.Themes = append(class.Themes, ThemeClaim{Path: path, Theme: norm})
		}
	}
	for _, doc := range requested {
		if _, ok := seen[doc.Path]; !ok {
			class.Unread = append(class.Unread, UnreadDocument{
				Path: doc.Path, Reason: "model omitted this document",
			})
		}
	}
	return nil
}

func knownRole(token string) bool {
	_, ok := roleTokens[token]
	return ok
}

type draftPersona struct {
	Name       string   `json:"name"`
	PainPoints []string `json:"pain_points"`
	Needs      []string `json:"needs"`
	Source     string   `json:"source"`
}

type draftCapability struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Timeline    string   `json:"timeline"`
	Priority    string   `json:"priority"`
	Serves      []string `json:"serves"`
	Source      string   `json:"source"`
}

type draftRisk struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Likelihood  string `json:"likelihood"`
	Impact      string `json:"impact"`
	Mitigation  string `json:"mitigation"`
	Source      string `json:"source"`
}

type draftRequirement struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Priority    string   `json:"priority"`
	Source      string   `json:"source"`
	Supports    []string `json:"supports"`
	Addresses   []string `json:"addresses"`
}

type draftPayload struct {
	Mission      string                     `json:"Mission"`
	Problem      string                     `json:"Problem"`
	Vision       string                     `json:"Vision"`
	Personas     []draftPersona             `json:"Personas"`
	Capabilities []draftCapability          `json:"Capabilities"`
	Risks        []draftRisk                `json:"Risks"`
	Requirements []draftRequirement         `json:"Requirements"`
	Constraints  []string                   `json:"Constraints"`
	FieldSources map[string]json.RawMessage `json:"FieldSources"`
}

func parseDraft(resp string, sources []BriefSource) (*Draft, error) {
	raw, err := finalJSON(resp)
	if err != nil {
		return nil, err
	}
	var payload draftPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("vision JSON: %v; body: %s", err, resp)
	}
	doc := WizardDocument{
		Mission:      payload.Mission,
		Problem:      payload.Problem,
		Vision:       payload.Vision,
		Constraints:  payload.Constraints,
		Personas:     make([]WizardPersona, 0, len(payload.Personas)),
		Capabilities: make([]WizardCapability, 0, len(payload.Capabilities)),
		Risks:        make([]WizardRisk, 0, len(payload.Risks)),
		Requirements: make([]WizardRequirement, 0, len(payload.Requirements)),
	}
	sourcesByField := map[string][]string{}
	for _, p := range payload.Personas {
		doc.Personas = append(doc.Personas, WizardPersona{
			Name: p.Name, PainPoints: p.PainPoints, Needs: p.Needs,
		})
		addSource(sourcesByField, fmt.Sprintf("Personas[%d]", len(doc.Personas)-1), p.Source)
	}
	for _, c := range payload.Capabilities {
		doc.Capabilities = append(doc.Capabilities, WizardCapability{
			ID: c.ID, Description: c.Description, Timeline: c.Timeline,
			Priority: c.Priority, Serves: c.Serves,
		})
		addSource(sourcesByField, fmt.Sprintf("Capabilities[%d]", len(doc.Capabilities)-1), c.Source)
	}
	for _, r := range payload.Risks {
		doc.Risks = append(doc.Risks, WizardRisk{
			ID: r.ID, Description: r.Description, Likelihood: r.Likelihood,
			Impact: r.Impact, Mitigation: r.Mitigation,
		})
		addSource(sourcesByField, fmt.Sprintf("Risks[%d]", len(doc.Risks)-1), r.Source)
	}
	for _, r := range payload.Requirements {
		doc.Requirements = append(doc.Requirements, WizardRequirement{
			ID: r.ID, Type: r.Type, Description: r.Description,
			Priority: r.Priority, Source: r.Source, Supports: r.Supports, Addresses: r.Addresses,
		})
		if err := validateRequirement(r); err != nil {
			return nil, err
		}
		addSource(sourcesByField, fmt.Sprintf("Requirements[%d]", len(doc.Requirements)-1), r.Source)
	}
	doc.Requirements, err = assignRequirementIDs(doc.Requirements, nil)
	if err != nil {
		return nil, err
	}

	fieldSources := map[string][]string{}
	for key, rawSrc := range payload.FieldSources {
		list, err := parseFieldSourceValue(rawSrc)
		if err != nil {
			return nil, fmt.Errorf("FieldSources[%s]: %v; body: %s", key, err, resp)
		}
		for _, src := range list {
			addSource(fieldSources, key, src)
		}
	}
	for key, list := range sourcesByField {
		for _, src := range list {
			addSource(fieldSources, key, src)
		}
	}

	draft := &Draft{Document: doc, FieldSources: fieldSources}
	if err := validateDraft(draft, sources); err != nil {
		return nil, err
	}
	pruneDraftLinks(draft)
	return draft, nil
}

func parseFieldSourceValue(raw json.RawMessage) ([]string, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		if strings.TrimSpace(s) == "" {
			return nil, nil
		}
		return []string{s}, nil
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	return list, nil
}

func addSource(m map[string][]string, key, src string) {
	if strings.TrimSpace(src) == "" {
		return
	}
	for _, existing := range m[key] {
		if existing == src {
			return
		}
	}
	m[key] = append(m[key], src)
}

// markDraftVision keeps a draft marked as a draft. The suffix is applied
// only when a vision source says so and the statement does not already.
// An empty Vision is left empty: inventing the phrase would assert a
// statement the sources did not contain.
func markDraftVision(draft *Draft, sources []BriefSource) {
	marked := false
	for _, src := range sources {
		for _, role := range src.Roles {
			if strings.TrimPrefix(role, "/") == "north_star_draft" {
				marked = true
			}
		}
		if textMarksDraft(src.Body) || textMarksDraft(src.Why) {
			marked = true
		}
	}
	if !marked {
		return
	}
	if strings.TrimSpace(draft.Document.Vision) == "" {
		draft.Notes = append(draft.Notes, "vision sources are marked draft but Vision is empty")
		return
	}
	if textMarksDraft(draft.Document.Vision) {
		return
	}
	draft.Document.Vision = draft.Document.Vision + " (work in progress)"
}

// Reserve explicit IDs before filling blanks; an earlier blank cannot steal
// a later ID. Explicit duplicates fail instead of being renamed silently.
func assignRequirementIDs(reqs, existing []WizardRequirement) ([]WizardRequirement, error) {
	taken := make(map[string]bool, len(reqs)+len(existing))
	for _, req := range existing {
		taken[strings.ToUpper(strings.TrimSpace(req.ID))] = true
	}
	for _, req := range reqs {
		if req.ID == "" {
			continue
		}
		if !requirementID.MatchString(req.ID) || taken[req.ID] {
			return nil, fmt.Errorf("invalid or duplicate requirement ID %q", req.ID)
		}
		taken[req.ID] = true
	}
	next := 1
	alloc := func() string {
		for {
			id := fmt.Sprintf("REQ-%03d", next)
			next++
			if !taken[id] {
				taken[id] = true
				return id
			}
		}
	}
	out := make([]WizardRequirement, len(reqs))
	for i, req := range reqs {
		if req.ID == "" {
			req.ID = alloc()
		}
		out[i] = req
	}
	return out, nil
}

type requirementsPayload struct {
	Requirements *[]draftRequirement `json:"requirements"`
}

func parseRequirements(resp string, existing []WizardRequirement) ([]WizardRequirement, error) {
	raw, err := finalJSON(resp)
	if err != nil {
		return nil, err
	}
	var payload requirementsPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("requirements JSON: %v; body: %s", err, resp)
	}
	if payload.Requirements == nil {
		return nil, fmt.Errorf("requirements JSON lacks requirements: %s", resp)
	}
	reqs := make([]WizardRequirement, 0, len(*payload.Requirements))
	for _, r := range *payload.Requirements {
		if err := validateRequirement(r); err != nil {
			return nil, err
		}
		reqs = append(reqs, WizardRequirement{
			ID: r.ID, Type: r.Type, Description: r.Description,
			Priority: r.Priority, Source: r.Source, Supports: r.Supports, Addresses: r.Addresses,
		})
	}
	return assignRequirementIDs(reqs, existing)
}

var requirementID = regexp.MustCompile(`^REQ-[0-9]{3,}$`)

func oneOf(value string, choices ...string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}

func validateRequirement(r draftRequirement) error {
	if strings.TrimSpace(r.Description) == "" || !oneOf(r.Type, "functional", "non-functional", "constraint") || !oneOf(r.Priority, "must-have", "should-have", "nice-to-have") {
		return fmt.Errorf("invalid description, type or priority for requirement %q", r.ID)
	}
	return nil
}

func validateDraft(draft *Draft, sources []BriefSource) error {
	allowed := make(map[string]bool, len(sources))
	for _, src := range sources {
		allowed[src.Path] = true
	}
	for key, paths := range draft.FieldSources {
		for _, path := range paths {
			if !allowed[path] {
				return fmt.Errorf("%s names unsupplied source path %q", key, path)
			}
		}
	}
	requireSource := func(key string) error {
		if len(draft.FieldSources[key]) == 0 {
			return fmt.Errorf("%s has no source path", key)
		}
		return nil
	}
	doc := &draft.Document
	for _, field := range []struct{ key, value string }{{"Mission", doc.Mission}, {"Problem", doc.Problem}, {"Vision", doc.Vision}} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("derived %s is empty", field.key)
		}
		if err := requireSource(field.key); err != nil {
			return err
		}
	}
	if len(doc.Constraints) > 0 {
		if err := requireSource("Constraints"); err != nil {
			return err
		}
	}
	names := make(map[string]bool)
	for i, p := range doc.Personas {
		if strings.TrimSpace(p.Name) == "" || names[p.Name] {
			return fmt.Errorf("empty or duplicate persona name %q", p.Name)
		}
		names[p.Name] = true
		if err := requireSource(fmt.Sprintf("Personas[%d]", i)); err != nil {
			return err
		}
	}
	ids := make(map[string]bool)
	for i := range doc.Capabilities {
		c := &doc.Capabilities[i]
		if c.ID == "" {
			c.ID = fmt.Sprintf("cap_%d", i+1)
		}
		if strings.TrimSpace(c.Description) == "" || ids[c.ID] || !oneOf(c.Timeline, "now", "next", "later") || !oneOf(c.Priority, "critical", "high", "medium", "low") {
			return fmt.Errorf("invalid fields or duplicate capability ID %q", c.ID)
		}
		ids[c.ID] = true
		if err := requireSource(fmt.Sprintf("Capabilities[%d]", i)); err != nil {
			return err
		}
	}
	ids = make(map[string]bool)
	for i := range doc.Risks {
		r := &doc.Risks[i]
		if r.ID == "" {
			r.ID = fmt.Sprintf("risk_%d", i+1)
		}
		if strings.TrimSpace(r.Description) == "" || ids[r.ID] || !oneOf(r.Likelihood, "high", "medium", "low") || !oneOf(r.Impact, "high", "medium", "low") {
			return fmt.Errorf("invalid fields or duplicate risk ID %q", r.ID)
		}
		ids[r.ID] = true
		if err := requireSource(fmt.Sprintf("Risks[%d]", i)); err != nil {
			return err
		}
	}
	for i := range doc.Requirements {
		if err := requireSource(fmt.Sprintf("Requirements[%d]", i)); err != nil {
			return err
		}
	}
	return nil
}

func pruneDraftLinks(draft *Draft) {
	personas, caps, risks := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, p := range draft.Document.Personas {
		personas[p.Name], personas["persona_"+p.Name] = true, true
	}
	for _, c := range draft.Document.Capabilities {
		caps[c.ID] = true
	}
	for _, r := range draft.Document.Risks {
		risks[r.ID] = true
	}
	prune := func(key string, links []string, allowed map[string]bool) []string {
		var out []string
		seen := make(map[string]bool)
		for _, raw := range links {
			link := strings.TrimSpace(raw)
			if !allowed[link] || seen[link] {
				draft.Notes = append(draft.Notes, fmt.Sprintf("%s pruned dangling or duplicate link %q", key, raw))
				continue
			}
			out = append(out, link)
			seen[link] = true
		}
		return out
	}
	for i := range draft.Document.Capabilities {
		c := &draft.Document.Capabilities[i]
		c.Serves = prune(c.ID+".serves", c.Serves, personas)
	}
	for i := range draft.Document.Requirements {
		r := &draft.Document.Requirements[i]
		r.Supports = prune(r.ID+".supports", r.Supports, caps)
		r.Addresses = prune(r.ID+".addresses", r.Addresses, risks)
	}
}
