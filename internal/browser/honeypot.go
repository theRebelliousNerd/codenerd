// Package browser provides honeypot detection using Mangle rules.
// Adapted from scraper_service for Cortex 1.5.0 Safety Layer.
package browser

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"

	"codenerd/internal/logging"
	"codenerd/internal/mangle"
	"codenerd/internal/types"

	"github.com/go-rod/rod"
)

// DetectionResult represents a honeypot detection result.
type DetectionResult struct {
	ElementID  string   `json:"element_id"`
	Selector   string   `json:"selector"`
	Reasons    []string `json:"reasons"`
	Confidence float64  `json:"confidence"`
	TagName    string   `json:"tag_name"`
	Href       string   `json:"href,omitempty"`
}

// Link represents a link on the page.
type Link struct {
	Selector        string   `json:"selector"`
	Href            string   `json:"href"`
	Text            string   `json:"text"`
	IsHoneypot      bool     `json:"is_honeypot"`
	HoneypotReasons []string `json:"honeypot_reasons,omitempty"`
}

// HoneypotStore is the fact substrate the detector needs: it asserts element
// evidence and reads back what the kernel derived from it. Taking an interface
// rather than *mangle.Engine lets the SessionManager route detection through
// whatever sink it was built with.
type HoneypotStore interface {
	PushFact(predicate string, args ...any) error
	QueryFacts(predicate string, args ...string) []mangle.Fact
}

// HoneypotDetector coordinates honeypot detection using Mangle rules.
type HoneypotDetector struct {
	engine HoneypotStore
}

// NewHoneypotDetector creates a new honeypot detector.
func NewHoneypotDetector(engine HoneypotStore) *HoneypotDetector {
	return &HoneypotDetector{engine: engine}
}

// honeypotScopeCounter namespaces the element IDs a single analysis asserts.
//
// Every previous entry point reused fixed IDs ("elem_0..N", "check_elem"). The
// fact store is monotonic, so those IDs accumulated evidence across pages and
// sessions: once any checked element was hidden, every later element assigned
// the same ID inherited its honeypot verdict. Scoping each call fixes that.
var honeypotScopeCounter atomic.Uint64

func newHoneypotScope() string {
	return fmt.Sprintf("hp%d", honeypotScopeCounter.Add(1))
}

// AnalyzePage scans a page for honeypot elements.
func (d *HoneypotDetector) AnalyzePage(page *rod.Page) ([]DetectionResult, error) {
	timer := logging.StartTimer(logging.CategoryBrowser, "Honeypot page analysis")
	defer timer.Stop()

	logging.Browser("Analyzing page for honeypot elements")
	// First, emit facts about page elements
	elemIDs, err := d.emitPageFacts(page)
	if err != nil {
		logging.BrowserError("Failed to emit page facts for honeypot detection: %v", err)
		return nil, fmt.Errorf("failed to emit page facts: %w", err)
	}

	// Query the kernel's verdict for exactly the elements this call asserted.
	// Scanning every is_honeypot fact would also return elements from earlier
	// analyses that are still in the monotonic store.
	logging.BrowserDebug("Evaluating is_honeypot for %d elements", len(elemIDs))
	var results []DetectionResult
	for _, elemID := range elemIDs {
		if !d.isHoneypot(elemID) {
			continue
		}
		result := DetectionResult{
			ElementID:  elemID,
			Reasons:    d.getHoneypotReasons(elemID),
			Confidence: d.calculateConfidence(elemID),
		}
		logging.BrowserDebug("Honeypot detected: %s (confidence=%.2f, reasons=%v)", elemID, result.Confidence, result.Reasons)
		results = append(results, result)
	}

	logging.Browser("Honeypot analysis complete: %d elements detected", len(results))
	return results, nil
}

// emitPageFacts extracts element information and pushes it as Mangle facts,
// returning the scoped IDs it assigned in DOM order.
func (d *HoneypotDetector) emitPageFacts(page *rod.Page) ([]string, error) {
	return d.emitFactsFor(page, "a, button, input, [onclick], [role='button'], [role='link']")
}

