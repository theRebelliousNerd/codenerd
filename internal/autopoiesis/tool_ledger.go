package autopoiesis

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"codenerd/internal/atomicfile"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// =============================================================================
// TOOL LEDGER - WHAT EACH TOOL COST AND WHAT IT DID
// =============================================================================
// The ledger is a section of the tool's profile (ToolQualityProfile.Value),
// kept in the same quality_profiles.json the profile store already persists:
// a profile says what a generated tool is expected to do, the ledger says what
// every tool, built-in or generated, actually cost and returned. There is one
// store. A tool with a ledger and no expectations is a ledger-only entry
// (ToolType empty); GetProfile does not return it, so the quality evaluator
// never mistakes a built-in's ledger for a generated tool's expectations.
//
// The ledger is measurement only. Which tools earn their place is decided by
// policy/tool_value.mg over the facts ToolLedgerFacts replays from it.

// ToolLedger is one tool's measured record, summed over every session the
// store has seen. Token fields are tokens, byte fields are bytes; every field
// is a sum, so two stores' ledgers merge by addition.
type ToolLedger struct {
	Origin types.ToolOrigin `json:"origin,omitempty"`

	// Prompt cost: paid on every compile that offered the tool.
	Offers        int64 `json:"offers,omitempty"`
	OfferSessions int64 `json:"offer_sessions,omitempty"`
	CatalogTokens int64 `json:"catalog_tokens,omitempty"`
	AtomTokens    int64 `json:"atom_tokens,omitempty"`

	// Use.
	Calls        int64 `json:"calls,omitempty"`
	Successes    int64 `json:"successes,omitempty"`
	CallSessions int64 `json:"call_sessions,omitempty"`
	CallTokens   int64 `json:"call_tokens,omitempty"`
	ResultTokens int64 `json:"result_tokens,omitempty"`
	ResultBytes  int64 `json:"result_bytes,omitempty"`
	DurationMs   int64 `json:"duration_ms,omitempty"`

	// Condensation, over only the calls whose tool reported what it scanned.
	// UnknownCalls is the calls that did not: counted, never guessed.
	KnownCalls      int64 `json:"known_calls,omitempty"`
	ConsumedBytes   int64 `json:"consumed_bytes,omitempty"`
	ConsumedItems   int64 `json:"consumed_items,omitempty"`
	ReturnedBytes   int64 `json:"returned_bytes,omitempty"`
	KnownCallTokens int64 `json:"known_call_tokens,omitempty"`
	UnknownCalls    int64 `json:"unknown_calls,omitempty"`

	// Verification calls attributed to this tool (a write that needed a build
	// or a test run to be trusted).
	VerifyCalls  int64 `json:"verify_calls,omitempty"`
	VerifyTokens int64 `json:"verify_tokens,omitempty"`

	Channels map[string]int64 `json:"channels,omitempty"`

	FirstSeen time.Time `json:"first_seen,omitempty"`
	LastOffer time.Time `json:"last_offer,omitempty"`
	LastCall  time.Time `json:"last_call,omitempty"`
	// RetiredAt is set when a generated tool was withdrawn through the
	// approval path. A retired tool is out of the policy's facts until it is
	// offered again, which starts its record over.
	RetiredAt time.Time `json:"retired_at,omitempty"`
}

// nonneg clamps a measurement at zero: a negative count is a bug in the
// reporter, and summing it would hide the bug inside a total.
func nonneg(v int64) int64 {
	if v < 0 {
		return 0
	}
	return v
}

func (l *ToolLedger) channel(c types.ToolChannel, n int64) {
	if c == "" {
		return
	}
	if l.Channels == nil {
		l.Channels = map[string]int64{}
	}
	l.Channels[string(c)] += n
}

// ledgerWrite is one mutation of one tool's ledger, applied under the store
// lock on top of the freshest on-disk state.
type ledgerWrite struct {
	tool string
	fn   func(l *ToolLedger)
}

// mutateLedgers reloads the ledgers other processes may have written, applies
// the writes, and persists once. A ledger that cannot persist logs and drops
// the write: a measurement is never a reason to fail a tool call.
func (ps *ProfileStore) mutateLedgers(writes ...ledgerWrite) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.adoptDiskLedgersLocked()
	now := time.Now()
	for _, w := range writes {
		p := ps.profiles[w.tool]
		if p == nil {
			p = &ToolQualityProfile{ToolName: w.tool}
			ps.profiles[w.tool] = p
		}
		if p.Value == nil {
			p.Value = &ToolLedger{FirstSeen: now}
		}
		w.fn(p.Value)
	}
	ps.save()
}

