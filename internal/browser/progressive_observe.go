package browser

// Progressive observation is adapted from BrowserNERD's Apache-2.0
// browser-observe contract. codeNERD keeps the behavior native: one bounded
// manager API, one live Cortex fact sink, and no embedded MCP/Mangle runtime.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	browsersecurity "codenerd/internal/browser/security"
	"codenerd/internal/mangle"
)

const (
	defaultObservationItems = 20
	maxObservationItems     = 100
)

type ObserveOptions struct {
	Mode         string
	View         string
	MaxItems     int
	Filter       string
	VisibleOnly  bool
	InternalOnly bool
	FullPage     bool
	SavePath     string
}

type ProgressiveObservation struct {
	Success         bool           `json:"success"`
	Status          string         `json:"status"`
	SessionID       string         `json:"session_id,omitempty"`
	Mode            string         `json:"mode"`
	View            string         `json:"view"`
	Generation      int            `json:"generation,omitempty"`
	Summary         string         `json:"summary"`
	Data            map[string]any `json:"data"`
	EvidenceHandles []string       `json:"evidence_handles"`
	Truncated       bool           `json:"truncated"`
}

type PageState struct {
	URL       string `json:"url"`
	Title     string `json:"title,omitempty"`
	Loading   bool   `json:"loading"`
	HasDialog bool   `json:"has_dialog"`
}

type PublicElementFingerprint struct {
	TagName     string             `json:"tag_name,omitempty"`
	ID          string             `json:"id,omitempty"`
	Name        string             `json:"name,omitempty"`
	Classes     []string           `json:"classes,omitempty"`
	TextContent string             `json:"text_content,omitempty"`
	AriaLabel   string             `json:"aria_label,omitempty"`
	DataTestID  string             `json:"data_testid,omitempty"`
	Role        string             `json:"role,omitempty"`
	InputType   string             `json:"input_type,omitempty"`
	RowKey      string             `json:"row_key,omitempty"`
	RowIndex    string             `json:"row_index,omitempty"`
	BoundingBox map[string]float64 `json:"bounding_box,omitempty"`
}

type InteractiveElement struct {
	Ref         string                    `json:"ref"`
	Type        string                    `json:"type"`
	Action      string                    `json:"action"`
	Label       string                    `json:"label,omitempty"`
	Disabled    bool                      `json:"disabled,omitempty"`
	Checked     bool                      `json:"checked,omitempty"`
	Fingerprint *PublicElementFingerprint `json:"fingerprint,omitempty"`
}

type NavigationElement struct {
	Ref      string `json:"ref"`
	Label    string `json:"label,omitempty"`
	Href     string `json:"href"`
	External bool   `json:"external,omitempty"`
}

type GridObservation struct {
	Type            string               `json:"type"`
	Label           string               `json:"label,omitempty"`
	RowCount        int                  `json:"row_count"`
	VisibleRowCount int                  `json:"visible_row_count"`
	ColumnCount     int                  `json:"column_count"`
	SampleRows      []InteractiveElement `json:"sample_rows,omitempty"`
}

type HiddenObservation struct {
	Ref                 string `json:"ref,omitempty"`
	Type                string `json:"type"`
	Trigger             string `json:"trigger,omitempty"`
	State               string `json:"state"`
	Expandable          bool   `json:"expandable,omitempty"`
	ContentPreview      string `json:"content_preview,omitempty"`
	InteractiveElements int    `json:"interactive_elements,omitempty"`
}

type ScreenshotEvidence struct {
	Path      string `json:"path"`
	MediaType string `json:"media_type"`
	Bytes     int    `json:"bytes"`
	SHA256    string `json:"sha256"`
}

type rawProgressiveSnapshot struct {
	URL              string       `json:"url"`
	Title            string       `json:"title"`
	Loading          bool         `json:"loading"`
	HasDialog        bool         `json:"has_dialog"`
	Interactive      []rawElement `json:"interactive"`
	Navigation       []rawElement `json:"navigation"`
	Grids            []rawGrid    `json:"grids"`
	Hidden           []rawHidden  `json:"hidden"`
	InteractiveTotal int          `json:"interactive_total"`
	NavigationTotal  int          `json:"navigation_total"`
	GridTotal        int          `json:"grid_total"`
	HiddenTotal      int          `json:"hidden_total"`
}

