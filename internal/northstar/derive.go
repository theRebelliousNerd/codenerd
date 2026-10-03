package northstar

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"codenerd/internal/atomicfile"
	"codenerd/internal/config"
	"codenerd/internal/types"
)

// Completer is the completion surface derivation needs. perception.LLMClient
// satisfies it; tests pass a script at this boundary and nowhere deeper.
type Completer interface {
	CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// PhasePrompt returns the system prompt for one northstar derive phase
// ("derive_classify", "derive_vision", "derive_requirements").
type PhasePrompt func(ctx context.Context, phase string) (string, error)

// DeriveBudget bounds the complete user message, including framing and the
// running summary. The system prompt and completion have separate budgets.
type DeriveBudget struct {
	MaxRequestBytes int
	// Attempts is how many times one exchange is sent when its reply cannot
	// be parsed (orient.derive_attempts). Below 1 is one attempt.
	Attempts int
	Source   string
}

func (b DeriveBudget) attempts() int {
	if b.Attempts < 1 {
		return 1
	}
	return b.Attempts
}

// DerivedVisionFileName is the draft written when a vision is already stored.
const DerivedVisionFileName = "northstar.derived.json"

// SourceDocument is one read-candidate body. Path is the path the model
// must echo; Body is the full text.
type SourceDocument struct {
	Path string
	Body string
}

// RoleClaim is one accepted doc_role_claim. Role has no leading slash.
type RoleClaim struct {
	Path       string
	Role       string
	Confidence int
	Evidence   string
}

// ThemeClaim is one accepted doc_theme. Theme is already normalized.
type ThemeClaim struct {
	Path  string
	Theme string
}

// RejectedClaim is a model statement that was not asserted.
type RejectedClaim struct {
	Path   string
	Reason string
}

// UnreadDocument is a requested document the model did not return.
type UnreadDocument struct {
	Path   string
	Reason string
}

// Classification is the transduction of one read set. Evidence stays here
// for the orientation report; only Claims and Themes become facts.
type Classification struct {
	Claims       []RoleClaim
	Themes       []ThemeClaim
	Rejected     []RejectedClaim
	Unread       []UnreadDocument
	Notes        []string
	RequestBytes int
}

// Facts are the doc_role_claim and doc_theme rows. Rejected and unread
// statements are not facts.
func (c *Classification) Facts() []types.Fact {
	if c == nil {
		return nil
	}
	out := make([]types.Fact, 0, len(c.Claims)+len(c.Themes))
	for _, claim := range c.Claims {
		out = append(out, types.Fact{
			Predicate: "doc_role_claim",
			Args:      []any{claim.Path, types.MangleAtom("/" + claim.Role), int64(claim.Confidence)},
		})
	}
	for _, theme := range c.Themes {
		out = append(out, types.Fact{
			Predicate: "doc_theme",
			Args:      []any{theme.Path, theme.Theme},
		})
	}
	return out
}

// Lines are the report lines for claims, themes, rejections, and omissions.
func (c *Classification) Lines() []string {
	if c == nil {
		return nil
	}
	out := make([]string, 0, len(c.Claims)+len(c.Themes)+len(c.Rejected)+len(c.Unread))
	for _, claim := range c.Claims {
		out = append(out, fmt.Sprintf("%s [%s %d%%] %s", claim.Path, claim.Role, claim.Confidence, claim.Evidence))
	}
	for _, theme := range c.Themes {
		out = append(out, fmt.Sprintf("%s theme: %s", theme.Path, theme.Theme))
	}
	for _, rejected := range c.Rejected {
		out = append(out, fmt.Sprintf("%s rejected: %s", rejected.Path, rejected.Reason))
	}
	for _, unread := range c.Unread {
		out = append(out, fmt.Sprintf("%s unread: %s", unread.Path, unread.Reason))
	}
	return out
}

// BriefSource is a document handed to the vision draft with its full text.
type BriefSource struct {
	Path   string
	Body   string
	Why    string
	Roles  []string
	Weight int
}

// MonthView is one calendar month of the repository.
type MonthView struct {
	Index      int64
	Label      string
	Commits    int64
	FilesAdded int64
	DocsAdded  int64
}

// EraView is one derived era and what was built during it.
type EraView struct {
	Index      int64
	StartMonth int64
	EndMonth   int64
	Kind       string
	Months     []MonthView
	Subjects   []string
	Documents  []string
}

// EvolutionLink is one doc_evolved_into or doc_superseded row.
type EvolutionLink struct {
	Old string
	New string
}

// OrientationBrief is the ordered evidence for a vision draft.
// NoEraNote is set when the policy returned no repo_era rows; Months then
// are repo_month facts, not eras.
type OrientationBrief struct {
	NoEraNote  string
	Eras       []EraView
	Months     []MonthView
	Origins    []BriefSource
	Evolved    []EvolutionLink
	Superseded []EvolutionLink
	Vision     []BriefSource
}

// Draft is a WizardDocument plus the source path of each field.
type Draft struct {
	Document     WizardDocument
	FieldSources map[string][]string
	Notes        []string
}

// WizardRequirementsInput is the wizard state the requirements phase reads.
type WizardRequirementsInput struct {
	Mission              string
	Problem              string
	Vision               string
	Personas             []WizardPersona
	Capabilities         []WizardCapability
	Risks                []WizardRisk
	Constraints          []string
	ExtractedFacts       []string
	ExistingRequirements []WizardRequirement
}

// InstallResult reports where a draft went.
type InstallResult struct {
	Installed   bool
	DerivedPath string
	Vision      *Vision
}

// ResolveDeriveBudget uses the typed orient derivation configuration.
func ResolveDeriveBudget(workspace string) (DeriveBudget, error) {
	cfg, err := config.LoadOrientConfig(workspace)
	if err != nil {
		return DeriveBudget{}, err
	}
	return DeriveBudget{MaxRequestBytes: cfg.DeriveRequestBytes, Attempts: cfg.DeriveAttempts, Source: "orient.derive_request_bytes, orient.derive_attempts"}, nil
}

// ClassifyDocuments asks the model what each read-candidate is. A document
// that fits the request budget is one frame, batched with other small
// documents. A larger document is paged in order; each non-final page must
// return a summary, which is carried forward, and only the last page's
// JSON is the classification. Nothing in the body is dropped.
func ClassifyDocuments(ctx context.Context, client Completer, prompt PhasePrompt, budget DeriveBudget, docs []SourceDocument) (*Classification, error) {
	if client == nil {
		return nil, fmt.Errorf("classify documents: nil client")
	}
	if budget.MaxRequestBytes <= 0 {
		return nil, fmt.Errorf("classify documents: request budget is %d bytes (%s)", budget.MaxRequestBytes, budget.Source)
	}
	seen := make(map[string]bool, len(docs))
	for _, doc := range docs {
		if strings.TrimSpace(doc.Path) == "" || strings.ContainsAny(doc.Path, "\r\n") || !utf8.ValidString(doc.Path) {
			return nil, fmt.Errorf("classify documents: invalid document path %q", doc.Path)
		}
		if seen[doc.Path] {
			return nil, fmt.Errorf("classify documents: duplicate path %q", doc.Path)
		}
		if !utf8.ValidString(doc.Body) {
			return nil, fmt.Errorf("classify documents: %s is not UTF-8; refusing lossy model transport", doc.Path)
		}
		seen[doc.Path] = true
	}
	if prompt == nil {
		prompt = CompilePhasePrompt
	}
	system, err := prompt(ctx, "derive_classify")
	if err != nil {
		return nil, err
	}
	class := &Classification{RequestBytes: budget.MaxRequestBytes}
	class.Notes = append(class.Notes, fmt.Sprintf(
		"complete user requests, including frame headers and running summaries, are bounded to %d bytes (%s); larger documents are paged in order",
		budget.MaxRequestBytes, budget.Source))

	type item struct {
		doc   SourceDocument
		frame string
	}
	var pending []item
	pendingBytes := 0
	flush := func() error {
		if len(pending) == 0 {
			return nil
		}
		var buf strings.Builder
		requested := make([]SourceDocument, 0, len(pending))
		for _, it := range pending {
			buf.WriteString(it.frame)
			requested = append(requested, it.doc)
		}
		pending = nil
		pendingBytes = 0
		// A reply that does not parse is sent again; applyClassification
		// fails only while decoding, before it records anything. After the
		// last attempt the batch is unread and classification goes on: one
		// malformed reply must not cost the whole read set.
		var lastErr error
		for attempt := 0; attempt < budget.attempts(); attempt++ {
			resp, err := completeOne(ctx, client, system, buf.String())
			if err != nil {
				return err
			}
			raw, err := finalJSON(resp)
			if err == nil {
				err = applyClassification(class, requested, raw, resp)
			}
			if err == nil {
				return nil
			}
			lastErr = err
		}
		markUnread(class, requested, budget.attempts(), lastErr)
		return nil
	}

	for _, doc := range docs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var frame strings.Builder
		frameDocument(&frame, "doc", doc.Path, 0, len(doc.Body), true, "", doc.Body)
		if frame.Len() > budget.MaxRequestBytes {
			if err := flush(); err != nil {
				return nil, err
			}
			if err := classifyPaged(ctx, client, system, class, doc, budget); err != nil {
				return nil, err
			}
			continue
		}
		if len(pending) > 0 && frame.Len() > budget.MaxRequestBytes-pendingBytes {
			if err := flush(); err != nil {
				return nil, err
			}
		}
		pending = append(pending, item{doc: doc, frame: frame.String()})
		pendingBytes += frame.Len()
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return class, nil
}

func classifyPaged(ctx context.Context, client Completer, system string, class *Classification, doc SourceDocument, budget DeriveBudget) error {
	summary := ""
	for start, index := 0, 1; ; index++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		pg, frame, err := nextFrame("page", doc.Path, doc.Body, start, summary, budget.MaxRequestBytes)
		if err != nil {
			return err
		}
		final := pg.End >= len(doc.Body)
		// Each page is one exchange: a reply that does not parse is sent
		// again, and after the last attempt the document is unread.
		var lastErr error
		done := false
		for attempt := 0; attempt < budget.attempts() && !done; attempt++ {
			resp, err := completeOne(ctx, client, system, frame)
			if err != nil {
				return err
			}
			if !final {
				next, perr := parseSummary(resp)
				if perr == nil && strings.TrimSpace(next) == "" {
					perr = fmt.Errorf("page %d of %s returned no summary", index, doc.Path)
				}
				if perr == nil {
					summary, done = next, true
				}
				lastErr = perr
				continue
			}
			raw, perr := finalJSON(resp)
			if perr == nil {
				perr = applyClassification(class, []SourceDocument{doc}, raw, resp)
			}
			if perr == nil {
				class.Notes = append(class.Notes, fmt.Sprintf("%s read in %d ordered pages (%d document bytes)", doc.Path, index, len(doc.Body)))
				return nil
			}
			lastErr = perr
		}
		if !done {
			markUnread(class, []SourceDocument{doc}, budget.attempts(), lastErr)
			return nil
		}
		start = pg.End
	}
}