// adoptDiskLedgersLocked folds in what another process wrote since this one
// last did: the file is the authority for ledgers, this process's memory for
// everything else. A profile only the file holds is adopted whole.
func (ps *ProfileStore) adoptDiskLedgersLocked() {
	path := filepath.Join(ps.storePath, profilesFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var disk map[string]*ToolQualityProfile
	if err := json.Unmarshal(data, &disk); err != nil {
		logging.AutopoiesisError("ProfileStore: corrupt %s: %v", path, err)
		return
	}
	for name, d := range disk {
		if d == nil {
			continue
		}
		if mem := ps.profiles[name]; mem != nil {
			// A file with no ledger for the tool (this process's write has
			// not landed, or failed) does not erase the one held in memory.
			if d.Value != nil {
				mem.Value = d.Value
			}
			continue
		}
		ps.profiles[name] = d
	}
}

// profilesFile is the store's one file.
const profilesFile = "quality_profiles.json"

// writeProfilesAtomically replaces the store file so a crash or a second
// process never sees half of it.
func writeProfilesAtomically(path string, data []byte) error {
	return atomicfile.WriteFile(path, data, 0o644)
}

// NoteToolOffers records one compile's offer of each tool (types.ToolMeter).
func (ps *ProfileStore) NoteToolOffers(offers []types.ToolOffer) {
	writes := make([]ledgerWrite, 0, len(offers))
	for _, o := range offers {
		if o.Tool == "" {
			continue
		}
		writes = append(writes, ledgerWrite{tool: o.Tool, fn: func(l *ToolLedger) {
			if !l.RetiredAt.IsZero() {
				*l = ToolLedger{FirstSeen: time.Now()}
			}
			if o.Origin != "" {
				l.Origin = o.Origin
			}
			l.Offers++
			if ps.firstInSession("offer", o.Tool, o.SessionID) {
				l.OfferSessions++
			}
			l.CatalogTokens += nonneg(int64(o.CatalogTokens))
			l.AtomTokens += nonneg(int64(o.AtomTokens))
			l.LastOffer = time.Now()
		}})
	}
	if len(writes) > 0 {
		ps.mutateLedgers(writes...)
	}
}

// NoteToolCall records one call on any channel (types.ToolMeter).
func (ps *ProfileStore) NoteToolCall(c types.ToolCallMeasure) {
	if c.Tool == "" {
		return
	}
	ps.mutateLedgers(ledgerWrite{tool: c.Tool, fn: func(l *ToolLedger) {
		if !l.RetiredAt.IsZero() {
			*l = ToolLedger{FirstSeen: time.Now()}
		}
		if c.Origin != "" {
			l.Origin = c.Origin
		}
		l.Calls++
		if c.Success {
			l.Successes++
		}
		if ps.firstInSession("call", c.Tool, c.SessionID) {
			l.CallSessions++
		}
		l.CallTokens += nonneg(int64(c.CallTokens))
		l.ResultTokens += nonneg(int64(c.ResultTokens))
		l.ResultBytes += nonneg(c.ResultBytes)
		l.DurationMs += c.Duration.Milliseconds()
		if c.Consumed.Known {
			l.KnownCalls++
			l.ConsumedBytes += nonneg(c.Consumed.Bytes)
			l.ConsumedItems += nonneg(c.Consumed.Items)
			l.ReturnedBytes += nonneg(c.ResultBytes)
			l.KnownCallTokens += nonneg(int64(c.CallTokens)) + nonneg(int64(c.ResultTokens))
		} else {
			l.UnknownCalls++
		}
		l.channel(c.Channel, 1)
		l.LastCall = time.Now()
	}})
}

// NoteToolVerification charges a verification call to the write that made it
// necessary (types.ToolMeter).
func (ps *ProfileStore) NoteToolVerification(v types.ToolVerification) {
	if v.Tool == "" {
		return
	}
	ps.mutateLedgers(ledgerWrite{tool: v.Tool, fn: func(l *ToolLedger) {
		l.VerifyCalls++
		l.VerifyTokens += nonneg(int64(v.Tokens))
	}})
}

// firstInSession reports whether (kind, tool, session) is new to this store,
// so a session counts once however many compiles or calls it made. An empty
// session id is never new: a measurement with no session cannot be told apart
// from another, and counting each as its own session would inflate the very
// number "never called over N sessions" is judged on.
func (ps *ProfileStore) firstInSession(kind, tool, session string) bool {
	if session == "" {
		return false
	}
	key := kind + "\x00" + tool + "\x00" + session
	if _, seen := ps.sessionSeen[key]; seen {
		return false
	}
	if ps.sessionSeen == nil {
		ps.sessionSeen = map[string]struct{}{}
	}
	ps.sessionSeen[key] = struct{}{}
	return true
}

// ToolLedgers returns a copy of every tool's ledger as the file holds it now.
func (ps *ProfileStore) ToolLedgers() map[string]ToolLedger {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.adoptDiskLedgersLocked()
	out := make(map[string]ToolLedger, len(ps.profiles))
	for name, p := range ps.profiles {
		if p == nil || p.Value == nil {
			continue
		}
		l := *p.Value
		l.Channels = copyChannels(p.Value.Channels)
		out[name] = l
	}
	return out
}

func copyChannels(in map[string]int64) map[string]int64 {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]int64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// MarkToolRetired records that a generated tool was withdrawn.
func (ps *ProfileStore) MarkToolRetired(tool string) {
	ps.mutateLedgers(ledgerWrite{tool: tool, fn: func(l *ToolLedger) { l.RetiredAt = time.Now() }})
}

var _ types.ToolMeter = (*ProfileStore)(nil)
