package campaign

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"codenerd/internal/session"
	"codenerd/internal/tactile"
	"codenerd/internal/tools"
	toolshell "codenerd/internal/tools/shell"

	"github.com/kballard/go-shellquote"
)

// A real immutable checker, launched through gates.Run and tactile. The
// command marker, rather than an environment flag, identifies child mode so
// loss of the declared overlay cannot silently skip the checker.
func TestRecurseContractProcess(t *testing.T) {
	if len(os.Args) < 3 || os.Args[len(os.Args)-3] != "recurse-contract" {
		return
	}
	mode := os.Args[len(os.Args)-2]
	body, err := os.ReadFile("store/check.txt")
	if err != nil || (mode != "exit" && os.Getenv("CHECK_PROFILE") != "strict") || strings.TrimSpace(string(body)) != "fixed" {
		fmt.Println("store/check.txt:1: strict contract failed")
		os.Exit(1)
	}
	if mode == "environment" {
		os.Exit(0)
	}
	os.Exit(3)
}

func TestRecurseContract_EnvironmentAndExitsReachTaskAndAcceptance(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		mode    string
		envYAML string
		codes   []int
		pass    int
	}{
		{"environment", "commands:\n  env:\n    CHECK_PROFILE: strict\n", []int{0}, 0},
		{"exit", "", []int{0, 3}, 3},
		{"combined", "commands:\n  env:\n    CHECK_PROFILE: strict\n", []int{0, 3}, 3},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			command := shellquote.Join(binary, "-test.run=^TestRecurseContractProcess$", "--", "recurse-contract", tc.mode, "{node}")
			root := gitFixture(t, map[string]string{
				".gitignore": ".nerd/\n", "go.mod": "module example.com/contract\n\ngo 1.21\n",
				"store/store.go": "package store\n", "store/check.txt": "broken\n",
				"nerd.md": fmt.Sprintf(`---
schema: nerd/v1
%sgates:
  - id: contract
    kind: audit
    run: %q
    scope: node
    ok_exit_codes: %s
---
`, tc.envYAML, command, acceptanceExitList(tc.codes)),
			}).root
			cfg := tactile.DefaultExecutorConfig()
			cfg.BaseEnvironment = []string{"CHECK_PROFILE=ambient-conflict"}
			exec := tactile.NewDirectExecutorWithConfig(cfg)
			toolshell.SetCheckExecutor(exec)
			t.Cleanup(func() { toolshell.ClearCheckExecutor(exec) })
			var witnessed int
			res, err := runRecurse(t, context.Background(), root, 1, onlyFixes(func(ctx context.Context, a RecurseAttempt) error {
				if a.Finding.Gate != "nerd.md:contract" {
					return nil
				}
				witnessed++
				if tc.envYAML != "" && a.CheckEnv["CHECK_PROFILE"] != "strict" {
					t.Fatal("reporting gate lost its declared environment")
				}
				c := RecurseAttemptCampaign(root, a)
				if len(a.CheckEnv) > 0 {
					a.CheckEnv["CHECK_PROFILE"] = "caller-mutated"
				}
				a.OKExitCodes[0] = 9
				for _, value := range []any{a, c} {
					raw, err := json.Marshal(value)
					if err != nil {
						t.Fatal(err)
					}
					if strings.Contains(string(raw), "CHECK_PROFILE") || strings.Contains(string(raw), "caller-mutated") {
						t.Fatal("environment leaked into model/snapshot JSON")
					}
				}
				clone, err := cloneCampaign(c)
				if err != nil {
					t.Fatal(err)
				}
				if clone.Acceptance.runtime == c.Acceptance.runtime {
					t.Fatal("campaign clone aliases runtime configuration")
				}
				if len(c.Acceptance.runtime.env) > 0 {
					c.Acceptance.runtime.env["CHECK_PROFILE"] = "changed-after-clone"
				}
				c.Acceptance.runtime.command[0] = "changed-after-clone"
				c.Acceptance.runtime.okExits[0] = 9
				write(t, root, "store/check.txt", "fixed\n")
				o := acceptanceOrchestrator(t, clone.Acceptance, exec)
				o.workspace = root
				o.campaign.Type = CampaignTypeRecurse
				o.taskExecutor = &MockTaskExecutor{ExecuteFunc: func(ctx context.Context, _ session.TaskRequest) (string, error) {
					check, ok := tools.CampaignCheckFrom(ctx)
					if !ok || !slices.Equal(check.OKExitCodes, tc.codes) || (tc.envYAML != "" && !slices.Contains(check.Environment, "CHECK_PROFILE=strict")) {
						t.Fatal("task context lost the check contract")
					}
					ctx, runs := tools.WithAcceptanceRunLog(ctx)
					out, err := toolshell.RunCheckTool().Execute(ctx, nil)
					if err != nil {
						return out, err
					}
					if got := runs(); len(got) != 1 || got[0].ExitCode != tc.pass || !slices.Equal(got[0].OKExitCodes, tc.codes) {
						t.Fatalf("task receipt lost raw exit/pass rule: %+v", got)
					}
					return out, nil
				}}
				ctx = tools.WithWorkspaceRoot(ctx, root)
				if _, err := o.spawnTask(ctx, &Task{ID: "/task_contract", Description: "repair checker input", Type: TaskTypeFileModify}, "/fix", "repair checker input"); err != nil {
					return err
				}
				outcome, err := o.settleAcceptance(ctx)
				if err != nil || outcome != acceptanceSatisfied {
					t.Fatalf("acceptance = %v, %v", outcome, err)
				}
				if len(o.campaign.Acceptance.Rounds) != 1 || o.campaign.Acceptance.Rounds[0].ExitCode != tc.pass {
					t.Fatal("acceptance did not preserve the raw accepted exit")
				}
				return nil
			}))
			if err != nil {
				t.Fatal(err)
			}
			if witnessed != 1 || res.Kept != 1 {
				t.Fatalf("witnessed=%d, result=%+v", witnessed, res)
			}
		})
	}
}