// emitFactsFor asserts evidence for every element matching selector.
func (d *HoneypotDetector) emitFactsFor(page *rod.Page, selector string) ([]string, error) {
	if d.engine == nil {
		return nil, fmt.Errorf("honeypot detector has no fact store")
	}
	logging.BrowserDebug("Extracting page facts for honeypot detection")
	elements, err := page.Elements(selector)
	if err != nil {
		logging.BrowserError("Failed to get page elements: %v", err)
		return nil, err
	}
	logging.BrowserDebug("Found %d interactive elements to analyze", len(elements))

	scope := newHoneypotScope()
	ids := make([]string, 0, len(elements))
	for i, el := range elements {
		elemID := fmt.Sprintf("%s_%d", scope, i)

		// Get tag name
		tagName, err := el.Eval(`() => this.tagName.toLowerCase()`)
		if err != nil {
			logging.BrowserDebug("Failed to get tag name for element %d: %v", i, err)
			continue
		}
		ids = append(ids, elemID)
		d.pushFact("element", elemID, tagName.Value.String(), "")
		d.emitElementEvidence(el, elemID)
	}

	return ids, nil
}

// emitElementEvidence asserts the styles, geometry, attributes, and link shape
// of one element under elemID.
func (d *HoneypotDetector) emitElementEvidence(el *rod.Element, elemID string) {
	styles, err := d.getComputedStyles(el)
	if err == nil {
		for prop, value := range styles {
			d.pushFact("css_property", elemID, prop, value)
		}
		// clip is a rectangle, not a keyword: parse it in Go and let the rule
		// file decide what counts as collapsed.
		if top, right, bottom, left, ok := parseClipRect(styles["clip"]); ok {
			d.pushFact("css_clip_rect", elemID, top, right, bottom, left)
		}
	}

	// position/5 is declared bound [/string, /number, /number, /number,
	// /number]. Formatting the coordinates as strings stored ast.String terms,
	// which never unify with the `X < -1000` and `W < 2` comparisons, so
	// honeypot_offscreen and honeypot_zero_size could not fire on a live page.
	if box, boxErr := el.Shape(); boxErr == nil && box != nil && len(box.Quads) > 0 {
		quad := box.Quads[0]
		x := (quad[0] + quad[2] + quad[4] + quad[6]) / 4
		y := (quad[1] + quad[3] + quad[5] + quad[7]) / 4
		width := quad[2] - quad[0]
		height := quad[5] - quad[1]
		d.pushFact("position", elemID, int64(x), int64(y), int64(width), int64(height))
	}

	attrs, err := d.getAttributes(el)
	if err == nil {
		for name, value := range attrs {
			d.pushFact("attribute", elemID, name, value)
		}
	}

	href, err := el.Attribute("href")
	if err == nil && href != nil && *href != "" {
		d.pushFact("link", elemID, *href)
		for _, pattern := range ClassifyLinkURL(*href) {
			d.pushFact("link_url_pattern", elemID, pattern)
		}
	}
}

func (d *HoneypotDetector) pushFact(predicate string, args ...any) {
	if d.engine == nil {
		return
	}
	if err := d.engine.PushFact(predicate, args...); err != nil {
		logging.BrowserDebug("honeypot fact %s rejected: %v", predicate, err)
	}
}

// getComputedStyles returns relevant computed styles for honeypot detection.
func (d *HoneypotDetector) getComputedStyles(el *rod.Element) (map[string]string, error) {
	result, err := el.Eval(`() => {
		const styles = window.getComputedStyle(this);
		return {
			display: styles.display,
			visibility: styles.visibility,
			opacity: styles.opacity,
			position: styles.position,
			left: styles.left,
			top: styles.top,
			width: styles.width,
			height: styles.height,
			overflow: styles.overflow,
			clip: styles.clip,
			pointerEvents: styles.pointerEvents
		};
	}`)
	if err != nil {
		return nil, err
	}

	styles := make(map[string]string)
	obj := result.Value.Map()
	for k, v := range obj {
		styles[k] = v.String()
	}

	return styles, nil
}

