package specs

import (
	"sort"
)

// A link is confirmed when someone (or the first scan that saw it) has looked
// at the section and the element together and found them in agreement. The
// confirmation records what each side said at that moment: its fingerprint.
// Drift is then a question about content, asked per link: the element no
// longer says what it said when the pair was confirmed, while the section
// still says what it said (the code moved, the spec did not follow). The
// reverse is a spec that moved while the code stayed. Both sides moving is the
// spec following the code, or the code following the spec: the pair is
// confirmed again at what they now say.
//
// Nothing here reads a clock or a file: the caller passes the scan time and
// keeps the confirmations (store.go).

// LinkKey identifies one link: a section and an element.
type LinkKey struct {
	Spec, Code string
}

// Confirmation is what a link's two sides said when it was last confirmed.
type Confirmation struct {
	SpecFingerprint string
	CodeFingerprint string
	// ConfirmedAt is the unix time of the scan, or the review, that confirmed it.
	ConfirmedAt int64
	// SpecChangedAt and CodeChangedAt are the unix time of the first scan that
	// saw that side differ from the confirmed one; zero while it matches.
	SpecChangedAt int64
	CodeChangedAt int64
}

// Confirmations is every link's confirmation.
type Confirmations map[LinkKey]Confirmation

// Clone returns a copy that can be changed without touching c.
func (c Confirmations) Clone() Confirmations {
	out := make(Confirmations, len(c))
	for k, v := range c {
		out[k] = v
	}
	return out
}

// Equal reports whether two sets hold the same confirmations.
func (c Confirmations) Equal(o Confirmations) bool {
	if len(c) != len(o) {
		return false
	}
	for k, v := range c {
		if w, ok := o[k]; !ok || v != w {
			return false
		}
	}
	return true
}

// Keys returns the links in a stable order.
func (c Confirmations) Keys() []LinkKey {
	keys := make([]LinkKey, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Spec != keys[j].Spec {
			return keys[i].Spec < keys[j].Spec
		}
		return keys[i].Code < keys[j].Code
	})
	return keys
}

// Reconcile advances prev to what one measurement saw and returns the result.
// links are the links of the measurement; sections and elements map a ref to
// its current fingerprint (only refs the world holds are in them). now is the
// scan time in unix seconds.
//
//   - A link with no confirmation is confirmed at what it says now: a fresh
//     checkout, or a link written today, has no drift to report.
//   - Both sides differ from the confirmed pair: the pair is confirmed again.
//   - One side differs: the confirmation stays, and the first scan to see that
//     side differ is remembered, so the report can say since when.
//   - A side back at its confirmed content is no longer changed.
//   - A link that is no longer a link loses its confirmation. A link with an
//     end that is gone keeps it: if the end comes back as it was, nothing drifted.
func Reconcile(prev Confirmations, links []Link, sections, elements map[string]string, now int64) Confirmations {
	out := make(Confirmations, len(links))
	for _, l := range links {
		key := LinkKey{Spec: l.SpecRef, Code: l.CodeRef}
		old, had := prev[key]
		sf, sok := sections[l.SpecRef]
		cf, cok := elements[l.CodeRef]
		if !sok || !cok || sf == "" || cf == "" {
			if had {
				out[key] = old
			}
			continue
		}
		if !had {
			out[key] = Confirmation{SpecFingerprint: sf, CodeFingerprint: cf, ConfirmedAt: now}
			continue
		}
		specMoved, codeMoved := old.SpecFingerprint != sf, old.CodeFingerprint != cf
		switch {
		case specMoved && codeMoved:
			out[key] = Confirmation{SpecFingerprint: sf, CodeFingerprint: cf, ConfirmedAt: now}
		default:
			c := old
			c.SpecChangedAt = firstSeen(specMoved, old.SpecChangedAt, now)
			c.CodeChangedAt = firstSeen(codeMoved, old.CodeChangedAt, now)
			out[key] = c
		}
	}
	return out
}

func firstSeen(moved bool, before, now int64) int64 {
	switch {
	case !moved:
		return 0
	case before != 0:
		return before
	}
	return now
}

// Confirm records that the link was reviewed and is in agreement as the two
// sides now say, whatever the scan last saw.
func Confirm(c Confirmations, spec, code, specFingerprint, codeFingerprint string, now int64) Confirmations {
	out := c.Clone()
	out[LinkKey{Spec: spec, Code: code}] = Confirmation{SpecFingerprint: specFingerprint, CodeFingerprint: codeFingerprint, ConfirmedAt: now}
	return out
}
