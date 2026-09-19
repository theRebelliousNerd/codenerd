package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Keys that were once valid in .nerd/config.json and are now gone, by the
// section they lived in, each with the reason an operator needs in order to
// act.
//
// decodeStrictJSON would already refuse the file -- it disallows unknown
// fields -- but its message ("unknown field \"diff_eval\"") reads like a typo
// and tells the operator nothing about why the key vanished or whether their
// configuration still does what they meant. A removed key is a behaviour
// change, so the failure names the key and says what happened to it.
//
// This is not a compatibility path. Nothing decodes, stores or honours these;
// the load fails and the message says what to delete.
var (
	// removedFeatureKeys: the `features` block.
	removedFeatureKeys = map[string]string{
		"diff_eval": "the differential evaluation path was deleted: it could not " +
			"un-derive a fact whose negated premise became true, and it accumulated " +
			"aggregate results instead of replacing them. The kernel now always " +
			"rebuilds the store from the EDB, which is what diff_eval=false already " +
			"did. Delete this key",
	}

	// removedCoreLimitKeys: `core_limits`. The tool-budget keys went on
	// 2026-09-18 when the tool loop stopped being bounded by counts; the
	// session ceiling on 2026-09-19 with the other run-level wall clocks.
	removedCoreLimitKeys = map[string]string{
		"max_tool_calls":                "no count of tool calls ends a turn; delete the key",
		"max_tool_iterations":           "no count of rounds ends a turn; delete the key",
		"adaptive_tool_budget":          "there is no ceiling left to extend; delete the key",
		"tool_iteration_extension_size": "there is no ceiling left to extend; delete the key",
		"max_tool_iteration_extensions": "there is no ceiling left to extend; delete the key",
		"tool_loop_repeat_threshold":    "the repeat span is policy: working_repeat_threshold in internal/context/working_set.mg; delete the key",
		"max_session_duration_min":      "a session has no wall clock: it runs while it makes progress; delete the key",
	}

	// removedLLMTimeoutKeys: `llm_timeouts`. The keys that bounded a run
	// rather than a request, removed 2026-09-19.
	removedLLMTimeoutKeys = map[string]string{
		"shard_execution_timeout":     "a shard runs while it makes progress; the working policy stops a stall; delete the key",
		"ooda_loop_timeout":           "a turn runs while it makes progress; the working policy stops a stall; delete the key",
		"campaign_phase_timeout":      "a campaign phase runs while it makes progress; delete the key",
		"document_processing_timeout": "ingestion runs to completion, each model request bounded by per_call_timeout; delete the key",
		"ouroboros_timeout":           "tool generation runs to completion, each model request bounded by per_call_timeout; delete the key",
	}

	// removedShardProfileKeys: every profile under `shard_profiles`, and
	// `default_shard`. Removed 2026-09-19.
	removedShardProfileKeys = map[string]string{
		"max_execution_time_sec": "a shard's task runs while it makes progress; the only wall clock on a run is the user's --timeout; delete the key",
	}
)

// rejectRemovedKeys fails a config load that still carries a key this
// codebase no longer honours. It runs BEFORE decodeStrictJSON so the specific
// explanation wins over the strict decoder's generic unknown-field error.
//
// Malformed JSON is not this function's problem: it returns nil and lets
// decodeStrictJSON produce the parse error.
func rejectRemovedKeys(data []byte) error {
	var envelope struct {
		CoreLimits    map[string]json.RawMessage            `json:"core_limits"`
		LLMTimeouts   map[string]json.RawMessage            `json:"llm_timeouts"`
		ShardProfiles map[string]map[string]json.RawMessage `json:"shard_profiles"`
		DefaultShard  map[string]json.RawMessage            `json:"default_shard"`
		Features      map[string]json.RawMessage            `json:"features"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil
	}

	var reasons []string
	reasons = append(reasons, removedIn("core_limits", envelope.CoreLimits, removedCoreLimitKeys)...)
	reasons = append(reasons, removedIn("llm_timeouts", envelope.LLMTimeouts, removedLLMTimeoutKeys)...)
	profiles := make([]string, 0, len(envelope.ShardProfiles))
	for name := range envelope.ShardProfiles {
		profiles = append(profiles, name)
	}
	sort.Strings(profiles)
	for _, name := range profiles {
		reasons = append(reasons, removedIn("shard_profiles."+name, envelope.ShardProfiles[name], removedShardProfileKeys)...)
	}
	reasons = append(reasons, removedIn("default_shard", envelope.DefaultShard, removedShardProfileKeys)...)
	reasons = append(reasons, removedIn("features", envelope.Features, removedFeatureKeys)...)
	if len(reasons) == 0 {
		return nil
	}
	return errors.New("remove from .nerd/config.json: " + strings.Join(reasons, "; "))
}

// removedIn names each key of section that is in removed, in key order, as
// "section.key is no longer a supported key: reason".
func removedIn(section string, present map[string]json.RawMessage, removed map[string]string) []string {
	var found []string
	for key := range present {
		if _, gone := removed[key]; gone {
			found = append(found, key)
		}
	}
	sort.Strings(found)
	out := make([]string, 0, len(found))
	for _, key := range found {
		out = append(out, fmt.Sprintf("%s.%s is no longer a supported key: %s", section, key, removed[key]))
	}
	return out
}
