package system

import "testing"

func TestDefaultRouterConfig_RoutesPolicyMappedActions(t *testing.T) {
	r := NewTactileRouterShard()

	actions := []string{
		"/analyze_code",
		"/fs_read",
		"/fs_write",
		"/search_files",
		"/exec_cmd",
		"/run_tests",
		"/delegate_reviewer",
		"/delegate_coder",
		"/delegate_researcher",
		"/delegate_tool_generator",
		"/show_diff",
	}

	for _, action := range actions {
		if _, ok := r.findRoute(action); !ok {
			t.Fatalf("expected default router to route action %s", action)
		}
	}
}