// markUnread records documents whose exchange never produced a parseable
// reply. The error, which carries the model's reply, is noted once.
func markUnread(class *Classification, docs []SourceDocument, attempts int, err error) {
	for _, doc := range docs {
		class.Unread = append(class.Unread, UnreadDocument{
			Path:   doc.Path,
			Reason: fmt.Sprintf("no parseable classification after %d attempts", attempts),
		})
	}
	class.Notes = append(class.Notes, fmt.Sprintf(
		"%d document(s) unread after %d attempts; last reply error: %v", len(docs), attempts, err))
}

// DraftVision drafts a WizardDocument from the orientation brief. The brief
// is one byte stream in the order timeline, origins, evolution, vision
// sources. A brief longer than the request budget is paged; the running
// summary is carried and only the last page is parsed.
func DraftVision(ctx context.Context, client Completer, prompt PhasePrompt, budget DeriveBudget, brief OrientationBrief) (*Draft, error) {
	if client == nil {
		return nil, fmt.Errorf("draft vision: nil client")
	}
	if budget.MaxRequestBytes <= 0 {
		return nil, fmt.Errorf("draft vision: request budget is %d bytes (%s)", budget.MaxRequestBytes, budget.Source)
	}
	if len(brief.Origins) == 0 && len(brief.Vision) == 0 {
		return nil, fmt.Errorf("draft vision: no source documents supplied")
	}
	if prompt == nil {
		prompt = CompilePhasePrompt
	}
	system, err := prompt(ctx, "derive_vision")
	if err != nil {
		return nil, err
	}
	sources := sortedVision(brief.Vision)
	brief.Vision = sources
	text := renderBrief(brief)
	allSources := append(append([]BriefSource(nil), brief.Origins...), sources...)
	var draft *Draft
	var notes []string
	var lastErr error
	for attempt := 0; attempt < budget.attempts() && draft == nil; attempt++ {
		resp, pageNotes, err := completeBrief(ctx, client, system, "orientation brief", text, budget.MaxRequestBytes)
		if err != nil {
			return nil, err
		}
		if draft, lastErr = parseDraft(resp, allSources); lastErr != nil {
			draft = nil
		}
		notes = pageNotes
	}
	if draft == nil {
		return nil, fmt.Errorf("draft vision: no parseable reply after %d attempts: %w", budget.attempts(), lastErr)
	}
	if len(notes) > 0 {
		draft.Notes = append(notes, draft.Notes...)
	}
	markDraftVision(draft, sources)
	return draft, nil
}

