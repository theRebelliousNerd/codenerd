package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	browsersecurity "codenerd/internal/browser/security"
	"codenerd/internal/logging"
	"codenerd/internal/mangle"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// startEventStream wires Rod CDP events into the fact sink.
func (m *SessionManager) startEventStream(ctx context.Context, sessionID string, page *rod.Page) {
	if page == nil {
		logging.BrowserDebug("Event stream skipped - nil page for session %s", sessionID)
		return
	}
	ctx = normalizeContext(ctx)
	handleNavigation := func(ev *proto.PageFrameNavigated) {
		if ev == nil || ev.Frame == nil || ev.Frame.ParentID != "" {
			return
		}
		// CDP can deliver queued frame events after a rapid navigate/back pair.
		// Ignore an event that no longer describes the live target; otherwise it
		// would roll metadata backward and invalidate freshly observed refs.
		if info, infoErr := page.Context(ctx).Info(); infoErr == nil && info != nil && info.URL != "" && info.URL != ev.Frame.URL {
			return
		}
		now := time.Now()
		safeURL := m.redactor.SanitizeString(ev.Frame.URL)
		registry := m.Registry(sessionID)
		// EachEvent may replay the page's initial same-URL frame event when the
		// stream is attached. That is not a navigation and must not race the
		// first observation. Explicit manager navigation already invalidates an
		// empty registry; event-driven navigation matters once refs exist.
		if registry != nil && registry.Count() > 0 {
			registry.Clear()
		}
		// A navigation retires everything the previous page asserted, so it is
		// the natural garbage-collection boundary and the point where the
		// per-epoch fact budget resets.
		m.RollSessionEpoch(sessionID)
		facts := []mangle.Fact{
			{
				Predicate: "navigation_event",
				Args:      []any{sessionID, ev.Frame.URL, now.UnixMilli()},
				Timestamp: now,
			},
			{
				Predicate: "current_url",
				Args:      []any{sessionID, ev.Frame.URL},
				Timestamp: now,
			},
		}
		if err := m.addStreamFacts(sessionID, facts); err != nil {
			logging.BrowserError("[session:%s] navigation fact error: %v", sessionID, err)
		}
		m.UpdateMetadata(sessionID, func(s Session) Session {
			s.URL = safeURL
			s.LastActive = now
			return s
		})
	}

	if m.engine == nil {
		// Ref invalidation and session metadata are lifecycle invariants, not fact
		// ingestion features. Keep the lightweight navigation stream alive for
		// standalone managers too.
		logging.BrowserDebug("Starting navigation-only event stream - no engine configured")
		go page.Context(ctx).EachEvent(handleNavigation)()
		return
	}

	logging.BrowserDebug("Starting event stream for session %s", sessionID)
	go func() {
		var wg sync.WaitGroup

		level := strings.ToLower(m.cfg.EventLoggingLevel)
		captureDOM := m.cfg.EnableDOMIngestion && level != "minimal"
		captureHeaders := m.cfg.ShouldIngestHeaders() && level != "minimal"
		consoleErrorsOnly := level == "minimal"
		throttler := newEventThrottler(m.cfg.EventThrottleMs)
		logging.BrowserDebug("Event stream config: level=%s, captureDOM=%v, captureHeaders=%v", level, captureDOM, captureHeaders)

		// Optionally capture initial DOM snapshot
		if captureDOM {
			_ = (proto.DOMEnable{}).Call(page)
			_ = m.captureDOMFacts(ctx, sessionID, page, true)
		}

		// Install lightweight click/input/state trackers
		_, _ = page.Context(ctx).Evaluate(&rod.EvalOptions{
			JS: `
			() => {
				const w = window;
				if (w.__browsernerdHooked) return true;
				w.__browsernerdHooked = true;
				w.__browsernerdEvents = [];
				w.__browsernerdToastKeys = new Set();
				const enqueue = (event) => {
					if (!Array.isArray(w.__browsernerdEvents)) w.__browsernerdEvents = [];
					if (w.__browsernerdEvents.length >= 200) w.__browsernerdEvents.shift();
					w.__browsernerdEvents.push(event);
				};
				const recordToast = (node) => {
					if (!node || node.nodeType !== 1) return;
					const candidates = [];
					if (node.matches && node.matches('[role="alert"],[role="status"],[aria-live],.toast,.snackbar,.notification,.alert')) candidates.push(node);
					if (node.querySelectorAll) candidates.push(...node.querySelectorAll('[role="alert"],[role="status"],[aria-live],.toast,.snackbar,.notification,.alert'));
					for (const el of candidates.slice(0, 20)) {
						const text = String(el.innerText || el.textContent || '').replace(/\s+/g, ' ').trim().slice(0, 500);
						if (!text) continue;
						const marker = [el.getAttribute && el.getAttribute('role'), el.className, el.getAttribute && el.getAttribute('aria-live')].filter(Boolean).join(' ').toLowerCase();
						const level = /error|danger|assertive/.test(marker) ? 'error' : (/warn/.test(marker) ? 'warning' : 'info');
						const key = level + '|' + text;
						if (w.__browsernerdToastKeys.has(key)) continue;
						if (w.__browsernerdToastKeys.size >= 100) w.__browsernerdToastKeys.clear();
						w.__browsernerdToastKeys.add(key);
						enqueue({ type: 'toast', text, level, source: 'dom', ts: Date.now() });
					}
				};

				document.addEventListener('click', (ev) => {
					try {
						const target = ev.target || {};
						const id = target.id || '';
						enqueue({ type: 'click', id, ts: Date.now() });
					} catch (e) {}
				}, true);

				document.addEventListener('input', (ev) => {
					try {
						const target = ev.target || {};
						const id = target.id || target.name || '';
						const value = target.value || '';
					const descriptor = [target.type, target.name, target.id, target.autocomplete, target.getAttribute && target.getAttribute('aria-label')].filter(Boolean).join(' ');
					enqueue({ type: 'input', id, value, descriptor, ts: Date.now() });
					} catch (e) {}
				}, true);

				document.addEventListener('change', (ev) => {
					try {
						const target = ev.target || {};
						const id = target.id || target.name || '';
						const value = target.value || '';
					const descriptor = [target.type, target.name, target.id, target.autocomplete, target.getAttribute && target.getAttribute('aria-label')].filter(Boolean).join(' ');
					enqueue({ type: 'input', id, value, descriptor, ts: Date.now() });
					} catch (e) {}
				}, true);

				let lastDOMEvent = 0;
				const obs = new MutationObserver((mutations) => {
					const now = Date.now();
					if (now - lastDOMEvent >= 100) {
						lastDOMEvent = now;
						enqueue({ type: 'dom', ts: now });
					}
					mutations.forEach((m) => {
						if (m.type === 'attributes' && m.attributeName && m.attributeName.startsWith('data-state')) {
							const val = (m.target && m.target.getAttribute) ? (m.target.getAttribute(m.attributeName) || '') : '';
							enqueue({ type: 'state', name: m.attributeName, value: val, ts: now });
						}
						if (m.type === 'childList') Array.from(m.addedNodes || []).forEach(recordToast);
					});
				});
				obs.observe(document.documentElement || document.body, { attributes: true, childList: true, subtree: true });
				recordToast(document.body);
				return true;
			}
			`,
			ByValue:      true,
			AwaitPromise: true,
		})

		// Navigation events
		waitNav := page.Context(ctx).EachEvent(handleNavigation)

		// Console, network, and DOM streams
		type requestState struct {
			started   time.Time
			method    string
			url       string
			initType  string
			factAdded bool
		}
		var requestMu sync.Mutex
		requestStates := make(map[proto.NetworkRequestID]requestState)
		waitRest := page.Context(ctx).EachEvent(
			func(ev *proto.RuntimeConsoleAPICalled) {
				if consoleErrorsOnly && ev.Type != proto.RuntimeConsoleAPICalledTypeError && ev.Type != proto.RuntimeConsoleAPICalledTypeWarning {
					return
				}
				if !throttler.Allow("console") {
					return
				}
				now := time.Now()
				msg := stringifyConsoleArgs(ev.Args)
				if err := m.addStreamFacts(sessionID, []mangle.Fact{{
					Predicate: "console_event",
					Args:      []any{sessionID, string(ev.Type), msg, now.UnixMilli()},
					Timestamp: now,
				}}); err != nil {
					logging.BrowserError("[session:%s] console fact error: %v", sessionID, err)
				}
			},
			func(ev *proto.NetworkRequestWillBeSent) {
				now := time.Now()
				initiatorType := ""
				initiatorID := ""
				initiatorScript := ""
				initiatorLineNo := 0

				if ev.Initiator != nil {
					initiatorType = string(ev.Initiator.Type)
					if ev.Initiator.RequestID != "" {
						initiatorID = string(ev.Initiator.RequestID)
					}
					if initiatorID == "" && ev.Initiator.URL != "" {
						initiatorID = ev.Initiator.URL
					}
					if ev.Initiator.Stack != nil && len(ev.Initiator.Stack.CallFrames) > 0 {
						frame := ev.Initiator.Stack.CallFrames[0]
						initiatorScript = frame.URL
						if initiatorScript == "" {
							initiatorScript = string(frame.ScriptID)
						}
						initiatorLineNo = frame.LineNumber
						for _, f := range ev.Initiator.Stack.CallFrames {
							if f.URL != "" && !isInternalScript(f.URL) {
								initiatorScript = f.URL
								initiatorLineNo = f.LineNumber
								break
							}
						}
					}
				}
				addRequestFact := throttler.Allow("net_request")
				requestMu.Lock()
				requestStates[ev.RequestID] = requestState{
					started: now, method: ev.Request.Method, url: ev.Request.URL,
					initType: initiatorType, factAdded: addRequestFact,
				}
				requestMu.Unlock()
				if !addRequestFact {
					return
				}

				facts := []mangle.Fact{{
					Predicate: "net_request",
					Args:      []any{sessionID, string(ev.RequestID), ev.Request.Method, ev.Request.URL, initiatorType, now.UnixMilli()},
					Timestamp: now,
				}}

				if initiatorType != "" || initiatorID != "" || initiatorScript != "" {
					parentRef := coalesceNonEmpty(initiatorID, initiatorScript)
					if initiatorLineNo > 0 && initiatorScript != "" {
						parentRef = fmt.Sprintf("%s:%d", initiatorScript, initiatorLineNo)
					}
					facts = append(facts, mangle.Fact{
						Predicate: "request_initiator",
						Args:      []any{sessionID, string(ev.RequestID), initiatorType, parentRef},
						Timestamp: now,
					})
				}

				if err := m.addStreamFacts(sessionID, facts); err != nil {
					logging.BrowserError("[session:%s] net_request fact error: %v", sessionID, err)
				}

				if captureHeaders && ev.Request != nil {
					for k, v := range ev.Request.Headers {
						if err := m.addStreamFacts(sessionID, []mangle.Fact{{
							Predicate: "net_header",
							Args:      []any{sessionID, string(ev.RequestID), "req", strings.ToLower(k), fmt.Sprintf("%v", v)},
							Timestamp: now,
						}}); err != nil {
							logging.BrowserError("[session:%s] net_header fact error: %v", sessionID, err)
						}
					}
				}
			},
			func(ev *proto.NetworkResponseReceived) {
				now := time.Now()
				var latency, duration int64
				if ev.Response != nil && ev.Response.Timing != nil {
					latency = int64(ev.Response.Timing.ReceiveHeadersEnd)
				}
				requestMu.Lock()
				state, tracked := requestStates[ev.RequestID]
				delete(requestStates, ev.RequestID)
				requestMu.Unlock()
				if tracked {
					duration = now.Sub(state.started).Milliseconds()
				}
				addResponseFact := tracked && state.factAdded
				if !addResponseFact {
					addResponseFact = ev.Response.Status >= 400 || throttler.Allow("net_response")
				}
				if !addResponseFact {
					return
				}
				facts := make([]mangle.Fact, 0, 2)
				if tracked && !state.factAdded {
					facts = append(facts, mangle.Fact{
						Predicate: "net_request",
						Args:      []any{sessionID, string(ev.RequestID), state.method, state.url, state.initType, state.started.UnixMilli()},
						Timestamp: state.started,
					})
				}
				facts = append(facts, mangle.Fact{
					Predicate: "net_response",
					Args:      []any{sessionID, string(ev.RequestID), int64(ev.Response.Status), latency, duration},
					Timestamp: now,
				})
				if err := m.addStreamFacts(sessionID, facts); err != nil {
					logging.BrowserError("[session:%s] net_response fact error: %v", sessionID, err)
				}

				if captureHeaders && ev.Response != nil {
					for k, v := range ev.Response.Headers {
						if err := m.addStreamFacts(sessionID, []mangle.Fact{{
							Predicate: "net_header",
							Args:      []any{sessionID, string(ev.RequestID), "res", strings.ToLower(k), fmt.Sprintf("%v", v)},
							Timestamp: now,
						}}); err != nil {
							logging.BrowserError("[session:%s] res net_header fact error: %v", sessionID, err)
						}
					}
				}
			},
			func(ev *proto.NetworkLoadingFailed) {
				now := time.Now()
				requestMu.Lock()
				state, tracked := requestStates[ev.RequestID]
				delete(requestStates, ev.RequestID)
				requestMu.Unlock()
				facts := make([]mangle.Fact, 0, 2)
				if tracked && !state.factAdded {
					facts = append(facts, mangle.Fact{
						Predicate: "net_request",
						Args:      []any{sessionID, string(ev.RequestID), state.method, state.url, state.initType, state.started.UnixMilli()},
						Timestamp: state.started,
					})
				}
				facts = append(facts, mangle.Fact{
					Predicate: "net_failure",
					Args:      []any{sessionID, string(ev.RequestID), ev.ErrorText, string(ev.BlockedReason), now.UnixMilli()},
					Timestamp: now,
				})
				if err := m.addStreamFacts(sessionID, facts); err != nil {
					logging.BrowserError("[session:%s] net_failure fact error: %v", sessionID, err)
				}
			},
			func(ev *proto.DOMDocumentUpdated) {
				if !captureDOM {
					return
				}
				if !throttler.Allow("dom_update") {
					return
				}
				if err := m.captureDOMFacts(ctx, sessionID, page, true); err != nil {
					logging.BrowserError("[session:%s] DOM capture error: %v", sessionID, err)
				}
			},
		)

		wg.Add(3)
		go func() {
			defer wg.Done()
			waitNav()
		}()
		go func() {
			defer wg.Done()
			waitRest()
		}()
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					res, err := page.Context(ctx).Evaluate(&rod.EvalOptions{
						JS: `
						() => {
							const buf = Array.isArray(window.__browsernerdEvents) ? window.__browsernerdEvents : [];
							window.__browsernerdEvents = [];
							return buf;
						}
						`,
						ByValue:      true,
						AwaitPromise: true,
					})
					if err != nil || res == nil {
						continue
					}
					if res.Value.Nil() {
						continue
					}
					raw, err := res.Value.MarshalJSON()
					if err != nil {
						continue
					}
					var events []struct {
						Type       string  `json:"type"`
						ID         string  `json:"id"`
						Name       string  `json:"name"`
						Value      string  `json:"value"`
						Text       string  `json:"text"`
						Level      string  `json:"level"`
						Source     string  `json:"source"`
						Descriptor string  `json:"descriptor"`
						TS         float64 `json:"ts"`
					}
					if err := json.Unmarshal(raw, &events); err != nil {
						continue
					}

					facts := make([]mangle.Fact, 0, len(events))
					for _, ev := range events {
						ts := time.UnixMilli(int64(ev.TS))
						switch ev.Type {
						case "click":
							facts = append(facts, mangle.Fact{
								Predicate: "click_event",
								Args:      []any{sessionID, ev.ID, ts.UnixMilli()},
								Timestamp: ts,
							})
						case "input":
							safeValue := m.redactor.RedactInputValue(ev.Descriptor, ev.Value)
							facts = append(facts, mangle.Fact{
								Predicate: "input_event",
								Args:      []any{sessionID, ev.ID, safeValue, ts.UnixMilli()},
								Timestamp: ts,
							})
						case "state":
							facts = append(facts, mangle.Fact{
								Predicate: "state_change",
								Args:      []any{sessionID, ev.Name, ev.Value, ts.UnixMilli()},
								Timestamp: ts,
							})
						case "dom":
							facts = append(facts, mangle.Fact{
								Predicate: "dom_updated",
								Args:      []any{sessionID, ts.UnixMilli()},
								Timestamp: ts,
							})
						case "toast":
							facts = append(facts, mangle.Fact{
								Predicate: "toast_notification",
								Args:      []any{sessionID, ev.Text, ev.Level, ev.Source, ts.UnixMilli()},
								Timestamp: ts,
							})
						}
					}
					if len(facts) > 0 {
						if err := m.addStreamFacts(sessionID, facts); err != nil {
							logging.BrowserError("[session:%s] click/state fact error: %v", sessionID, err)
						}
					}
				}
			}
		}()
		wg.Wait()
	}()
}