// getAttributes returns element attributes.
func (d *HoneypotDetector) getAttributes(el *rod.Element) (map[string]string, error) {
	result, err := el.Eval(`() => {
		const attrs = {};
		for (const attr of this.attributes) {
			attrs[attr.name] = attr.value;
		}
		return attrs;
	}`)
	if err != nil {
		return nil, err
	}

	attrs := make(map[string]string)
	obj := result.Value.Map()
	for k, v := range obj {
		attrs[k] = v.String()
	}

	return attrs, nil
}

// honeypotReasonCode pairs a Mangle reason code with its operator-facing text.
type honeypotReasonCode struct {
	Code string
	Text string
}

// honeypotReasonCodes is the presentation layer for honeypot_reason/2 in
// internal/core/defaults/policy/browser_honeypot.mg. It is ordered so reports
// are deterministic, and it must stay in exact sync with the rule file - see
// TestHoneypotReasonCodes_WhenComparedToPolicy_ShouldMatchExactly. The previous
// Go-side checklist queried predicates directly and had drifted: it named two
// predicates (clip/overflow) that no rule derived and treated tabindex="-1"
// alone as a honeypot even though is_honeypot never did.
var honeypotReasonCodes = []honeypotReasonCode{
	{"/css_hidden", "Hidden via display:none"},
	{"/css_invisible", "Hidden via visibility:hidden"},
	{"/opacity_hidden", "Hidden via opacity:0"},
	{"/offscreen", "Positioned off-screen"},
	{"/zero_size", "Zero or near-zero size"},
	{"/aria_hidden", "Marked as aria-hidden"},
	{"/no_keyboard", "Not keyboard accessible (negative tabindex)"},
	{"/suspicious_url", "Suspicious URL pattern"},
	{"/pointer_events_none", "Pointer events disabled"},
	{"/clip_hidden", "Clipped to zero size"},
	{"/overflow_hidden", "Content clipped via overflow"},
}

// getHoneypotReasons returns the reasons an element was flagged as a honeypot.
func (d *HoneypotDetector) getHoneypotReasons(elemID string) []string {
	if d.engine == nil {
		return nil
	}
	derived := make(map[string]bool)
	for _, fact := range d.engine.QueryFacts("honeypot_reason", elemID) {
		if len(fact.Args) < 2 {
			continue
		}
		code := types.ExtractString(fact.Args[1])
		if !strings.HasPrefix(code, "/") {
			code = "/" + code
		}
		derived[code] = true
	}

	var reasons []string
	for _, entry := range honeypotReasonCodes {
		if derived[entry.Code] {
			reasons = append(reasons, entry.Text)
		}
	}
	return reasons
}

// isHoneypot reports the kernel's verdict for an element. It is deliberately
// not "len(reasons) > 0": evidence codes and the verdict are different
// questions, and is_honeypot excludes weak signals like negative tabindex.
func (d *HoneypotDetector) isHoneypot(elemID string) bool {
	if d.engine == nil {
		return false
	}
	return len(d.engine.QueryFacts("is_honeypot", elemID)) > 0
}