// DeriveRequirements drafts requirements from wizard state. Every returned
// requirement is kept; the prompt's count guidance is not a cap.
func DeriveRequirements(ctx context.Context, client Completer, prompt PhasePrompt, budget DeriveBudget, state WizardRequirementsInput) ([]WizardRequirement, error) {
	if client == nil {
		return nil, fmt.Errorf("derive requirements: nil client")
	}
	if budget.MaxRequestBytes <= 0 {
		return nil, fmt.Errorf("derive requirements: request budget is %d bytes (%s)", budget.MaxRequestBytes, budget.Source)
	}
	if prompt == nil {
		prompt = CompilePhasePrompt
	}
	system, err := prompt(ctx, "derive_requirements")
	if err != nil {
		return nil, err
	}
	text, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("encode requirements state: %w", err)
	}
	var lastErr error
	for attempt := 0; attempt < budget.attempts(); attempt++ {
		resp, _, err := completeBrief(ctx, client, system, "requirements state", string(text), budget.MaxRequestBytes)
		if err != nil {
			return nil, err
		}
		reqs, perr := parseRequirements(resp, state.ExistingRequirements)
		if perr == nil {
			return reqs, nil
		}
		lastErr = perr
	}
	return nil, fmt.Errorf("derive requirements: no parseable reply after %d attempts: %w", budget.attempts(), lastErr)
}