func stringifyConsoleArgs(args []*proto.RuntimeRemoteObject) string {
	parts := make([]string, 0, len(args))
	for _, a := range args {
		if a == nil {
			continue
		}
		if !a.Value.Nil() {
			parts = append(parts, a.Value.String())
			continue
		}
		if a.Description != "" {
			parts = append(parts, a.Description)
		}
	}
	return strings.Join(parts, " ")
}

// captureDOMFacts snapshots a limited DOM view into facts. budgeted marks the
// stream-driven calls, which repeat for the life of the tab and must respect
// the per-epoch fact budget; an explicit SnapshotDOM is caller-initiated and is
// never silently dropped.
func (m *SessionManager) captureDOMFacts(ctx context.Context, sessionID string, page *rod.Page, budgeted bool) error {
	const maxNodes = 200
	script := fmt.Sprintf(`
	() => {
		const nodes = Array.from(document.querySelectorAll('*')).slice(0, %d);
		return nodes.map((el, idx) => {
			const attrs = {};
			for (const { name, value } of Array.from(el.attributes || [])) {
				attrs[name] = value;
			}
			const rect = el.getBoundingClientRect();
			const style = window.getComputedStyle(el);
			const isVisible = style.display !== 'none' && style.visibility !== 'hidden' && style.opacity !== '0' && rect.width > 0 && rect.height > 0;

			return {
				id: el.id || ('node_' + idx),
				tag: el.tagName,
				text: (el.innerText || '').slice(0, 256),
				parent: el.parentElement && (el.parentElement.id || el.parentElement.tagName || 'root'),
				attrs,
				layout: {
					x: rect.x,
					y: rect.y,
					width: rect.width,
					height: rect.height,
					visible: isVisible
				},
				styles: {
					display: style.display || '',
					visibility: style.visibility || '',
					opacity: style.opacity || '',
					pointerEvents: style.pointerEvents || ''
				}
			};
		});
	}
	`, maxNodes)

	res, err := page.Context(ctx).Evaluate(&rod.EvalOptions{
		JS:           script,
		ByValue:      true,
		AwaitPromise: true,
	})
	if err != nil || res == nil {
		return err
	}

	raw, err := res.Value.MarshalJSON()
	if err != nil {
		return err
	}

	var nodes []domSnapshotNode
	if err := json.Unmarshal(raw, &nodes); err != nil {
		return err
	}

	facts := m.buildDOMFacts(sessionID, nodes, time.Now())
	if budgeted {
		return m.addStreamFacts(sessionID, facts)
	}
	return m.addFacts(facts)
}

