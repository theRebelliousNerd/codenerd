package autopoiesis

import (
	"fmt"
	"sort"
	"strings"

	"codenerd/internal/config"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// CatalogTool is one built-in tool the process can offer: the universe the
// ledger is read against, so a tool nobody ever offered or called is a row
// with zero uses instead of being absent from the report.
type CatalogTool struct {
	Name       string
	Capability tools.Capability
}

// CatalogOf lists the tools a registry holds with the capability each
// declares. A tool with no declaration lists with an empty capability, which
// the policy reads as not unique.
func CatalogOf(r *tools.Registry) []CatalogTool {
	all := r.All()
	out := make([]CatalogTool, 0, len(all))
	for _, t := range all {
		c, _ := t.DeclaredCapability()
		out = append(out, CatalogTool{Name: t.Name, Capability: c})
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Name < out[b].Name })
	return out
}

// ToolLedgerSnapshot is everything the tool-value policy is asked about: the
// measured ledgers, the built-in catalog, and the generated tools currently
// registered.
type ToolLedgerSnapshot struct {
	Ledgers   map[string]ToolLedger
	Catalog   []CatalogTool
	Generated []string
}

// ledgerPredicates are the measurement predicates ToolLedgerFacts owns. A
// refresh retracts all of them and asserts the new set, so a tool that
// stopped reporting a scan stops having a condensation fact.
var ledgerPredicates = []string{
	"tool_origin", "tool_capability_declared", "tool_use_count",
	"tool_offer_count", "tool_cost", "tool_condensation", "tool_channel_use",
	jevTallyPred, jevVerdictPred,
}

func nameAtom(v string) types.MangleAtom { return types.MangleAtom("/" + v) }

// ToolLedgerFacts renders the snapshot as the EDB facts policy/tool_value.mg
// reads, Jev's persisted readings (tool_ledger_jev.go) among them. Every number
// is a ledger sum; nothing is estimated here. A tool whose
// calls never reported a scan has no tool_condensation fact. A retired tool
// has no facts at all.
func ToolLedgerFacts(s ToolLedgerSnapshot) []types.Fact {
	origin := map[string]types.ToolOrigin{}
	capability := map[string]tools.Capability{}
	for _, c := range s.Catalog {
		origin[c.Name] = types.ToolOriginBuiltin
		capability[c.Name] = c.Capability
	}
	for _, g := range s.Generated {
		origin[g] = types.ToolOriginGenerated
	}
	for tool, l := range s.Ledgers {
		if l.Origin != "" && origin[tool] == "" {
			origin[tool] = l.Origin
		}
	}
	names := make([]string, 0, len(origin))
	for tool := range origin {
		names = append(names, tool)
	}
	sort.Strings(names)

	var facts []types.Fact
	for _, tool := range names {
		l := s.Ledgers[tool]
		if !l.RetiredAt.IsZero() {
			continue
		}
		facts = append(facts,
			types.Fact{Predicate: "tool_origin", Args: []any{tool, nameAtom(string(origin[tool]))}},
			types.Fact{Predicate: "tool_use_count", Args: []any{tool, l.Calls, l.CallSessions}},
			types.Fact{Predicate: "tool_offer_count", Args: []any{tool, l.Offers, l.OfferSessions}},
			types.Fact{Predicate: "tool_cost", Args: []any{tool,
				l.CatalogTokens + l.AtomTokens, l.CallTokens, l.ResultTokens, l.VerifyTokens}},
		)
		if c := capability[tool]; c != "" {
			facts = append(facts, types.Fact{Predicate: "tool_capability_declared", Args: []any{tool, nameAtom(string(c))}})
		}
		if l.KnownCalls > 0 {
			facts = append(facts, types.Fact{Predicate: "tool_condensation", Args: []any{tool,
				l.KnownCalls, l.ConsumedBytes, l.ReturnedBytes, l.KnownCallTokens}})
		}
		facts = append(facts, jevFacts(tool, l.Jev)...)
		channels := make([]string, 0, len(l.Channels))
		for ch := range l.Channels {
			channels = append(channels, ch)
		}
		sort.Strings(channels)
		for _, ch := range channels {
			facts = append(facts, types.Fact{Predicate: "tool_channel_use", Args: []any{tool, nameAtom(ch), l.Channels[ch]}})
		}
	}
	return facts
}

// AssertToolLedger replaces the kernel's measurement facts and thresholds with
// the snapshot's and the section's. It refuses to run while a threshold the
// policy requires is missing: a rule over an absent bar derives nothing, and a
// silent empty answer would read as "no tool is dead".
func AssertToolLedger(k types.Kernel, s ToolLedgerSnapshot, cfg config.ToolValueConfig) error {
	if err := config.EnsureParams(k, cfg.Params()); err != nil {
		return fmt.Errorf("tool value thresholds: %w", err)
	}
	if missing, err := k.Query("config_param_missing"); err == nil {
		var keys []string
		for _, f := range missing {
			if len(f.Args) == 2 && types.ExtractString(f.Args[0]) == "/tool_value" {
				keys = append(keys, types.ExtractString(f.Args[1]))
			}
		}
		if len(keys) > 0 {
			return fmt.Errorf("tool value: the policy requires thresholds the section did not supply: %s", strings.Join(keys, ", "))
		}
	}
	for _, pred := range ledgerPredicates {
		if err := k.Retract(pred); err != nil {
			return fmt.Errorf("tool value: clear %s: %w", pred, err)
		}
	}
	if facts := ToolLedgerFacts(s); len(facts) > 0 {
		if err := k.AssertBatch(facts); err != nil {
			return fmt.Errorf("tool value: assert measurements: %w", err)
		}
	}
	return nil
}
