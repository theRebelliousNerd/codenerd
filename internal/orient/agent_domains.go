package orient

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"codenerd/internal/types"
)

// AgentDomains derives the catalog domain of every stack agent from the
// workspace's saved orientation snapshot: orient_agent_domain
// (ecosystem_agents.mg) over the profile signals init measured. The result
// maps each agent name, as the catalog spells it ("GoExpert"), to its
// technology keys ("go"), sorted.
//
// Boot asks this for a shard agent whose agents.json entry predates the
// `domains` field, so an existing workspace learns its agents' domains without
// re-running init. Only profile_signal rows are asserted: orient_agent_domain
// joins nothing else measured, so evaluating the whole snapshot would derive
// the same rows at the cost of the history and document census. Mangle makes
// the decision; Go selects the measurements it reads (as evaluateFreshness
// does). A workspace with no snapshot returns the os.ReadFile error, which
// callers test with errors.Is(err, fs.ErrNotExist).
func AgentDomains(ctx context.Context, root string) (map[string][]string, error) {
	s, err := LoadSnapshot(root)
	if err != nil {
		return nil, err
	}
	var signals []types.Fact
	for _, row := range s.Facts {
		if row.Predicate == "profile_signal" {
			signals = append(signals, row)
		}
	}
	e, err := NewEngine(&s.Config)
	if err != nil {
		return nil, fmt.Errorf("orientation engine: %w", err)
	}
	defer e.Close()
	if len(signals) > 0 {
		if err := e.Assert(signals); err != nil {
			return nil, fmt.Errorf("assert profile signals: %w", err)
		}
	}
	if err := e.Evaluate(ctx); err != nil {
		return nil, fmt.Errorf("evaluate orientation: %w", err)
	}
	rows, err := e.Query("orient_agent_domain")
	if err != nil {
		return nil, err
	}
	out := map[string][]string{}
	for _, row := range rows {
		if len(row.Args) < 2 {
			continue
		}
		name := types.ExtractString(row.Args[0])
		key := strings.ToLower(strings.TrimSpace(types.ExtractString(row.Args[1])))
		if name == "" || key == "" {
			continue
		}
		out[name] = append(out[name], key)
	}
	for name := range out {
		sort.Strings(out[name])
		out[name] = compactSorted(out[name])
	}
	return out, nil
}

func compactSorted(in []string) []string {
	out := in[:0]
	for i, v := range in {
		if i == 0 || v != in[i-1] {
			out = append(out, v)
		}
	}
	return out
}