func completeBrief(ctx context.Context, client Completer, system, label, text string, budget int) (string, []string, error) {
	if !utf8.ValidString(text) {
		return "", nil, fmt.Errorf("%s is not UTF-8; refusing lossy model transport", label)
	}
	var notes []string
	summary := ""
	for start, index := 0, 1; ; index++ {
		if err := ctx.Err(); err != nil {
			return "", notes, err
		}
		pg, frame, err := nextFrame("brief", "", text, start, summary, budget)
		if err != nil {
			return "", notes, err
		}
		resp, err := completeOne(ctx, client, system, frame)
		if err != nil {
			return "", notes, err
		}
		if pg.End < len(text) {
			summary, err = parseSummary(resp)
			if err != nil {
				return "", notes, err
			}
			if strings.TrimSpace(summary) == "" {
				return "", notes, fmt.Errorf("page %d of %s returned no summary; refusing to continue without it", index, label)
			}
			start = pg.End
			continue
		}
		notes = append(notes, fmt.Sprintf("%s read in %d ordered pages (%d bytes); complete user requests bounded to %d bytes", label, index, len(text), budget))
		return resp, notes, nil
	}
}

func completeOne(ctx context.Context, client Completer, system, user string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return client.CompleteWithSystem(ctx, system, user)
}