type rawElement struct {
	Selector     string             `json:"selector"`
	AltSelectors []string           `json:"alt_selectors"`
	TagName      string             `json:"tag_name"`
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	Classes      []string           `json:"classes"`
	TextContent  string             `json:"text_content"`
	AriaLabel    string             `json:"aria_label"`
	DataTestID   string             `json:"data_testid"`
	Role         string             `json:"role"`
	InputType    string             `json:"input_type"`
	RowKey       string             `json:"row_key"`
	RowIndex     string             `json:"row_index"`
	BoundingBox  map[string]float64 `json:"bounding_box"`
	Type         string             `json:"type"`
	Action       string             `json:"action"`
	Label        string             `json:"label"`
	Href         string             `json:"href"`
	Disabled     bool               `json:"disabled"`
	Checked      bool               `json:"checked"`
	External     bool               `json:"external"`
}

type rawGrid struct {
	Type            string       `json:"type"`
	Label           string       `json:"label"`
	RowCount        int          `json:"row_count"`
	VisibleRowCount int          `json:"visible_row_count"`
	ColumnCount     int          `json:"column_count"`
	SampleRows      []rawElement `json:"sample_rows"`
}

type rawHidden struct {
	Element             rawElement `json:"element"`
	Type                string     `json:"type"`
	Trigger             string     `json:"trigger"`
	State               string     `json:"state"`
	Expandable          bool       `json:"expandable"`
	ContentPreview      string     `json:"content_preview"`
	InteractiveElements int        `json:"interactive_elements"`
}

