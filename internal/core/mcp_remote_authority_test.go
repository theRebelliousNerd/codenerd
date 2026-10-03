package core

import (
	"fmt"
	"strconv"
	"testing"
)

func TestMCPRemoteAuthority_ConstitutionAndRequestBinding(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		effect    string
		action    string
		confirmed bool
		approved  bool
		allowed   bool
	}{
		{"read", "/read", "/read_file", false, false, true},
		{"write", "/write", "/write_file", false, false, true},
		{"delete-confirm-only", "/delete", "/delete_file", true, false, false},
		{"execute-confirm-only", "/execute", "/run_arbitrary_command", true, false, false},
		{"delete-approved", "/delete", "/delete_file", true, true, true},
		{"execute-approved", "/execute", "/run_arbitrary_command", true, true, true},
		{"approved-without-confirmation", "/execute", "/run_arbitrary_command", false, true, false},
		{"outer-verb-is-not-execution-authority", "/execute", "/mcp_call", true, false, false},
		{"unknown-effect", "/unknown", "/read_file", true, false, false},
	} {
		t.Run(scenario.name, func(test *testing.T) {
			kernel, err := NewRealKernelWithWorkspace(test.TempDir())
			if err != nil {
				test.Fatal(err)
			}
			assert := func(fact string) {
				test.Helper()
				if err := kernel.AssertString(fact); err != nil {
					test.Fatal(err)
				}
			}
			for _, fact := range []string{
				`mcp_server_registered("srv", "endpoint", /http, 1)`,
				`mcp_server_status("srv", /connected)`,
				`mcp_tool_registered("srv/op", "srv", 1)`,
				`mcp_tool_name("srv/op", "op")`,
				`mcp_tool_risk("srv/op", /safe)`,
				`mcp_tool_risk_source("srv/op", /annotation)`,
				`mcp_tool_schema_hash("srv/op", "schema")`,
			} {
				assert(fact)
			}
			if scenario.approved {
				assert(fmt.Sprintf("signed_approval(%s)", scenario.action))
				assert(`admin_override("host-reviewer")`)
			}
			confirmed := "/false"
			if scenario.confirmed {
				confirmed = "/true"
			}
			payload := strconv.Quote(`{"request_id":"exec-mcp-test","scope":"scope","call_id":"call","server_id":"srv","tool_id":"srv/op","args":{"text":"λ\u0000\""}}`)
			transient := []string{
				`mcp_remote_operation("exec-mcp-test", /tool)`,
				fmt.Sprintf(`mcp_remote_reviewed("exec-mcp-test", "srv", "srv/op", "schema", %s)`, scenario.effect),
				fmt.Sprintf(`mcp_remote_request("exec-mcp-test", "scope", "call", "srv", "srv/op", "schema", %s, %s, "srv/op", %s, "digest", /safe, %s)`, scenario.effect, scenario.action, payload, confirmed),
				fmt.Sprintf(`pending_action("exec-mcp-test", %s, "srv/op", %s, 1)`, scenario.action, payload),
			}
			for _, fact := range transient {
				assert(fact)
			}
			query := `mcp_remote_permitted("exec-mcp-test", "scope", "call", "srv", "srv/op", "schema", "digest")`
			rows, err := kernel.Query(query)
			if err != nil {
				test.Fatal(err)
			}
			if (len(rows) == 1) != scenario.allowed {
				test.Fatalf("positive authority: got %d rows, want allowed=%v", len(rows), scenario.allowed)
			}
			if scenario.allowed {
				for _, mismatch := range []string{
					`mcp_remote_permitted("other-request", "scope", "call", "srv", "srv/op", "schema", "digest")`,
					`mcp_remote_permitted("exec-mcp-test", "other-scope", "call", "srv", "srv/op", "schema", "digest")`,
					`mcp_remote_permitted("exec-mcp-test", "scope", "other-call", "srv", "srv/op", "schema", "digest")`,
					`mcp_remote_permitted("exec-mcp-test", "scope", "call", "other-server", "srv/op", "schema", "digest")`,
					`mcp_remote_permitted("exec-mcp-test", "scope", "call", "srv", "other-tool", "schema", "digest")`,
					`mcp_remote_permitted("exec-mcp-test", "scope", "call", "srv", "srv/op", "other-schema", "digest")`,
					`mcp_remote_permitted("exec-mcp-test", "scope", "call", "srv", "srv/op", "schema", "other-digest")`,
				} {
					rows, err := kernel.Query(mismatch)
					if err != nil || len(rows) != 0 {
						test.Fatalf("mismatched authority %s: %d, %v", mismatch, len(rows), err)
					}
				}
				permission, err := kernel.Query(fmt.Sprintf("permitted(%s, \"srv/op\", %s)", scenario.action, payload))
				if err != nil || len(permission) != 1 {
					test.Fatalf("corresponding constitutional permission absent: %v", err)
				}
				rows, err := kernel.Query(fmt.Sprintf("permitted(%s, \"srv/op\", \"old-payload\")", scenario.action))
				if err != nil || len(rows) != 0 {
					test.Fatalf("different payload inherits permission: %v", err)
				}
			}
			for _, predicate := range []string{"permitted_action", "permission_check_result", "next_action"} {
				rows, err := kernel.Query(predicate)
				if err != nil || len(rows) != 0 {
					test.Fatalf("remote admission issued executable %s: %d, %v", predicate, len(rows), err)
				}
			}
			parsed := make([]Fact, 0, len(transient))
			for _, fact := range transient {
				item, err := ParseFactString(fact)
				if err != nil {
					test.Fatal(err)
				}
				parsed = append(parsed, item)
			}
			if err := kernel.RetractExactFactsBatch(parsed); err != nil {
				test.Fatal(err)
			}
			for _, predicate := range []string{"mcp_remote_request", "mcp_remote_reviewed", "mcp_remote_operation", "mcp_remote_permitted", "pending_action"} {
				rows, err := kernel.Query(predicate)
				if err != nil || len(rows) != 0 {
					test.Fatalf("authority survives cleanup for %s: %d, %v", predicate, len(rows), err)
				}
			}
		})
	}
}
