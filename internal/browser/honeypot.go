// Package browser provides honeypot detection using Mangle rules.
// Adapted from scraper_service for Cortex 1.5.0 Safety Layer.
package browser

import (
	"fmt"
	"log"
	"strings"

	"codenerd/internal/mangle"

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

// HoneypotDetector coordinates honeypot detection using Mangle rules.
type HoneypotDetector struct {
	engine *mangle.Engine
}

// NewHoneypotDetector creates a new honeypot detector.
func NewHoneypotDetector(engine *mangle.Engine) *HoneypotDetector {
	return &HoneypotDetector{engine: engine}
}

// AnalyzePage scans a page for honeypot elements.
func (d *HoneypotDetector) AnalyzePage(page *rod.Page) ([]DetectionResult, error) {
	// First, emit facts about page elements
	if err := d.emitPageFacts(page); err != nil {
		return nil, fmt.Errorf("failed to emit page facts: %w", err)
	}

	// Query for honeypot elements using Mangle rules
	honeypots := d.engine.EvaluateRule("is_honeypot")

	var results []DetectionResult
	for _, hp := range honeypots {
		if len(hp.Args) > 0 {
			elemID := fmt.Sprintf("%v", hp.Args[0])
			result := DetectionResult{
				ElementID:  elemID,
				Reasons:    d.getHoneypotReasons(elemID),
				Confidence: d.calculateConfidence(elemID),
			}
			results = append(results, result)
		}
	}

	return results, nil
}

// emitPageFacts extracts element information and pushes as Mangle facts.
func (d *HoneypotDetector) emitPageFacts(page *rod.Page) error {
	// Get all clickable/interactive elements
	elements, err := page.Elements("a, button, input, [onclick], [role='button'], [role='link']")
	if err != nil {
		return err
	}

	for i, el := range elements {
		elemID := fmt.Sprintf("elem_%d", i)

		// Get tag name
		tagName, err := el.Eval(`() => this.tagName.toLowerCase()`)
		if err != nil {
			continue
		}
		d.engine.PushFact("element", elemID, tagName.Value.String(), "")

		// Get computed styles
		styles, err := d.getComputedStyles(el)
		if err == nil {
			for prop, value := range styles {
				d.engine.PushFact("css_property", elemID, prop, value)
			}
		}

		// Get position
		box, err := el.Shape()
		if err == nil && box != nil && len(box.Quads) > 0 {
			quad := box.Quads[0]
			x := (quad[0] + quad[2] + quad[4] + quad[6]) / 4
			y := (quad[1] + quad[3] + quad[5] + quad[7]) / 4
			width := quad[2] - quad[0]
			height := quad[5] - quad[1]
			d.engine.PushFact("position", elemID,
				fmt.Sprintf("%.0f", x),
				fmt.Sprintf("%.0f", y),
				fmt.Sprintf("%.0f", width),
				fmt.Sprintf("%.0f", height))
		}

		// Get attributes
		attrs, err := d.getAttributes(el)
		if err == nil {
			for name, value := range attrs {
				d.engine.PushFact("attribute", elemID, name, value)
			}
		}

		// Get href for links
		href, err := el.Attribute("href")
		if err == nil && href != nil && *href != "" {
			d.engine.PushFact("link", elemID, *href)
		}
	}

	return nil
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

// getHoneypotReasons returns the reasons an element was flagged as a honeypot.
func (d *HoneypotDetector) getHoneypotReasons(elemID string) []string {
	var reasons []string

	// Check each honeypot rule
	ruleChecks := []struct {
		predicate string
		reason    string
	}{
		{"honeypot_css_hidden", "Hidden via display:none"},
		{"honeypot_css_invisible", "Hidden via visibility:hidden"},
		{"honeypot_opacity_hidden", "Hidden via opacity:0"},
		{"honeypot_offscreen", "Positioned off-screen"},
		{"honeypot_zero_size", "Zero or near-zero size"},
		{"honeypot_aria_hidden", "Marked as aria-hidden"},
		{"honeypot_no_keyboard", "Not keyboard accessible (negative tabindex)"},
		{"honeypot_suspicious_url", "Suspicious URL pattern"},
		{"honeypot_pointer_events_none", "Pointer events disabled"},
		{"honeypot_clip_hidden", "Clipped to zero size"},
		{"honeypot_overflow_hidden", "Content clipped via overflow"},
	}

	for _, check := range ruleChecks {
		facts := d.engine.QueryFacts(check.predicate, elemID)
		if len(facts) > 0 {
			reasons = append(reasons, check.reason)
		}
	}

	return reasons
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

	// Cap at 1.0
	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// IsHoneypot checks if a specific element is a honeypot.
func (d *HoneypotDetector) IsHoneypot(page *rod.Page, selector string) (bool, []string, error) {
	el, err := page.Element(selector)
	if err != nil {
		return false, nil, fmt.Errorf("element not found: %w", err)
	}

	// Emit facts for this element
	elemID := "check_elem"

	// Get computed styles
	styles, err := d.getComputedStyles(el)
	if err == nil {
		for prop, value := range styles {
			d.engine.PushFact("css_property", elemID, prop, value)
		}
	}

	// Get position
	box, err := el.Shape()
	if err == nil && box != nil && len(box.Quads) > 0 {
		quad := box.Quads[0]
		x := (quad[0] + quad[2] + quad[4] + quad[6]) / 4
		y := (quad[1] + quad[3] + quad[5] + quad[7]) / 4
		width := quad[2] - quad[0]
		height := quad[5] - quad[1]
		d.engine.PushFact("position", elemID,
			fmt.Sprintf("%.0f", x),
			fmt.Sprintf("%.0f", y),
			fmt.Sprintf("%.0f", width),
			fmt.Sprintf("%.0f", height))
	}

	// Get attributes
	attrs, err := d.getAttributes(el)
	if err == nil {
		for name, value := range attrs {
			d.engine.PushFact("attribute", elemID, name, value)
		}
	}

	// Get href
	href, err := el.Attribute("href")
	if err == nil && href != nil && *href != "" {
		d.engine.PushFact("link", elemID, *href)
	}

	// Check for honeypot
	reasons := d.getHoneypotReasons(elemID)
	isHoneypot := len(reasons) > 0

	return isHoneypot, reasons, nil
}

// GetSafeLinks returns all links that are not honeypots.
func (d *HoneypotDetector) GetSafeLinks(page *rod.Page) ([]Link, error) {
	// First analyze the page
	if err := d.emitPageFacts(page); err != nil {
		return nil, fmt.Errorf("failed to analyze page: %w", err)
	}

	// Get all links
	elements, err := page.Elements("a[href]")
	if err != nil {
		return nil, fmt.Errorf("failed to get links: %w", err)
	}

	var links []Link
	for i, el := range elements {
		elemID := fmt.Sprintf("elem_%d", i)

		href, err := el.Attribute("href")
		if err != nil || href == nil || *href == "" {
			continue
		}

		text, err := el.Text()
		if err != nil {
			text = ""
		}

		// Check if this element is a honeypot
		reasons := d.getHoneypotReasons(elemID)
		isHoneypot := len(reasons) > 0

		link := Link{
			Selector:   fmt.Sprintf("a[href='%s']", *href),
			Href:       *href,
			Text:       strings.TrimSpace(text),
			IsHoneypot: isHoneypot,
		}

		if isHoneypot {
			link.HoneypotReasons = reasons
			log.Printf("Detected honeypot link: %s (reasons: %v)", *href, reasons)
		} else {
			links = append(links, link)
		}
	}

	return links, nil
}

// GetAllLinksWithAnalysis returns all links with honeypot analysis.
func (d *HoneypotDetector) GetAllLinksWithAnalysis(page *rod.Page) ([]Link, error) {
	if err := d.emitPageFacts(page); err != nil {
		return nil, fmt.Errorf("failed to analyze page: %w", err)
	}

	elements, err := page.Elements("a[href]")
	if err != nil {
		return nil, fmt.Errorf("failed to get links: %w", err)
	}

	var links []Link
	for i, el := range elements {
		elemID := fmt.Sprintf("elem_%d", i)

		href, err := el.Attribute("href")
		if err != nil || href == nil || *href == "" {
			continue
		}

		text, err := el.Text()
		if err != nil {
			text = ""
		}

		reasons := d.getHoneypotReasons(elemID)

		link := Link{
			Selector:        fmt.Sprintf("a[href='%s']", *href),
			Href:            *href,
			Text:            strings.TrimSpace(text),
			IsHoneypot:      len(reasons) > 0,
			HoneypotReasons: reasons,
		}

		links = append(links, link)
	}

	return links, nil
}

// HoneypotRules returns the Mangle rules for honeypot detection.
// These rules should be loaded into the engine schema.
func HoneypotRules() string {
	return `
# Honeypot Detection Rules
# These rules derive is_honeypot(ElemID) based on CSS and attribute patterns

# CSS-based hiding
Decl honeypot_css_hidden(elem: string).
honeypot_css_hidden(Elem) :- css_property(Elem, "display", "none").

Decl honeypot_css_invisible(elem: string).
honeypot_css_invisible(Elem) :- css_property(Elem, "visibility", "hidden").

Decl honeypot_opacity_hidden(elem: string).
honeypot_opacity_hidden(Elem) :- css_property(Elem, "opacity", "0").

# Position-based hiding (off-screen)
Decl honeypot_offscreen(elem: string).
honeypot_offscreen(Elem) :-
    position(Elem, X, _, _, _),
    fn:int64:lt(X, -1000).
honeypot_offscreen(Elem) :-
    position(Elem, _, Y, _, _),
    fn:int64:lt(Y, -1000).

# Zero or near-zero size
Decl honeypot_zero_size(elem: string).
honeypot_zero_size(Elem) :-
    position(Elem, _, _, W, H),
    fn:int64:lt(W, 2),
    fn:int64:lt(H, 2).

# ARIA hidden
Decl honeypot_aria_hidden(elem: string).
honeypot_aria_hidden(Elem) :- attribute(Elem, "aria-hidden", "true").

# Negative tabindex (not keyboard accessible)
Decl honeypot_no_keyboard(elem: string).
honeypot_no_keyboard(Elem) :- attribute(Elem, "tabindex", "-1").

# Pointer events disabled
Decl honeypot_pointer_events_none(elem: string).
honeypot_pointer_events_none(Elem) :- css_property(Elem, "pointerEvents", "none").

# Suspicious URL patterns
Decl honeypot_suspicious_url(elem: string).
honeypot_suspicious_url(Elem) :-
    link(Elem, Href),
    fn:string:contains(Href, "honeypot").
honeypot_suspicious_url(Elem) :-
    link(Elem, Href),
    fn:string:contains(Href, "trap").
honeypot_suspicious_url(Elem) :-
    link(Elem, Href),
    fn:string:contains(Href, "captcha").

# Main honeypot derivation
Decl is_honeypot(elem: string).
is_honeypot(Elem) :- honeypot_css_hidden(Elem).
is_honeypot(Elem) :- honeypot_css_invisible(Elem).
is_honeypot(Elem) :- honeypot_opacity_hidden(Elem).
is_honeypot(Elem) :- honeypot_offscreen(Elem).
is_honeypot(Elem) :- honeypot_zero_size(Elem).
is_honeypot(Elem) :- honeypot_aria_hidden(Elem).
is_honeypot(Elem) :- honeypot_pointer_events_none(Elem).
is_honeypot(Elem) :- honeypot_suspicious_url(Elem).

# High confidence honeypot (multiple indicators)
Decl high_confidence_honeypot(elem: string).
high_confidence_honeypot(Elem) :-
    honeypot_css_hidden(Elem),
    honeypot_zero_size(Elem).
high_confidence_honeypot(Elem) :-
    honeypot_offscreen(Elem),
    honeypot_no_keyboard(Elem).
`
}

// BrowserSchemas returns the Mangle schemas for browser facts.
func BrowserSchemas() string {
	return `
# Browser DOM Facts
Decl element(id: string, tag: string, parent: string).
Decl css_property(elem: string, property: string, value: string).
Decl position(elem: string, x: string, y: string, width: string, height: string).
Decl attribute(elem: string, name: string, value: string).
Decl link(elem: string, href: string).

# DOM Tree
Decl dom_node(id: string, tag: string, text: string, parent: string).
Decl dom_text(id: string, text: string).
Decl dom_attr(id: string, key: string, value: string).
Decl dom_layout(id: string, x: float64, y: float64, width: float64, height: float64, visible: string).

# React Fiber
Decl react_component(fiber_id: string, name: string, parent: string).
Decl react_prop(fiber_id: string, key: string, value: string).
Decl react_state(fiber_id: string, hook_index: int64, value: string).
Decl dom_mapping(fiber_id: string, dom_id: string).

# Network
Decl net_request(request_id: string, method: string, url: string, initiator_type: string, timestamp: int64).
Decl net_response(request_id: string, status: int64, latency: int64, duration: int64).
Decl net_header(request_id: string, direction: string, key: string, value: string).
Decl request_initiator(request_id: string, initiator_type: string, parent_ref: string).

# Events
Decl navigation_event(session_id: string, url: string, timestamp: int64).
Decl current_url(session_id: string, url: string).
Decl console_event(level: string, message: string, timestamp: int64).
Decl click_event(element_id: string, timestamp: int64).
Decl input_event(element_id: string, value: string, timestamp: int64).
Decl state_change(name: string, value: string, timestamp: int64).

# Interactive elements
Decl interactable(id: string, elem_type: string).
Decl geometry(id: string, x: int64, y: int64, width: int64, height: int64).
Decl visible(id: string, is_visible: string).
`
}
