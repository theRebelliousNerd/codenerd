package features

import (
	"fmt"
	"strings"
)

// ConfigSchemaJSON returns a documented JSON snippet for the `features` block
// of .nerd/config.json, showing every recognised key with its compile-time
// default and the environment variable that overrides it.
//
// It is generated from boolFlags/intFlags rather than hand-written so a
// hand-written doc snippet cannot drift from the accessors — which is exactly
// how the previous documentation ended up naming env vars that no accessor
// read. `nerd features --schema` prints this, and the features corpus embeds
// the same output.
//
// The result is deliberately NOT valid strict JSON: it carries // comments,
// because a schema an operator cannot read is not documentation. Use
// ConfigSchemaKeys when a machine needs the key list.
func ConfigSchemaJSON() string {
	var sb strings.Builder
	sb.WriteString("{\n")
	sb.WriteString("  // .nerd/config.json — every key below is optional.\n")
	sb.WriteString("  // Omitting a key uses the default shown; precedence is\n")
	sb.WriteString("  // canonical env > legacy env > this block > default.\n")
	sb.WriteString("  \"features\": {\n")

	for i, f := range boolFlags {
		comma := ","
		if i == len(boolFlags)-1 && len(intFlags) == 0 {
			comma = ""
		}
		sb.WriteString(fmt.Sprintf("    %-22s %-7s // %s\n",
			"\""+f.name+"\":", fmt.Sprintf("%t%s", f.def, comma), envHint(f.envVar, f.legacyEnvVar)))
	}

	// The integer overrides are zero-valued rather than absent-valued: zero
	// means "call site picks", which is why they carry no *bool treatment.
	for i, f := range intFlags {
		comma := ","
		if i == len(intFlags)-1 {
			comma = ""
		}
		sb.WriteString(fmt.Sprintf("    %-22s %-7s // %s; 0 = call site default\n",
			"\""+f.name+"\":", "0"+comma, envHint(f.envVar, f.legacyEnvVar)))
	}

	sb.WriteString("  }\n")
	sb.WriteString("}\n")
	return sb.String()
}

// ConfigSchemaKeys returns every recognised key of the `features` block, in the
// order ConfigSchemaJSON prints them.
func ConfigSchemaKeys() []string {
	keys := make([]string, 0, len(boolFlags)+len(intFlags))
	for _, f := range boolFlags {
		keys = append(keys, f.name)
	}
	for _, f := range intFlags {
		keys = append(keys, f.name)
	}
	return keys
}

func envHint(envVar, legacyEnvVar string) string {
	if legacyEnvVar == "" {
		return "env: " + envVar
	}
	return "env: " + envVar + " (legacy: " + legacyEnvVar + ")"
}