// domSnapshotNode is one entry of the bounded DOM view the page script returns.
type domSnapshotNode struct {
	ID     string            `json:"id"`
	Tag    string            `json:"tag"`
	Text   string            `json:"text"`
	Parent string            `json:"parent"`
	Attrs  map[string]string `json:"attrs"`
	Layout struct {
		X       float64 `json:"x"`
		Y       float64 `json:"y"`
		Width   float64 `json:"width"`
		Height  float64 `json:"height"`
		Visible bool    `json:"visible"`
	} `json:"layout"`
	Styles map[string]string `json:"styles"`
}

// buildDOMFacts turns a decoded DOM view into the fact batch SnapshotDOM
// asserts. It is separated from the page evaluation so the schema contract
// (every predicate declared, every argument matching its declared bound type)
// can be checked without a live browser.
func (m *SessionManager) buildDOMFacts(sessionID string, nodes []domSnapshotNode, now time.Time) []mangle.Fact {
	facts := make([]mangle.Fact, 0, len(nodes)*6+1)
	for _, n := range nodes {
		n.Text = m.redactor.SanitizeString(n.Text)
		descriptor := strings.Join([]string{n.Tag, n.ID, n.Attrs["type"], n.Attrs["name"], n.Attrs["autocomplete"], n.Attrs["aria-label"]}, " ")
		n.ID = qualifyBrowserNode(sessionID, n.ID)
		n.Parent = qualifyBrowserNode(sessionID, n.Parent)
		for key, value := range n.Attrs {
			if strings.EqualFold(key, "value") {
				n.Attrs[key] = m.redactor.RedactInputValue(descriptor, value)
			} else if m.redactor.IsSensitiveKey(key) {
				n.Attrs[key] = "[REDACTED]"
			} else {
				n.Attrs[key] = m.redactor.SanitizeString(value)
			}
		}
		// 1. Assert standard DOM predicates with session-qualified identities.
		// The capture script reads el.tagName, which the DOM always reports
		// upper-cased for HTML elements. element/3 below already lower-cases it,
		// and every rule and fixture that matches a tag - target_checkbox in
		// policy/browser.mg, testdata/honeypot.edb - writes it lower-case, so an
		// upper-case dom_node tag unified with none of them.
		facts = append(facts, mangle.Fact{
			Predicate: "dom_node",
			Args:      []any{n.ID, strings.ToLower(n.Tag), n.Text, n.Parent},
			Timestamp: now,
		})
		if n.Text != "" {
			facts = append(facts, mangle.Fact{
				Predicate: "dom_text",
				Args:      []any{n.ID, n.Text},
				Timestamp: now,
			})
		}
		for k, v := range n.Attrs {
			facts = append(facts, mangle.Fact{
				Predicate: "dom_attr",
				Args:      []any{n.ID, k, v},
				Timestamp: now,
			})
			facts = append(facts, mangle.Fact{
				Predicate: "attribute",
				Args:      []any{n.ID, k, v},
				Timestamp: now,
			})
		}

		visibleAtom := "/false"
		if n.Layout.Visible {
			visibleAtom = "/true"
		}
		facts = append(facts, mangle.Fact{
			Predicate: "dom_layout",
			Args:      []any{n.ID, int64(n.Layout.X), int64(n.Layout.Y), int64(n.Layout.Width), int64(n.Layout.Height), visibleAtom},
			Timestamp: now,
		})

		// 2. Assert element, position, and geometry predicates
		facts = append(facts, mangle.Fact{
			Predicate: "element",
			Args:      []any{n.ID, strings.ToLower(n.Tag), n.Parent},
			Timestamp: now,
		})
		facts = append(facts, mangle.Fact{
			Predicate: "position",
			Args:      []any{n.ID, int64(n.Layout.X), int64(n.Layout.Y), int64(n.Layout.Width), int64(n.Layout.Height)},
			Timestamp: now,
		})
		facts = append(facts, mangle.Fact{
			Predicate: "geometry",
			Args:      []any{n.ID, int64(n.Layout.X), int64(n.Layout.Y), int64(n.Layout.Width), int64(n.Layout.Height)},
			Timestamp: now,
		})

		// 3. Assert interactable predicates
		tagLower := strings.ToLower(n.Tag)
		isInteractable := false
		var elemType string
		if tagLower == "button" || tagLower == "a" {
			isInteractable = true
			elemType = "/click"
		} else if tagLower == "input" {
			isInteractable = true
			inputType := strings.ToLower(n.Attrs["type"])
			if inputType == "checkbox" {
				elemType = "/checkbox"
			} else if inputType == "radio" {
				elemType = "/radio"
			} else if inputType == "submit" || inputType == "button" {
				elemType = "/click"
			} else {
				elemType = "/input"
			}
		} else if tagLower == "textarea" || tagLower == "select" {
			isInteractable = true
			elemType = "/input"
		}
		if isInteractable {
			facts = append(facts, mangle.Fact{
				Predicate: "interactable",
				Args:      []any{n.ID, elemType},
				Timestamp: now,
			})
		}

		// 4. Assert css_property and computed_style predicates for computed styles
		for k, v := range n.Styles {
			if v != "" {
				facts = append(facts, mangle.Fact{
					Predicate: "computed_style",
					Args:      []any{n.ID, k, v},
					Timestamp: now,
				})
				facts = append(facts, mangle.Fact{
					Predicate: "css_property",
					Args:      []any{n.ID, k, v},
					Timestamp: now,
				})
			}
		}
	}
	facts = append(facts, mangle.Fact{
		Predicate: "dom_updated",
		Args:      []any{sessionID, now.UnixMilli()},
		Timestamp: now,
	})
	return facts
}

