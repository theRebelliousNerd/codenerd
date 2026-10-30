package perception

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"
)

// Restriction is the limit a user put on the current request, carried from
// understanding to the action boundary (Docs/audits/perception-restriction-
// continuity.md). It is an immutable value: the kinds are fixed at
// construction, a copy shares nothing mutable, and the only way to widen one
// is Merge, which can only add kinds. It is an observation about the
// request, never a grant: the kernel derives what a kind forbids
// (policy/constitution.mg, section 7D) and the existing permission and approval
// paths still decide everything else.
//
// A zero Restriction is "none recognised", which includes a producer that
// never filled the field. The user's own words stay in Text for explanation;
// constraints outside the supported vocabulary are listed in Unenforced so a
// phrase the system could not turn into an obligation is never reported as
// one.
type Restriction struct {
	kinds      []RestrictionKind
	text       string
	unenforced []string
}

// RestrictionKind is one obligation the system can enforce.
type RestrictionKind string

const (
	// RestrictReadOnly, RestrictNoChanges and RestrictDryRun say the same to
	// the kernel: nothing in the workspace may change and nothing may run.
	// They stay distinct so the receipt names what the user said.
	RestrictReadOnly  RestrictionKind = "read_only"
	RestrictNoChanges RestrictionKind = "no_changes"
	RestrictDryRun    RestrictionKind = "dry_run"
	// RestrictHypothetical is a what-if: consult, act on nothing.
	RestrictHypothetical RestrictionKind = "hypothetical"
	// RestrictConfirmFirst owes the user's own decision before a consequential
	// effect. Standing autonomy (yolo) does not discharge it.
	RestrictConfirmFirst RestrictionKind = "confirm_first"
)

// restrictionAliases maps a normalised constraint spelling to its kind. The
// understanding prompt asks for these tokens; the aliases are the spellings a
// model emits for them. Anything else is prose, kept in Text and Unenforced.
var restrictionAliases = map[string]RestrictionKind{
	"read_only":     RestrictReadOnly,
	"readonly":      RestrictReadOnly,
	"no_changes":    RestrictNoChanges,
	"no_change":     RestrictNoChanges,
	"dry_run":       RestrictDryRun,
	"dryrun":        RestrictDryRun,
	"hypothetical":  RestrictHypothetical,
	"confirm_first": RestrictConfirmFirst,
	"confirm":       RestrictConfirmFirst,
}

func normalizeConstraintToken(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.NewReplacer(" ", "_", "-", "_").Replace(s)
}

// RestrictionFrom reads the restriction out of an understanding: the
// hypothetical and confirmation signals, and the user constraints that name a
// supported kind. Nil is none.
func RestrictionFrom(u *Understanding) Restriction {
	if u == nil {
		return Restriction{}
	}
	var kinds []RestrictionKind
	var unenforced []string
	if u.Signals.IsHypothetical {
		kinds = append(kinds, RestrictHypothetical)
	}
	if u.Signals.RequiresConfirmation {
		kinds = append(kinds, RestrictConfirmFirst)
	}
	for _, c := range u.UserConstraints {
		if strings.TrimSpace(c) == "" {
			continue
		}
		if kind, ok := restrictionAliases[normalizeConstraintToken(c)]; ok {
			kinds = append(kinds, kind)
		} else {
			unenforced = append(unenforced, strings.TrimSpace(c))
		}
	}
	return newRestriction(kinds, strings.Join(u.UserConstraints, "; "), unenforced)
}

func newRestriction(kinds []RestrictionKind, text string, unenforced []string) Restriction {
	kinds = slices.Clone(kinds)
	slices.Sort(kinds)
	kinds = slices.Compact(kinds)
	if len(kinds) == 0 && len(unenforced) == 0 {
		return Restriction{}
	}
	var seen []string
	for _, u := range unenforced {
		if !slices.Contains(seen, u) {
			seen = append(seen, u)
		}
	}
	return Restriction{kinds: kinds, text: text, unenforced: seen}
}

// RestrictionOf is a restriction of the given kinds from another reader of
// the turn than perception (Jev's judgments, policy/jev_evidence.mg). It only
// ever adds to the turn's restriction through Merge.
func RestrictionOf(text string, kinds ...RestrictionKind) Restriction {
	return newRestriction(kinds, text, nil)
}

// Kinds returns the enforceable kinds, sorted. The slice is a copy.
func (r Restriction) Kinds() []RestrictionKind { return slices.Clone(r.kinds) }

// Has reports whether the restriction carries the kind.
func (r Restriction) Has(k RestrictionKind) bool { return slices.Contains(r.kinds, k) }

// Enforced reports whether any obligation is enforceable. A restriction that
// holds only unrecognised prose is not enforced and says so.
func (r Restriction) Enforced() bool { return len(r.kinds) > 0 }

// IsZero reports that nothing was recognised.
func (r Restriction) IsZero() bool { return len(r.kinds) == 0 && len(r.unenforced) == 0 }

// Text is the user's constraint words as perception returned them.
func (r Restriction) Text() string { return r.text }

// Unenforced lists the constraint phrases outside the supported vocabulary.
// They reach the model as prose and are enforced by nothing.
func (r Restriction) Unenforced() []string { return slices.Clone(r.unenforced) }

// Merge is the restriction of work that must honour both: the union of the
// kinds. A child may add to its parent's restriction and can never drop it.
func (r Restriction) Merge(o Restriction) Restriction {
	if o.IsZero() {
		return r
	}
	if r.IsZero() {
		return o
	}
	text := r.text
	if o.text != "" && o.text != r.text {
		if text != "" {
			text += "; "
		}
		text += o.text
	}
	return newRestriction(append(slices.Clone(r.kinds), o.kinds...), text, append(slices.Clone(r.unenforced), o.unenforced...))
}

// Digest identifies the restriction by its enforceable content, so an
// approval or receipt can bind to it: a changed restriction is another digest.
func (r Restriction) Digest() string {
	parts := make([]string, len(r.kinds))
	for i, k := range r.kinds {
		parts[i] = string(k)
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, ",")))
	return "sha256:" + hex.EncodeToString(sum[:])
}

type restrictionCtxKey struct{}

// WithRestriction returns ctx carrying r merged with any restriction ctx
// already carries: a derived context can narrow further but never lift what
// its parent was held to. Every delegation, preset task and subagent runs
// under the context of the turn that spawned it, so this is how a task
// inherits its parent's limits without a second model call to re-derive them.
func WithRestriction(ctx context.Context, r Restriction) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if r.IsZero() {
		return ctx
	}
	return context.WithValue(ctx, restrictionCtxKey{}, RestrictionFromContext(ctx).Merge(r))
}

// RestrictionFromContext is the restriction the context's turn is held to;
// zero when none.
func RestrictionFromContext(ctx context.Context) Restriction {
	if ctx == nil {
		return Restriction{}
	}
	r, _ := ctx.Value(restrictionCtxKey{}).(Restriction)
	return r
}

// MarshalJSON preserves the original request's obligations in durable
// continuations. The private fields otherwise encode as an empty object.
func (r Restriction) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kinds      []RestrictionKind `json:"kinds"`
		Text       string            `json:"text"`
		Unenforced []string          `json:"unenforced,omitempty"`
	}{r.Kinds(), r.Text(), r.Unenforced()})
}

func (r *Restriction) UnmarshalJSON(data []byte) error {
	var wire struct {
		Kinds      []RestrictionKind `json:"kinds"`
		Text       string            `json:"text"`
		Unenforced []string          `json:"unenforced,omitempty"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*r = newRestriction(wire.Kinds, wire.Text, wire.Unenforced)
	return nil
}