func sortedVision(sources []BriefSource) []BriefSource {
	out := append([]BriefSource(nil), sources...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Weight != out[j].Weight {
			return out[i].Weight > out[j].Weight
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func renderBrief(brief OrientationBrief) string {
	var b strings.Builder
	b.WriteString("## Timeline\n")
	if len(brief.Eras) == 0 {
		if brief.NoEraNote != "" {
			b.WriteString(brief.NoEraNote)
			b.WriteString("\n")
		}
		writeMonths(&b, brief.Months)
	}
	for _, era := range brief.Eras {
		fmt.Fprintf(&b, "\n### Era %d months %d-%d kind %s\n", era.Index, era.StartMonth, era.EndMonth, era.Kind)
		writeMonths(&b, era.Months)
		b.WriteString("Commit subjects:\n")
		if len(era.Subjects) == 0 {
			b.WriteString("(none)\n")
		}
		for _, subject := range era.Subjects {
			b.WriteString("- ")
			b.WriteString(subject)
			b.WriteByte('\n')
		}
		if len(era.Documents) > 0 {
			b.WriteString("Documents:\n")
			for _, doc := range era.Documents {
				b.WriteString("- ")
				b.WriteString(doc)
				b.WriteByte('\n')
			}
		}
	}
	b.WriteString("\n## Origin sources\n")
	writeSources(&b, brief.Origins)
	b.WriteString("\n## Evolution\n")
	if len(brief.Evolved) == 0 && len(brief.Superseded) == 0 {
		b.WriteString("(none)\n")
	}
	for _, link := range brief.Evolved {
		fmt.Fprintf(&b, "- %s evolved into %s\n", link.Old, link.New)
	}
	for _, link := range brief.Superseded {
		fmt.Fprintf(&b, "- %s superseded by %s\n", link.Old, link.New)
	}
	b.WriteString("\n## Vision sources\n")
	writeSources(&b, brief.Vision)
	return b.String()
}

func writeMonths(b *strings.Builder, months []MonthView) {
	if len(months) == 0 {
		b.WriteString("(no months)\n")
		return
	}
	for _, month := range months {
		fmt.Fprintf(b, "- month %d %s commits %d files_added %d docs_added %d\n",
			month.Index, month.Label, month.Commits, month.FilesAdded, month.DocsAdded)
	}
}

func writeSources(b *strings.Builder, sources []BriefSource) {
	if len(sources) == 0 {
		b.WriteString("(none)\n")
		return
	}
	for _, src := range sources {
		fmt.Fprintf(b, "### %s\n", src.Path)
		if src.Why != "" {
			fmt.Fprintf(b, "why: %s\n", src.Why)
		}
		if len(src.Roles) > 0 {
			fmt.Fprintf(b, "roles: %s\n", strings.Join(src.Roles, ", "))
		}
		if src.Weight != 0 {
			fmt.Fprintf(b, "weight_pct: %d\n", src.Weight)
		}
		b.WriteString(src.Body)
		if !strings.HasSuffix(src.Body, "\n") {
			b.WriteByte('\n')
		}
	}
}

// InstallDerivedVision writes a draft. An empty mission is refused before
// any file is written, because the loader treats an empty mission as no
// vision. A vision already in the store is left in place and the draft is
// written beside it. A fresh vision is inserted only if authority is absent,
// and both boot surfaces are written from that same value.
func InstallDerivedVision(store *Store, nerdDir string, draft *Draft) (InstallResult, error) {
	if draft == nil {
		return InstallResult{}, fmt.Errorf("derived vision is nil")
	}
	if strings.TrimSpace(draft.Document.Mission) == "" {
		return InstallResult{}, fmt.Errorf("derived vision has an empty mission; refusing to write a vision the loader treats as absent")
	}
	if store == nil {
		return InstallResult{}, fmt.Errorf("northstar store is nil")
	}
	vision := draft.Document.ToVision()
	installed, err := store.installDerivedVisionIfAbsent(vision)
	if err != nil {
		return InstallResult{}, err
	}
	if !installed {
		if err := os.MkdirAll(nerdDir, 0o755); err != nil {
			return InstallResult{}, fmt.Errorf("create %s: %w", nerdDir, err)
		}
		path := filepath.Join(nerdDir, DerivedVisionFileName)
		payload := struct {
			Note         string              `json:"note"`
			Notes        []string            `json:"notes,omitempty"`
			FieldSources map[string][]string `json:"field_sources,omitempty"`
			Document     WizardDocument      `json:"document"`
		}{
			Note:         "generated, do not hand-edit. A vision already existed in the store; this draft was not installed over it.",
			Notes:        draft.Notes,
			FieldSources: draft.FieldSources,
			Document:     draft.Document,
		}
		data, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return InstallResult{}, err
		}
		if err := atomicfile.WriteFile(path, append(data, '\n'), 0o600); err != nil {
			return InstallResult{}, fmt.Errorf("write %s: %w", path, err)
		}
		return InstallResult{Installed: false, DerivedPath: path, Vision: draft.Document.ToVision()}, nil
	}
	result := InstallResult{Installed: true, Vision: vision}
	if _, err := WriteVisionJSON(nerdDir, vision); err != nil {
		return result, fmt.Errorf("derived vision installed; JSON export failed: %w", err)
	}
	if err := WriteVisionMangle(nerdDir, vision); err != nil {
		return result, fmt.Errorf("derived vision installed; Mangle export failed: %w", err)
	}
	return result, nil
}

// The insert guard belongs in the transaction: LoadVision followed by
// SaveVision could overwrite a vision installed by another caller in between.
func (s *Store) installDerivedVisionIfAbsent(v *Vision) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	values := []any{v.Mission, v.Problem, v.VisionStmt}
	for _, field := range []struct {
		name  string
		value any
	}{
		{"personas", v.Personas}, {"capabilities", v.Capabilities}, {"risks", v.Risks},
		{"requirements", v.Requirements}, {"constraints", v.Constraints},
	} {
		text, err := marshalJSONString("derived vision "+field.name, field.value)
		if err != nil {
			return false, err
		}
		values = append(values, text)
	}
	now := time.Now()
	created := v.CreatedAt
	if created.IsZero() {
		created = now
	}
	values = append(values, created, now)
	tx, err := s.db.Begin()
	if err != nil {
		return false, fmt.Errorf("begin derived vision install: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.Exec(`INSERT INTO vision (id, mission, problem, vision_statement, personas_json,
		capabilities_json, risks_json, requirements_json, constraints_json, created_at, updated_at)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO NOTHING`, values...)
	if err != nil {
		return false, fmt.Errorf("insert derived vision: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("count derived vision insert: %w", err)
	}
	if count == 0 {
		return false, nil
	}
	if _, err := tx.Exec(`UPDATE guardian_state SET vision_defined = 1 WHERE id = 1`); err != nil {
		return false, fmt.Errorf("update derived vision guardian state: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit derived vision install: %w", err)
	}
	v.CreatedAt, v.UpdatedAt = created, now
	return true, nil
}