func qualifyBrowserNode(sessionID, nodeID string) string {
	return sessionID + ":" + nodeID
}

// SnapshotDOM triggers a one-off DOM capture for the given session.
func (m *SessionManager) SnapshotDOM(ctx context.Context, sessionID string) error {
	if err := m.ensureStarted(ctx); err != nil {
		return err
	}
	page, ok := m.Page(sessionID)
	if !ok {
		return fmt.Errorf("unknown session: %s", sessionID)
	}
	return m.captureDOMFacts(ctx, sessionID, page, false)
}

func snapshotStorage(page *rod.Page, store string) string {
	jsFunc := fmt.Sprintf(`() => {
		try {
			const out = {};
			for (const key of Object.keys(%s)) {
				out[key] = %s.getItem(key);
			}
			return JSON.stringify(out);
		} catch (e) {
			return "{}";
		}
	}`, store, store)

	res, err := page.Evaluate(&rod.EvalOptions{
		JS:           jsFunc,
		ByValue:      true,
		AwaitPromise: true,
	})
	if err != nil || res == nil || res.Value.Nil() {
		return "{}"
	}
	return res.Value.String()
}

func restoreStorage(page *rod.Page, localJSON, sessionJSON string) {
	_, _ = page.Evaluate(&rod.EvalOptions{
		JS: `
		(local, session) => {
			try {
				const l = JSON.parse(local || "{}");
				Object.entries(l).forEach(([k, v]) => localStorage.setItem(k, v));
			} catch (e) {}
			try {
				const s = JSON.parse(session || "{}");
				Object.entries(s).forEach(([k, v]) => sessionStorage.setItem(k, v));
			} catch (e) {}
		}
		`,
		JSArgs:       []any{localJSON, sessionJSON},
		ByValue:      true,
		AwaitPromise: true,
		UserGesture:  true,
	})
}