// Observe returns one bounded disclosure slice and registers opaque element
// refs for later Act calls.
func (m *SessionManager) Observe(ctx context.Context, sessionID string, opts ObserveOptions) (ProgressiveObservation, error) {
	mode, view, maxItems, filter, err := normalizeObserveOptions(opts)
	if err != nil {
		return ProgressiveObservation{}, err
	}

	if mode == "sessions" {
		sessions := m.List()
		truncated := len(sessions) > maxItems
		if truncated {
			sessions = sessions[:maxItems]
		}
		data := map[string]any{"session_count": len(m.List())}
		if view != "summary" {
			data["sessions"] = sessions
		}
		return ProgressiveObservation{
			Success: true, Status: "ok", Mode: mode, View: view,
			Summary: fmt.Sprintf("%d active session(s)", len(m.List())), Data: data,
			EvidenceHandles: []string{"browser:sessions"}, Truncated: truncated,
		}, nil
	}
	if strings.TrimSpace(sessionID) == "" {
		return ProgressiveObservation{}, fmt.Errorf("session_id is required")
	}

	registry := m.Registry(sessionID)
	if registry == nil {
		return ProgressiveObservation{}, fmt.Errorf("unknown session: %s", sessionID)
	}
	generation := registry.Generation()
	handle := fmt.Sprintf("browser:%s:g%d:%s", sessionID, generation, mode)

	switch mode {
	case "screenshot":
		evidence, captureErr := m.captureScreenshotEvidence(ctx, sessionID, opts.FullPage, opts.SavePath)
		if captureErr != nil {
			return ProgressiveObservation{}, captureErr
		}
		return ProgressiveObservation{
			Success: true, Status: "ok", SessionID: sessionID, Mode: mode, View: view,
			Generation: generation, Summary: "screenshot captured", Data: map[string]any{"screenshot": evidence},
			EvidenceHandles: []string{handle},
		}, nil
	case "react":
		facts, reactErr := m.ReifyReact(ctx, sessionID)
		if reactErr != nil {
			return ProgressiveObservation{}, reactErr
		}
		components := 0
		for _, fact := range facts {
			if fact.Predicate == "react_component" {
				components++
			}
		}
		return ProgressiveObservation{
			Success: true, Status: "ok", SessionID: sessionID, Mode: mode, View: view,
			Generation: generation, Summary: fmt.Sprintf("React tree: %d component(s)", components),
			Data: map[string]any{"component_count": components, "fact_count": len(facts)}, EvidenceHandles: []string{handle},
		}, nil
	case "dom_snapshot":
		if snapshotErr := m.SnapshotDOM(ctx, sessionID); snapshotErr != nil {
			return ProgressiveObservation{}, snapshotErr
		}
		return ProgressiveObservation{
			Success: true, Status: "ok", SessionID: sessionID, Mode: mode, View: view,
			Generation: generation, Summary: "DOM snapshot captured into the live kernel",
			Data: map[string]any{"captured": true, "node_limit": 200}, EvidenceHandles: []string{handle},
		}, nil
	}

	page, ok := m.Page(sessionID)
	if !ok || page == nil {
		return ProgressiveObservation{}, fmt.Errorf("unknown or detached session: %s", sessionID)
	}
	result, evalErr := page.Context(ctx).Eval(progressiveObserveJS, map[string]any{
		"mode": mode, "maxItems": maxItems, "filter": filter,
		"visibleOnly": opts.VisibleOnly, "internalOnly": opts.InternalOnly,
	})
	if evalErr != nil {
		return ProgressiveObservation{}, fmt.Errorf("observe page: %w", evalErr)
	}
	rawJSON, marshalErr := result.Value.MarshalJSON()
	if marshalErr != nil {
		return ProgressiveObservation{}, fmt.Errorf("marshal observation: %w", marshalErr)
	}
	var snapshot rawProgressiveSnapshot
	if unmarshalErr := json.Unmarshal(rawJSON, &snapshot); unmarshalErr != nil {
		return ProgressiveObservation{}, fmt.Errorf("decode observation: %w", unmarshalErr)
	}
	if info, infoErr := page.Context(ctx).Info(); infoErr == nil && info != nil && info.URL != "" && info.URL != snapshot.URL {
		return ProgressiveObservation{}, fmt.Errorf("page changed URL during observation; observe session %s again", sessionID)
	}
	if registry.Generation() != generation {
		return ProgressiveObservation{}, fmt.Errorf("page navigated during observation; observe session %s again", sessionID)
	}

	state := PageState{
		URL: m.SanitizeForEvidence(snapshot.URL), Title: m.SanitizeForEvidence(snapshot.Title),
		Loading: snapshot.Loading, HasDialog: snapshot.HasDialog,
	}
	data := map[string]any{"state": state}
	counts := map[string]int{
		"navigation": snapshot.NavigationTotal, "interactive": snapshot.InteractiveTotal,
		"grids": snapshot.GridTotal, "hidden": snapshot.HiddenTotal,
	}
	data["counts"] = counts

	navigation, navFacts, materializeErr := m.materializeNavigation(sessionID, registry, generation, snapshot.Navigation)
	if materializeErr != nil {
		return ProgressiveObservation{}, materializeErr
	}
	interactive, interactiveFacts, materializeErr := m.materializeInteractive(sessionID, registry, generation, snapshot.Interactive, view == "full")
	if materializeErr != nil {
		return ProgressiveObservation{}, materializeErr
	}
	grids, materializeErr := m.materializeGrids(registry, generation, snapshot.Grids, view == "full")
	if materializeErr != nil {
		return ProgressiveObservation{}, materializeErr
	}
	hidden, materializeErr := m.materializeHidden(registry, generation, snapshot.Hidden)
	if materializeErr != nil {
		return ProgressiveObservation{}, materializeErr
	}
	if registry.Generation() != generation {
		return ProgressiveObservation{}, fmt.Errorf("page navigated while materializing observation; observe session %s again", sessionID)
	}
	facts := append(navFacts, interactiveFacts...)
	if state.URL != "" {
		facts = append(facts, mangle.Fact{Predicate: "current_url", Args: []any{sessionID, state.URL}, Timestamp: time.Now()})
	}
	if len(facts) > 0 {
		if factErr := m.addFacts(facts); factErr != nil {
			return ProgressiveObservation{}, fmt.Errorf("assert observation facts: %w", factErr)
		}
	}

	if view != "summary" {
		switch mode {
		case "state":
		case "nav":
			data["navigation"] = navigation
		case "interactive":
			data["interactive"] = interactive
		case "grids":
			data["grids"] = grids
		case "hidden":
			data["hidden"] = hidden
		case "composite":
			data["navigation"] = navigation
			data["interactive"] = interactive
			data["grids"] = grids
			if view == "full" {
				data["hidden"] = hidden
			}
		}
	}

	generation = registry.Generation()
	handle = fmt.Sprintf("browser:%s:g%d:%s", sessionID, generation, mode)
	truncated := snapshot.NavigationTotal > len(snapshot.Navigation) ||
		snapshot.InteractiveTotal > len(snapshot.Interactive) ||
		snapshot.GridTotal > len(snapshot.Grids) || snapshot.HiddenTotal > len(snapshot.Hidden)
	return ProgressiveObservation{
		Success: true, Status: "ok", SessionID: sessionID, Mode: mode, View: view,
		Generation: generation, Summary: observationSummary(mode, counts), Data: data,
		EvidenceHandles: []string{handle}, Truncated: truncated,
	}, nil
}

