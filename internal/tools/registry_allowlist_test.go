package tools

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func probeTool(name string, ran *bool) *Tool {
	return &Tool{
		Name:        name,
		Description: "probe",
		Category:    CategoryCode,
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			*ran = true
			return "ok", nil
		},
	}
}

// The contract: an absent capability envelope is not a grant of every
// capability. session.Executor.isToolAllowed already fails closed on an empty
// AllowedTools; the registry is the layer underneath it, reachable
// process-globally via tools.Execute, and it used to run anything registered.

func TestAllowlist_WhenEnforcedAndEmpty_ShouldDenyEveryTool(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	ran := false
	r.MustRegister(probeTool("write_file", &ran))

	r.SetAllowlist(&Allowlist{Enforced: true})

	_, err := r.Execute(context.Background(), "write_file", map[string]any{})
	if err == nil {
		t.Fatal("an enforced-but-empty allowlist permitted execution")
	}
	if !errors.Is(err, ErrToolNotAllowed) {
		t.Fatalf("expected ErrToolNotAllowed, got %v", err)
	}
	if ran {
		t.Fatal("the tool body ran despite being outside the envelope")
	}
}

func TestAllowlist_WhenEnforcedAndListed_ShouldPermit(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	ran := false
	r.MustRegister(probeTool("read_file", &ran))

	r.SetAllowlist(&Allowlist{Enforced: true, Names: []string{"read_file"}})

	if _, err := r.Execute(context.Background(), "read_file", map[string]any{}); err != nil {
		t.Fatalf("listed tool refused: %v", err)
	}
	if !ran {
		t.Fatal("listed tool did not execute")
	}
}

func TestAllowlist_WhenEnforcedAndUnlisted_ShouldDeny(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	readRan, writeRan := false, false
	r.MustRegister(probeTool("read_file", &readRan))
	r.MustRegister(probeTool("write_file", &writeRan))

	r.SetAllowlist(&Allowlist{Enforced: true, Names: []string{"read_file"}})

	if _, err := r.Execute(context.Background(), "write_file", map[string]any{}); !errors.Is(err, ErrToolNotAllowed) {
		t.Fatalf("expected ErrToolNotAllowed, got %v", err)
	}
	if writeRan {
		t.Fatal("unlisted tool executed")
	}
}

func TestAllowlist_WhenNotEnforced_ShouldPermitEverything(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	ran := false
	r.MustRegister(probeTool("write_file", &ran))

	// Explicitly not enforced: the CLI/dev case, which must stay open.
	r.SetAllowlist(&Allowlist{Enforced: false})
	if _, err := r.Execute(context.Background(), "write_file", map[string]any{}); err != nil {
		t.Fatalf("unenforced allowlist refused execution: %v", err)
	}
	if !ran {
		t.Fatal("tool did not run with allowlist off")
	}
}

func TestAllowlist_WhenSetLaterMutated_ShouldNotAffectRegistry(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	ran := false
	r.MustRegister(probeTool("write_file", &ran))

	names := []string{"read_file"}
	r.SetAllowlist(&Allowlist{Enforced: true, Names: names})
	names[0] = "write_file" // caller mutates its own slice afterwards

	if _, err := r.Execute(context.Background(), "write_file", map[string]any{}); !errors.Is(err, ErrToolNotAllowed) {
		t.Fatalf("caller-side mutation widened the envelope: %v", err)
	}
}

func TestFilterByIntent_WhenAllowlistEnforced_ShouldNotWidenOnUnknownIntent(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	a, b := false, false
	r.MustRegister(probeTool("read_file", &a))
	r.MustRegister(probeTool("write_file", &b))

	r.SetAllowlist(&Allowlist{Enforced: true, Names: []string{"read_file"}})

	// An unknown intent falls back to "everything", which must still mean
	// "everything the envelope permits".
	got := r.FilterByIntent("/not-a-real-intent")
	if len(got) != 1 || got[0].Name != "read_file" {
		names := make([]string, 0, len(got))
		for _, tool := range got {
			names = append(names, tool.Name)
		}
		t.Fatalf("unknown-intent fallback leaked tools outside the envelope: %v", names)
	}
}