// persistSessions writes session metadata to disk.
func (m *SessionManager) persistSessions() error {
	if m.cfg.SessionStore == "" {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]Session, 0, len(m.sessions))
	for _, rec := range m.sessions {
		sessions = append(sessions, rec.meta)
	}

	data, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		return err
	}

	if err := browsersecurity.EnsurePrivateDir(filepath.Dir(m.cfg.SessionStore)); err != nil {
		return err
	}
	return browsersecurity.WritePrivateFile(m.cfg.SessionStore, data)
}

// loadSessionsLocked loads persisted metadata. Caller must hold lock.
func (m *SessionManager) loadSessionsLocked() error {
	if m.cfg.SessionStore == "" {
		return nil
	}

	data, err := os.ReadFile(m.cfg.SessionStore)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var sessions []Session
	if err := json.Unmarshal(data, &sessions); err != nil {
		return err
	}

	for _, s := range sessions {
		s.Status = "detached"
		m.sessions[s.ID] = &sessionRecord{meta: s, page: nil, registry: NewElementRegistry()}
	}
	return nil
}

func coalesceNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func isInternalScript(url string) bool {
	internalPrefixes := []string{
		"chrome://",
		"chrome-extension://",
		"devtools://",
		"about:",
		"data:",
		"blob:",
	}
	for _, prefix := range internalPrefixes {
		if strings.HasPrefix(url, prefix) {
			return true
		}
	}
	return false
}
