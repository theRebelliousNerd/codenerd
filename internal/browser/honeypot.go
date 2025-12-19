// Package browser provides honeypot detection using Mangle rules.
// Adapted from scraper_service for Cortex 1.5.0 Safety Layer.
package browser

import (
	"fmt"
	"strings"

	"codenerd/internal/logging"
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
	timer := logging.StartTimer(logging.CategoryBrowser, "Honeypot page analysis")
	defer timer.Stop()

	logging.Browser("Analyzing page for honeypot elements")
	// First, emit facts about page elements
	if err := d.emitPageFacts(page); err != nil {
		logging.BrowserError("Failed to emit page facts for honeypot detection: %v", err)
		return nil, fmt.Errorf("failed to emit page facts: %w", err)
	}

	// Query for honeypot elements using Mangle rules
	logging.BrowserDebug("Evaluating is_honeypot rule")
	honeypots := d.engine.EvaluateRule("is_honeypot")
	logging.BrowserDebug("Found %d potential honeypot elements", len(honeypots))

	var results []DetectionResult
	for _, hp := range honeypots {
		if len(hp.Args) > 0 {
			elemID := fmt.Sprintf("%v", hp.Args[0])
			result := DetectionResult{
				ElementID:  elemID,
				Reasons:    d.getHoneypotReasons(elemID),
				Confidence: d.calculateConfidence(elemID),
			}
			logging.BrowserDebug("Honeypot detected: %s (confidence=%.2f, reasons=%v)", elemID, result.Confidence, result.Reasons)
			results = append(results, result)
		}
	}

	logging.Browser("Honeypot analysis complete: %d elements detected", len(results))
	return results, nil
}

// emitPageFacts extracts element information and pushes as Mangle facts.
func (d *HoneypotDetector) emitPageFacts(page *rod.Page) error {
	logging.BrowserDebug("Extracting page facts for honeypot detection")
	// Get all clickable/interactive elements
	elements, err := page.Elements("a, button, input, [onclick], [role='button'], [role='link']")
	if err != nil {
		logging.BrowserError("Failed to get page elements: %v", err)
		return err
	}
	logging.BrowserDebug("Found %d interactive elements to analyze", len(elements))

	for i, el := range elements {
		elemID := fmt.Sprintf("elem_%d", i)

		// Get tag name
		tagName, err := el.Eval(`() => this.tagName.toLowerCase()`)
		if err != nil {
			logging.BrowserDebug("Failed to get tag name for element %d: %v", i, err)
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
	logging.BrowserDebug("Checking if element is honeypot: %s", selector)
	el, err := page.Element(selector)
	if err != nil {
		logging.BrowserError("Element not found for honeypot check: %s - %v", selector, err)
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

	if isHoneypot {
		logging.BrowserDebug("Element %s IS a honeypot (reasons=%v)", selector, reasons)
	} else {
		logging.BrowserDebug("Element %s is NOT a honeypot", selector)
	}
	return isHoneypot, reasons, nil
}

// GetSafeLinks returns all links that are not honeypots.
func (d *HoneypotDetector) GetSafeLinks(page *rod.Page) ([]Link, error) {
	logging.Browser("Getting safe links from page")
	// First analyze the page
	if err := d.emitPageFacts(page); err != nil {
		logging.BrowserError("Failed to analyze page for safe links: %v", err)
		return nil, fmt.Errorf("failed to analyze page: %w", err)
	}

	// Get all links
	elements, err := page.Elements("a[href]")
	if err != nil {
		logging.BrowserError("Failed to get links: %v", err)
		return nil, fmt.Errorf("failed to get links: %w", err)
	}
	logging.BrowserDebug("Found %d links to analyze", len(elements))

	var links []Link
	honeypotCount := 0
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
			honeypotCount++
			link.HoneypotReasons = reasons
			logging.BrowserDebug("Detected honeypot link: %s (reasons: %v)", *href, reasons)
		} else {
			links = append(links, link)
		}
	}

	logging.Browser("Safe links analysis complete: %d safe, %d honeypots filtered", len(links), honeypotCount)
	return links, nil
}

// GetAllLinksWithAnalysis returns all links with honeypot analysis.
func (d *HoneypotDetector) GetAllLinksWithAnalysis(page *rod.Page) ([]Link, error) {
	logging.Browser("Getting all links with honeypot analysis")
	if err := d.emitPageFacts(page); err != nil {
		logging.BrowserError("Failed to analyze page for link analysis: %v", err)
		return nil, fmt.Errorf("failed to analyze page: %w", err)
	}

	elements, err := page.Elements("a[href]")
	if err != nil {
		logging.BrowserError("Failed to get links for analysis: %v", err)
		return nil, fmt.Errorf("failed to get links: %w", err)
	}
	logging.BrowserDebug("Analyzing %d links with honeypot detection", len(elements))

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