func TestRegistry_WhenWorkspaceRootSet_ShouldReachToolViaContext(t *testing.T) {
	// Not parallel: t.Setenv below pins the process-global fallback this test
	// has to out-rank.
	root := t.TempDir()
	r := NewRegistry()
	r.SetWorkspaceRoot(root)

	var seen string
	r.MustRegister(&Tool{
		Name:     "root_probe",
		Category: CategoryGeneral,
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			got, err := WorkspaceRoot(ctx)
			seen = got
			return got, err
		},
	})

	// Env points somewhere else: the registry's root must win, which is the
	// whole point of retiring the process-global coupling.
	t.Setenv("CODENERD_WORKSPACE_ROOT", t.TempDir())

	if _, err := r.Execute(context.Background(), "root_probe", map[string]any{}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if seen != filepath.Clean(root) {
		t.Fatalf("tool saw workspace root %q, want %q", seen, root)
	}
}

func TestRegistry_WhenContextCarriesRoot_ShouldNotOverrideIt(t *testing.T) {
	t.Parallel()
	registryRoot := t.TempDir()
	callRoot := t.TempDir()

	r := NewRegistry()
	r.SetWorkspaceRoot(registryRoot)

	var seen string
	r.MustRegister(&Tool{
		Name:     "root_probe",
		Category: CategoryGeneral,
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			seen, _ = WorkspaceRoot(ctx)
			return seen, nil
		},
	})

	// codedom apply_edits narrows the root for a single call to stage edits
	// under a temp tree; the registry must not clobber that.
	if _, err := r.Execute(WithWorkspaceRoot(context.Background(), callRoot), "root_probe", nil); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if seen != filepath.Clean(callRoot) {
		t.Fatalf("registry overrode a per-call workspace root: got %q want %q", seen, callRoot)
	}
}

func TestRegistry_Metrics_ShouldCountSuccessesAndFailures(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	fail := false
	r.MustRegister(&Tool{
		Name:     "flaky",
		Category: CategoryGeneral,
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			if fail {
				return "", errors.New("boom")
			}
			return "ok", nil
		},
	})

	_, _ = r.Execute(context.Background(), "flaky", nil)
	fail = true
	_, _ = r.Execute(context.Background(), "flaky", nil)
	_, _ = r.Execute(context.Background(), "flaky", nil)

	m := r.Metrics("flaky")
	if m.Calls != 3 || m.Successes != 1 || m.Failures != 2 {
		t.Fatalf("unexpected counters: %+v", m)
	}
	if got := m.SuccessRate(); got < 0.33 || got > 0.34 {
		t.Fatalf("SuccessRate = %v, want ~0.333", got)
	}
	if _, ok := r.AllMetrics()["flaky"]; !ok {
		t.Fatal("AllMetrics did not include the tool")
	}
}

func TestRegistry_FactSink_ShouldReceiveOneRecordPerExecution(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	r.MustRegister(&Tool{
		Name:     "recorded",
		Category: CategoryGeneral,
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			return "ok", nil
		},
	})

	type record struct {
		name    string
		success bool
	}
	var got []record
	r.SetFactSink(func(_ context.Context, name string, success bool, _ int64, _ int64) {
		got = append(got, record{name, success})
	})

	if _, err := r.Execute(context.Background(), "recorded", nil); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(got) != 1 || got[0].name != "recorded" || !got[0].success {
		t.Fatalf("fact sink saw %+v", got)
	}
}

func TestRegistry_FactSink_ShouldNotFireForRefusedExecution(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	ran := false
	r.MustRegister(probeTool("blocked", &ran))
	r.SetAllowlist(&Allowlist{Enforced: true})

	fired := false
	r.SetFactSink(func(context.Context, string, bool, int64, int64) { fired = true })

	_, _ = r.Execute(context.Background(), "blocked", nil)
	// A refusal is not an execution: recording it as one would teach the
	// kernel that the tool fails, when it never ran.
	if fired {
		t.Fatal("fact sink fired for a call the allowlist refused")
	}
}