func normalizeObserveOptions(opts ObserveOptions) (mode, view string, maxItems int, filter string, err error) {
	mode = strings.ToLower(strings.TrimSpace(opts.Mode))
	if mode == "" {
		mode = "composite"
	}
	if mode == "navigation" {
		mode = "nav"
	}
	if mode == "dom" {
		mode = "dom_snapshot"
	}
	switch mode {
	case "state", "nav", "interactive", "hidden", "grids", "composite", "sessions", "screenshot", "react", "dom_snapshot":
	default:
		return "", "", 0, "", fmt.Errorf("unsupported observation mode %q", opts.Mode)
	}
	view = strings.ToLower(strings.TrimSpace(opts.View))
	if view == "" {
		view = "compact"
	}
	if view != "summary" && view != "compact" && view != "full" {
		return "", "", 0, "", fmt.Errorf("unsupported observation view %q", opts.View)
	}
	maxItems = opts.MaxItems
	if maxItems <= 0 {
		maxItems = defaultObservationItems
	}
	if maxItems > maxObservationItems {
		maxItems = maxObservationItems
	}
	filter = strings.ToLower(strings.TrimSpace(opts.Filter))
	if filter == "" {
		filter = "all"
	}
	switch filter {
	case "all", "buttons", "inputs", "links", "selects":
	default:
		return "", "", 0, "", fmt.Errorf("unsupported interactive filter %q", opts.Filter)
	}
	return mode, view, maxItems, filter, nil
}

func (m *SessionManager) captureScreenshotEvidence(ctx context.Context, sessionID string, fullPage bool, requested string) (ScreenshotEvidence, error) {
	data, err := m.Screenshot(ctx, sessionID, fullPage)
	if err != nil {
		return ScreenshotEvidence{}, err
	}
	shortID := sessionID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	name := fmt.Sprintf("browser-%s-%d.png", shortID, time.Now().UnixMilli())
	defaultRoot := ""
	if m.pathPolicy != nil {
		defaultRoot = m.pathPolicy.DefaultRoot()
	}
	path, err := m.ResolveOutputPath(requested, defaultRoot, name)
	if err != nil {
		return ScreenshotEvidence{}, err
	}
	if err := browsersecurity.EnsurePrivateDir(filepath.Dir(path)); err != nil {
		return ScreenshotEvidence{}, fmt.Errorf("create screenshot directory: %w", err)
	}
	if err := browsersecurity.WritePrivateFile(path, data); err != nil {
		return ScreenshotEvidence{}, fmt.Errorf("write screenshot: %w", err)
	}
	digest := sha256.Sum256(data)
	return ScreenshotEvidence{Path: path, MediaType: "image/png", Bytes: len(data), SHA256: hex.EncodeToString(digest[:])}, nil
}