func acceptanceExitList(codes []int) string {
	parts := make([]string, len(codes))
	for i, code := range codes {
		parts[i] = fmt.Sprint(code)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func TestRecurseContract_RestoredAndLegacyCampaignsCannotInventConfiguration(t *testing.T) {
	root := t.TempDir()
	c := RecurseAttemptCampaign(root, RecurseAttempt{Check: []string{"checker"}, CheckEnv: map[string]string{"CHECK_PROFILE": "strict"}})
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var restored Campaign
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	for _, legacy := range []bool{false, true} {
		restored.Acceptance.RuntimeContract = !legacy
		exec := &scriptedAcceptance{exits: []int{0}}
		o := &Orchestrator{workspace: root, campaign: &restored, executor: exec,
			taskExecutor: &MockTaskExecutor{ExecuteFunc: func(context.Context, session.TaskRequest) (string, error) {
				t.Fatal("restored task ran")
				return "", nil
			}}}
		if err := o.runAcceptanceRound(context.Background()); err == nil {
			t.Fatal("restored contract was treated as an empty environment")
		}
		if err := o.Run(context.Background()); err == nil {
			t.Fatal("restored campaign entered execution without its runtime contract")
		}
		if _, err := o.settleAcceptance(context.Background()); err == nil {
			t.Fatal("restored campaign settled without its runtime contract")
		}
		if _, err := o.spawnTask(context.Background(), &Task{ID: "/task_restored"}, "/fix", "fix"); err == nil {
			t.Fatal("restored task accepted missing configuration")
		}
		if len(exec.ran) != 0 {
			t.Fatal("restored check executed")
		}
	}
}

func TestRecurseContract_ChangedDeclarationOrWorkspaceCannotExecute(t *testing.T) {
	for _, change := range []string{"command", "exits", "gate", "workspace", "removed command"} {
		t.Run(change, func(t *testing.T) {
			root := t.TempDir()
			c := RecurseAttemptCampaign(root, RecurseAttempt{Check: []string{"checker"}, OKExitCodes: []int{0, 3}})
			exec := &scriptedAcceptance{exits: []int{0}}
			o := &Orchestrator{workspace: root, campaign: c, executor: exec}
			switch change {
			case "command":
				c.Acceptance.Command[0] = "another-checker"
			case "exits":
				c.Acceptance.OKExitCodes[1] = 9
			case "gate":
				c.Acceptance.GateID = "another-gate"
			case "workspace":
				o.workspace = t.TempDir()
			case "removed command":
				c.Acceptance.Command = nil
			}
			if err := o.Run(context.Background()); err == nil {
				t.Fatal("changed contract entered execution")
			}
			if _, _, err := o.campaignCheck(&Task{ID: "/task_changed"}); err == nil {
				t.Fatal("changed contract reached task context")
			}
			if len(exec.ran) != 0 {
				t.Fatal("changed check executed")
			}
		})
	}
}