// calculateConfidence calculates detection confidence based on reasons.
func (d *HoneypotDetector) calculateConfidence(elemID string) float64 {
	reasons := d.getHoneypotReasons(elemID)
	if len(reasons) == 0 {
		return 0.0
	}

	// More reasons = higher confidence
	// Base confidence for any detection
	confidence := 0.5

	// Add confidence per reason
	confidence += float64(len(reasons)) * 0.15

	// The rule file has its own multi-indicator verdict; honor it rather than
	// letting arithmetic disagree with the kernel.
	if d.engine != nil && len(d.engine.QueryFacts("high_confidence_honeypot", elemID)) > 0 && confidence < 0.9 {
		confidence = 0.9
	}

	// Cap at 1.0
	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// IsHoneypot checks if a specific element is a honeypot.
func (d *HoneypotDetector) IsHoneypot(page *rod.Page, selector string) (bool, []string, error) {
	logging.BrowserDebug("Checking if element is honeypot: %s", selector)
	el, err := page.Element(selector)
	if err != nil {
		logging.BrowserError("Element not found for honeypot check: %s - %v", selector, err)
		return false, nil, fmt.Errorf("element not found: %w", err)
	}
	return d.checkElement(el)
}

// checkElement asserts evidence for a resolved element under a fresh scope and
// returns the kernel's verdict plus the derived reason list.
func (d *HoneypotDetector) checkElement(el *rod.Element) (bool, []string, error) {
	if d.engine == nil {
		return false, nil, fmt.Errorf("honeypot detector has no fact store")
	}
	elemID := newHoneypotScope() + "_check"
	d.emitElementEvidence(el, elemID)

	isHoneypot := d.isHoneypot(elemID)
	reasons := d.getHoneypotReasons(elemID)
	if isHoneypot {
		logging.BrowserDebug("Element %s IS a honeypot (reasons=%v)", elemID, reasons)
	} else {
		logging.BrowserDebug("Element %s is NOT a honeypot", elemID)
	}
	return isHoneypot, reasons, nil
}

// GetSafeLinks returns all links that are not honeypots.
func (d *HoneypotDetector) GetSafeLinks(page *rod.Page) ([]Link, error) {
	logging.Browser("Getting safe links from page")
	links, err := d.analyzeLinks(page)
	if err != nil {
		return nil, err
	}

	safe := make([]Link, 0, len(links))
	honeypotCount := 0
	for _, link := range links {
		if link.IsHoneypot {
			honeypotCount++
			logging.BrowserDebug("Detected honeypot link: %s (reasons: %v)", link.Href, link.HoneypotReasons)
			continue
		}
		safe = append(safe, link)
	}

	logging.Browser("Safe links analysis complete: %d safe, %d honeypots filtered", len(safe), honeypotCount)
	return safe, nil
}

// GetAllLinksWithAnalysis returns all links with honeypot analysis.
func (d *HoneypotDetector) GetAllLinksWithAnalysis(page *rod.Page) ([]Link, error) {
	logging.Browser("Getting all links with honeypot analysis")
	return d.analyzeLinks(page)
}

// analyzeLinks asserts evidence for the anchors on the page and reads back the
// verdict for each.
//
// The link walk used to reuse the IDs assigned by the interactive-element walk
// ("elem_%d"), which enumerates buttons and inputs as well. Any non-anchor
// ahead of an anchor shifted the indices, so link N was reported using the
// evidence of some unrelated element. Emitting under the link walk's own scope
// keeps identity and evidence aligned.
func (d *HoneypotDetector) analyzeLinks(page *rod.Page) ([]Link, error) {
	if d.engine == nil {
		return nil, fmt.Errorf("honeypot detector has no fact store")
	}
	elements, err := page.Elements("a[href]")
	if err != nil {
		logging.BrowserError("Failed to get links: %v", err)
		return nil, fmt.Errorf("failed to get links: %w", err)
	}
	logging.BrowserDebug("Analyzing %d links with honeypot detection", len(elements))

	scope := newHoneypotScope()
	links := make([]Link, 0, len(elements))
	for i, el := range elements {
		href, hrefErr := el.Attribute("href")
		if hrefErr != nil || href == nil || *href == "" {
			continue
		}

		elemID := fmt.Sprintf("%s_%d", scope, i)
		d.emitElementEvidence(el, elemID)

		text, textErr := el.Text()
		if textErr != nil {
			text = ""
		}

		isHoneypot := d.isHoneypot(elemID)
		link := Link{
			Selector:   fmt.Sprintf("a[href='%s']", *href),
			Href:       *href,
			Text:       strings.TrimSpace(text),
			IsHoneypot: isHoneypot,
		}
		if isHoneypot {
			link.HoneypotReasons = d.getHoneypotReasons(elemID)
		}
		links = append(links, link)
	}

	return links, nil
}

// honeypotBaitTokens are path/query tokens that only appear on links meant for
// crawlers. Matching is by whole token after splitting on non-alphanumerics, so
// "/trapani" and "/honeypot-check" are treated differently: the first has no
// bait token, the second does.
var honeypotBaitTokens = map[string]bool{
	"honeypot":    true,
	"honeypots":   true,
	"trap":        true,
	"traps":       true,
	"spamtrap":    true,
	"blackhole":   true,
	"donotclick":  true,
	"dontclick":   true,
	"donotfollow": true,
	"nofollow":    true,
	"nocrawl":     true,
	"badbot":      true,
	"botcatcher":  true,
}

// ClassifyLinkURL returns the Mangle pattern names describing href's shape.
// It measures only; browser_honeypot.mg decides which patterns are traps.
func ClassifyLinkURL(href string) []string {
	trimmed := strings.TrimSpace(href)
	if trimmed == "" {
		return nil
	}

	lower := strings.ToLower(trimmed)
	var patterns []string
	if lower == "#" || strings.HasPrefix(lower, "javascript:") {
		patterns = append(patterns, "/empty_js_target")
	}

	path := lower
	query := ""
	if parsed, err := url.Parse(trimmed); err == nil {
		path = strings.ToLower(parsed.Path)
		query = strings.ToLower(parsed.RawQuery)
		if parsed.Fragment != "" {
			path += "/" + strings.ToLower(parsed.Fragment)
		}
	} else if index := strings.IndexByte(lower, '?'); index >= 0 {
		path, query = lower[:index], lower[index+1:]
	}

	if hasBaitToken(path) {
		patterns = append(patterns, "/bait_path")
	}
	if hasBaitToken(query) {
		patterns = append(patterns, "/trap_query")
	}
	return patterns
}

func hasBaitToken(value string) bool {
	if value == "" {
		return false
	}
	tokens := strings.FieldsFunc(value, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	})
	// Windows of up to three adjacent tokens are joined before matching so
	// "/do-not-click" and "/do_not_follow" read the same as "/donotclick".
	// Whole-token matching (rather than substring) keeps "/trapani" clean.
	for i := range tokens {
		t0 := tokens[i]
		if honeypotBaitTokens[t0] {
			return true
		}

		if i+1 < len(tokens) {
			t1 := t0 + tokens[i+1]
			if honeypotBaitTokens[t1] {
				return true
			}

			if i+2 < len(tokens) {
				t2 := t1 + tokens[i+2]
				if honeypotBaitTokens[t2] {
					return true
				}
			}
		}
	}
	return false
}