func (m *SessionManager) materializeNavigation(sessionID string, registry *ElementRegistry, generation int, raw []rawElement) ([]NavigationElement, []mangle.Fact, error) {
	registered, ok := registerRawElements(registry, generation, raw)
	if !ok {
		return nil, nil, fmt.Errorf("page navigated while registering navigation refs")
	}
	result := make([]NavigationElement, 0, len(raw))
	facts := make([]mangle.Fact, 0, len(raw))
	now := time.Now()
	for i := range raw {
		href := m.SanitizeForEvidence(raw[i].Href)
		result = append(result, NavigationElement{
			Ref: registered[i].Ref, Label: m.SanitizeForEvidence(raw[i].Label), Href: href, External: raw[i].External,
		})
		facts = append(facts, mangle.Fact{Predicate: "link", Args: []any{qualifiedRef(sessionID, registered[i].Ref), href}, Timestamp: now})
	}
	return result, facts, nil
}

func (m *SessionManager) materializeInteractive(sessionID string, registry *ElementRegistry, generation int, raw []rawElement, verbose bool) ([]InteractiveElement, []mangle.Fact, error) {
	registered, ok := registerRawElements(registry, generation, raw)
	if !ok {
		return nil, nil, fmt.Errorf("page navigated while registering interactive refs")
	}
	result := make([]InteractiveElement, 0, len(raw))
	facts := make([]mangle.Fact, 0, len(raw)*2)
	now := time.Now()
	for i := range raw {
		item := InteractiveElement{
			Ref: registered[i].Ref, Type: raw[i].Type, Action: raw[i].Action,
			Label: m.SanitizeForEvidence(raw[i].Label), Disabled: raw[i].Disabled, Checked: raw[i].Checked,
		}
		if verbose {
			item.Fingerprint = publicFingerprint(m, registered[i])
		}
		result = append(result, item)
		ref := qualifiedRef(sessionID, registered[i].Ref)
		facts = append(facts, mangle.Fact{Predicate: "interactable", Args: []any{ref, interactionName(raw[i])}, Timestamp: now})
		if raw[i].BoundingBox["width"] > 0 && raw[i].BoundingBox["height"] > 0 {
			facts = append(facts, mangle.Fact{Predicate: "visible", Args: []any{ref}, Timestamp: now})
		}
	}
	return result, facts, nil
}

func (m *SessionManager) materializeGrids(registry *ElementRegistry, generation int, raw []rawGrid, verbose bool) ([]GridObservation, error) {
	result := make([]GridObservation, 0, len(raw))
	for _, grid := range raw {
		registered, ok := registerRawElements(registry, generation, grid.SampleRows)
		if !ok {
			return nil, fmt.Errorf("page navigated while registering grid refs")
		}
		rows := make([]InteractiveElement, 0, len(grid.SampleRows))
		for i := range grid.SampleRows {
			row := InteractiveElement{Ref: registered[i].Ref, Type: "row", Action: "click", Label: m.SanitizeForEvidence(grid.SampleRows[i].Label)}
			if verbose {
				row.Fingerprint = publicFingerprint(m, registered[i])
			}
			rows = append(rows, row)
		}
		result = append(result, GridObservation{
			Type: grid.Type, Label: m.SanitizeForEvidence(grid.Label), RowCount: grid.RowCount,
			VisibleRowCount: grid.VisibleRowCount, ColumnCount: grid.ColumnCount, SampleRows: rows,
		})
	}
	return result, nil
}

