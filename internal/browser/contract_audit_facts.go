package browser

import (
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/mangle"
)

// maxAuditFacts caps the number of facts AuditDiscoveryFacts may return.
// A discovery is already bounded, but the kernel must not be floodable through
// this conversion path, so the output is truncated cleanly at the cap.
const maxAuditFacts = 500

type auditLineIndex struct {
	needlePath map[string]int64
	byPath     map[string]int64
}

func buildAuditMatchIndexes(matches []RepoMatch) auditLineIndex {
	needlePath := make(map[string]int64, len(matches))
	byPath := make(map[string]int64, len(matches))
	for _, m := range matches {
		key := m.Needle + "\x00" + m.Path
		if _, ok := needlePath[key]; !ok {
			needlePath[key] = int64(m.Line)
		}
		if _, ok := byPath[m.Path]; !ok {
			byPath[m.Path] = int64(m.Line)
		}
	}
	return auditLineIndex{needlePath: needlePath, byPath: byPath}
}

func auditRelativePath(p string) string {
	clean := filepath.ToSlash(p)
	if !filepath.IsAbs(clean) {
		return clean
	}
	clean = strings.TrimPrefix(clean, "/")
	if len(clean) >= 2 && clean[1] == ':' {
		clean = strings.TrimPrefix(clean[2:], "/")
		clean = strings.TrimPrefix(clean, "\\")
		clean = filepath.ToSlash(clean)
	}
	clean = strings.TrimLeft(clean, "/")
	if clean == "" {
		clean = filepath.Base(filepath.ToSlash(p))
		clean = filepath.ToSlash(clean)
	}
	if filepath.IsAbs(clean) {
		clean = filepath.Base(clean)
	}
	return clean
}

func auditSourceLine(subject, src, clean string, idx auditLineIndex) int64 {
	if v, ok := idx.needlePath[subject+"\x00"+src]; ok {
		return v
	}
	if v, ok := idx.byPath[src]; ok {
		return v
	}
	if v, ok := idx.byPath[clean]; ok {
		return v
	}
	return 0
}

func appendAuditFindings(dst []mangle.Fact, sessionID string, findings []AuditFinding, nowMs int64, ts time.Time) []mangle.Fact {
	for _, f := range findings {
		if len(dst) >= maxAuditFacts {
			break
		}
		if strings.TrimSpace(f.Subject) == "" {
			continue
		}
		dst = append(dst, mangle.Fact{
			Predicate: "audit_finding",
			Args:      []any{sessionID, string(f.Kind), f.Subject, f.Detail, nowMs},
			Timestamp: ts,
		})
	}
	return dst
}

func appendAuditNeedles(dst []mangle.Fact, sessionID string, needles []string, ts time.Time) []mangle.Fact {
	for _, n := range needles {
		if len(dst) >= maxAuditFacts {
			break
		}
		if strings.TrimSpace(n) == "" {
			continue
		}
		dst = append(dst, mangle.Fact{
			Predicate: "audit_needle",
			Args:      []any{sessionID, n},
			Timestamp: ts,
		})
	}
	return dst
}

func appendAuditSources(dst []mangle.Fact, sessionID string, disc ContractAuditDiscovery, idx auditLineIndex, ts time.Time) []mangle.Fact {
	for _, f := range disc.Findings {
		if strings.TrimSpace(f.Subject) == "" {
			continue
		}
		for _, src := range f.Sources {
			if len(dst) >= maxAuditFacts {
				return dst
			}
			if strings.TrimSpace(src) == "" {
				continue
			}
			clean := auditRelativePath(src)
			line := auditSourceLine(f.Subject, src, clean, idx)
			dst = append(dst, mangle.Fact{
				Predicate: "audit_source",
				Args:      []any{sessionID, f.Subject, clean, line},
				Timestamp: ts,
			})
		}
	}
	return dst
}

// AuditDiscoveryFacts converts a discovery into session-scoped kernel facts.
// It is a pure conversion so it can be tested without a kernel.
func AuditDiscoveryFacts(sessionID string, disc ContractAuditDiscovery, nowMs int64) []mangle.Fact {
	idx := buildAuditMatchIndexes(disc.Matches)
	ts := time.UnixMilli(nowMs)
	facts := make([]mangle.Fact, 0, 32)
	facts = appendAuditFindings(facts, sessionID, disc.Findings, nowMs, ts)
	if len(facts) >= maxAuditFacts {
		return facts[:maxAuditFacts]
	}
	facts = appendAuditNeedles(facts, sessionID, disc.Needles, ts)
	if len(facts) >= maxAuditFacts {
		return facts[:maxAuditFacts]
	}
	facts = appendAuditSources(facts, sessionID, disc, idx, ts)
	if len(facts) > maxAuditFacts {
		facts = facts[:maxAuditFacts]
	}
	return facts
}

// AssertAuditDiscovery asserts a discovery into the session kernel.
// It routes through addFacts so audit facts are redacted and flight-recorded
// on exactly the same path as every other browser fact.
func (m *SessionManager) AssertAuditDiscovery(sessionID string, disc ContractAuditDiscovery) error {
	if m == nil {
		return nil
	}
	if strings.TrimSpace(sessionID) == "" {
		return nil
	}
	nowMs := time.Now().UnixMilli()
	facts := AuditDiscoveryFacts(sessionID, disc, nowMs)
	if len(facts) == 0 {
		return nil
	}
	return m.addFacts(facts)
}