// parseClipRect parses a computed `clip` value. Chrome reports "auto" when the
// property is unset and "rect(Xpx, Ypx, Zpx, Wpx)" otherwise; the legacy
// space-separated form is still accepted by parsers, so handle both.
func parseClipRect(value string) (top, right, bottom, left int64, ok bool) {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if !strings.HasPrefix(trimmed, "rect(") || !strings.HasSuffix(trimmed, ")") {
		return 0, 0, 0, 0, false
	}
	inner := trimmed[len("rect(") : len(trimmed)-1]
	fields := strings.FieldsFunc(inner, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
	if len(fields) != 4 {
		return 0, 0, 0, 0, false
	}
	values := make([]int64, 4)
	for i, field := range fields {
		parsed, err := parseCSSLength(field)
		if err != nil {
			return 0, 0, 0, 0, false
		}
		values[i] = parsed
	}
	return values[0], values[1], values[2], values[3], true
}

func parseCSSLength(field string) (int64, error) {
	trimmed := strings.TrimSpace(field)
	trimmed = strings.TrimSuffix(trimmed, "px")
	if trimmed == "auto" {
		// An `auto` edge means "not clipped on this side"; report it as a large
		// extent so the collapse rule does not fire on a partially clipped box.
		return 1 << 20, nil
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(trimmed), 64)
	if err != nil {
		return 0, err
	}
	return int64(parsed), nil
}