func (m *SessionManager) materializeHidden(registry *ElementRegistry, generation int, raw []rawHidden) ([]HiddenObservation, error) {
	result := make([]HiddenObservation, 0, len(raw))
	for i := range raw {
		ref := ""
		if raw[i].Element.Selector != "" {
			registered, ok := registerRawElements(registry, generation, []rawElement{raw[i].Element})
			if !ok {
				return nil, fmt.Errorf("page navigated while registering hidden refs")
			}
			ref = registered[0].Ref
		}
		result = append(result, HiddenObservation{
			Ref: ref, Type: raw[i].Type, Trigger: m.SanitizeForEvidence(raw[i].Trigger), State: raw[i].State,
			Expandable: raw[i].Expandable, ContentPreview: m.SanitizeForEvidence(raw[i].ContentPreview),
			InteractiveElements: raw[i].InteractiveElements,
		})
	}
	return result, nil
}

func registerRawElements(registry *ElementRegistry, generation int, raw []rawElement) ([]ElementFingerprint, bool) {
	fingerprints := make([]ElementFingerprint, len(raw))
	for i := range raw {
		fingerprints[i] = ElementFingerprint{
			Selector: raw[i].Selector, AltSelectors: raw[i].AltSelectors, TagName: raw[i].TagName,
			ID: raw[i].ID, Name: raw[i].Name, Classes: raw[i].Classes, TextContent: raw[i].TextContent,
			AriaLabel: raw[i].AriaLabel, DataTestID: raw[i].DataTestID, Role: raw[i].Role,
			InputType: raw[i].InputType, RowKey: raw[i].RowKey, RowIndex: raw[i].RowIndex,
			BoundingBox: raw[i].BoundingBox,
		}
	}
	return registry.RegisterBatchForGeneration(generation, fingerprints)
}

func publicFingerprint(m *SessionManager, fp ElementFingerprint) *PublicElementFingerprint {
	classes := make([]string, 0, len(fp.Classes))
	for _, className := range fp.Classes {
		classes = append(classes, m.SanitizeForEvidence(className))
	}
	return &PublicElementFingerprint{
		TagName: fp.TagName, ID: m.SanitizeForEvidence(fp.ID), Name: m.SanitizeForEvidence(fp.Name), Classes: classes,
		TextContent: m.SanitizeForEvidence(fp.TextContent), AriaLabel: m.SanitizeForEvidence(fp.AriaLabel),
		DataTestID: m.SanitizeForEvidence(fp.DataTestID), Role: fp.Role, InputType: fp.InputType,
		RowKey: m.SanitizeForEvidence(fp.RowKey), RowIndex: m.SanitizeForEvidence(fp.RowIndex), BoundingBox: fp.BoundingBox,
	}
}

func interactionName(raw rawElement) string {
	switch raw.Type {
	case "checkbox":
		return "/checkbox"
	case "radio":
		return "/radio"
	case "input", "select":
		return "/input"
	default:
		return "/click"
	}
}

func qualifiedRef(sessionID, ref string) string { return sessionID + ":" + ref }

func observationSummary(mode string, counts map[string]int) string {
	switch mode {
	case "state":
		return "page state captured"
	case "nav":
		return fmt.Sprintf("%d navigation target(s)", counts["navigation"])
	case "interactive":
		return fmt.Sprintf("%d interactive element(s)", counts["interactive"])
	case "grids":
		return fmt.Sprintf("%d grid surface(s)", counts["grids"])
	case "hidden":
		return fmt.Sprintf("%d hidden section(s)", counts["hidden"])
	default:
		return fmt.Sprintf("page map: %d navigation, %d interactive, %d grids", counts["navigation"], counts["interactive"], counts["grids"])
	}
}

const progressiveObserveJS = `(opts) => {
  const maxItems = Math.max(1, Math.min(Number(opts.maxItems || 20), 100));
  const clean = value => String(value || '').replace(/\s+/g, ' ').trim();
  const esc = value => window.CSS && CSS.escape ? CSS.escape(String(value)) : String(value).replace(/[^a-zA-Z0-9_-]/g, ch => '\\' + ch);
  const attr = value => String(value || '').replace(/\\/g, '\\\\').replace(/"/g, '\\"');
  const visible = el => {
    const box = el.getBoundingClientRect();
    const style = getComputedStyle(el);
    return box.width > 0 && box.height > 0 && style.display !== 'none' && style.visibility !== 'hidden' && style.opacity !== '0';
  };
  const cssPath = el => {
    const parts = [];
    let cur = el;
    while (cur && cur.nodeType === 1 && parts.length < 8) {
      let part = cur.tagName.toLowerCase();
      if (cur.id && document.querySelectorAll('#' + esc(cur.id)).length === 1) {
        parts.unshift('#' + esc(cur.id));
        break;
      }
      const siblings = cur.parentElement ? Array.from(cur.parentElement.children).filter(x => x.tagName === cur.tagName) : [];
      if (siblings.length > 1) part += ':nth-of-type(' + (siblings.indexOf(cur) + 1) + ')';
      parts.unshift(part);
      cur = cur.parentElement;
    }
    return parts.join(' > ');
  };
  const selectorFor = el => {
	const testidName = el.hasAttribute('data-testid') ? 'data-testid' : (el.hasAttribute('data-test-id') ? 'data-test-id' : '');
	const testid = testidName ? el.getAttribute(testidName) : '';
	const preferred = [];
	if (testid) preferred.push('[' + testidName + '="' + attr(testid) + '"]');
    if (el.id) preferred.push('#' + esc(el.id));
    if (el.name) preferred.push(el.tagName.toLowerCase() + '[name="' + attr(el.name) + '"]');
    const aria = el.getAttribute('aria-label') || '';
    if (aria) preferred.push('[aria-label="' + attr(aria) + '"]');
    const path = cssPath(el);
    if (path) preferred.push(path);
    return {selector: preferred[0] || path, alt_selectors: Array.from(new Set(preferred.slice(1))) };
  };
  const record = el => {
    const tag = el.tagName.toLowerCase();
    const role = el.getAttribute('role') || '';
    const classes = Array.from(el.classList || []).slice(0, 5);
    const box = el.getBoundingClientRect();
    const selected = selectorFor(el);
    const inputType = tag === 'input' ? (el.type || 'text') : '';
    let type = 'clickable', action = 'click';
    if (tag === 'a') type = 'link';
    else if (tag === 'select' || role === 'combobox' || role === 'listbox') { type = 'select'; action = 'select'; }
    else if (inputType === 'checkbox' || role === 'checkbox' || role === 'switch') { type = 'checkbox'; action = 'toggle'; }
    else if (inputType === 'radio' || role === 'radio') { type = 'radio'; action = 'toggle'; }
    else if (tag === 'input' || tag === 'textarea' || el.isContentEditable) { type = 'input'; action = 'type'; }
    else if (role === 'row' || tag === 'tr') type = 'row';
    else if (tag === 'button' || role === 'button' || role === 'tab') type = 'button';
    const label = clean(el.getAttribute('aria-label') || el.innerText || el.placeholder || el.title || el.alt).slice(0, 120);
    return {
      ...selected, tag_name: tag, id: el.id || '', name: el.name || '', classes,
      text_content: clean(el.innerText).slice(0, 200), aria_label: el.getAttribute('aria-label') || '',
      data_testid: el.getAttribute('data-testid') || el.getAttribute('data-test-id') || '', role,
      input_type: inputType, row_key: el.getAttribute('data-row-id') || el.getAttribute('data-row-key') || '',
      row_index: el.getAttribute('aria-rowindex') || el.getAttribute('data-rowindex') || el.getAttribute('row-index') || '',
      bounding_box: {x: Math.round(box.x), y: Math.round(box.y), width: Math.round(box.width), height: Math.round(box.height)},
      type, action, label, href: tag === 'a' ? (el.href || '') : '', disabled: !!el.disabled, checked: !!el.checked
    };
  };

  const selectors = {
    buttons: 'button,input[type="submit"],input[type="button"],[role="button"],[role="tab"],[role="menuitem"],[role="option"],[role="switch"],[role="checkbox"],[role="radio"]',
    inputs: 'input:not([type="hidden"]):not([type="submit"]):not([type="button"]),textarea,[contenteditable="true"]',
    links: 'a[href]',
    selects: 'select,[role="combobox"],[role="listbox"]'
  };
  const allSelector = Object.values(selectors).join(',');
  const chosenSelector = opts.filter === 'all' ? allSelector : (selectors[opts.filter] || allSelector);
  const allInteractive = Array.from(document.querySelectorAll(chosenSelector)).filter(el => !opts.visibleOnly || visible(el));
  const interactive = allInteractive.slice(0, maxItems).map(record);

  let navAll = Array.from(document.querySelectorAll('a[href]')).filter(visible).map(record);
  navAll = navAll.map(item => ({...item, external: (() => { try { return new URL(item.href, location.href).origin !== location.origin; } catch (_) { return false; } })()}));
  if (opts.internalOnly) navAll = navAll.filter(item => !item.external);
  const navigation = navAll.slice(0, maxItems);

  const gridRoots = Array.from(document.querySelectorAll('[role="grid"],[role="treegrid"],[role="table"],table,.ag-root,.MuiDataGrid-root,[data-grid]'));
  const grids = gridRoots.slice(0, maxItems).map(grid => {
    const rows = Array.from(grid.querySelectorAll('[role="row"],tr,.ag-row,.MuiDataGrid-row,[data-row-id],[data-row-key]'));
    const visibleRows = rows.filter(visible);
    const columns = grid.querySelectorAll('[role="columnheader"],th').length;
    return {type: grid.getAttribute('role') || (grid.tagName.toLowerCase() === 'table' ? 'html-table' : 'grid'), label: clean(grid.getAttribute('aria-label') || grid.caption?.innerText).slice(0, 120), row_count: rows.length, visible_row_count: visibleRows.length, column_count: columns, sample_rows: (visibleRows.length ? visibleRows : rows).slice(0, Math.min(3, maxItems)).map(record)};
  });

  const hidden = [];
  document.querySelectorAll('details').forEach(details => {
    const trigger = details.querySelector('summary');
    const body = Array.from(details.children).find(el => el.tagName !== 'SUMMARY');
    hidden.push({element: trigger ? record(trigger) : {}, type: 'details', trigger: clean(trigger?.innerText).slice(0, 120), state: details.open ? 'expanded' : 'collapsed', expandable: !details.open && !!trigger, content_preview: clean(body?.innerText).slice(0, 160), interactive_elements: details.querySelectorAll(allSelector).length});
  });
  document.querySelectorAll('[aria-expanded]').forEach(trigger => {
    const expanded = trigger.getAttribute('aria-expanded') === 'true';
    const panel = document.getElementById(trigger.getAttribute('aria-controls') || '');
    hidden.push({element: record(trigger), type: 'aria-accordion', trigger: clean(trigger.innerText || trigger.getAttribute('aria-label')).slice(0, 120), state: expanded ? 'expanded' : 'collapsed', expandable: !expanded, content_preview: clean(panel?.innerText).slice(0, 160), interactive_elements: panel ? panel.querySelectorAll(allSelector).length : 0});
  });
  document.querySelectorAll('[role="tabpanel"]').forEach(panel => {
    const isHidden = panel.hidden || panel.getAttribute('aria-hidden') === 'true' || !visible(panel);
    if (!isHidden) return;
    const trigger = document.querySelector('[aria-controls="' + attr(panel.id) + '"]');
    hidden.push({element: trigger ? record(trigger) : {}, type: 'tab-panel', trigger: clean(trigger?.innerText).slice(0, 120), state: 'hidden', expandable: !!trigger, content_preview: clean(panel.innerText).slice(0, 160), interactive_elements: panel.querySelectorAll(allSelector).length});
  });

  return {
    url: location.href, title: document.title || '', loading: document.readyState !== 'complete',
    has_dialog: !!document.querySelector('[role="dialog"],dialog[open]'),
    interactive, navigation, grids, hidden: hidden.slice(0, maxItems),
    interactive_total: allInteractive.length, navigation_total: navAll.length,
    grid_total: gridRoots.length, hidden_total: hidden.length
  };
}`
