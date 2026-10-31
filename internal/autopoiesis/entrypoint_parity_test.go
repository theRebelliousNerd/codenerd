package autopoiesis

import (
	"context"
	"encoding/json"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCompiledEntryPointMatchesArena(t *testing.T) {
	const safe = `func Execute(ctx context.Context, input string) (string, error) { return "chosen:"+input, nil }`
	const decoy = `func Decoy(ctx context.Context, input string) (string, error) { panic("unselected entry") }`
	for _, tc := range []struct {
		name, code string
		survives   bool
	}{
		{"preferred_first", safe + "\n" + decoy, true},
		{"preferred_last", decoy + "\n" + safe, true},
		{"selected_panic", `func Execute(context.Context,string)(string,error){panic("selected entry panic")}; func Decoy(context.Context,string)(string,error){return "not selected",nil}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code := "package tools\nimport \"context\"\n" + tc.code
			cfg := DefaultOuroborosConfig(t.TempDir())
			cfg.WorkspaceRoot = ""
			cfg.TargetOS = runtime.GOOS
			cfg.TargetArch = runtime.GOARCH
			cfg.CompileTimeout = time.Minute
			compiler := NewToolCompiler(cfg)
			arena := NewThunderdomeWithConfig(ThunderdomeConfig{Timeout: 5 * time.Second, MaxMemoryMB: 100, WorkDir: t.TempDir(), ParallelAttacks: 1})
			tested, call, err := arena.findEntryPointCall(code)
			if err != nil {
				t.Fatal(err)
			}
			compiled, err := compiler.findEntryPoint(code)
			if err != nil {
				t.Fatal(err)
			}
			if tested != "Execute" || compiled != tested {
				t.Errorf("arena tests %q (%s), compiler selects %q", tested, call, compiled)
			}
			tool := &GeneratedTool{Name: "entry_parity", Code: code}
			battle, err := arena.Battle(t.Context(), tool, []AttackVector{{Name: "same-input", Input: "needle", Category: "entry"}})
			if err != nil || battle.Survived != tc.survives {
				t.Fatalf("arena: %+v / %v", battle, err)
			}
			built, err := compiler.Compile(t.Context(), tool)
			if err != nil || !built.Success {
				t.Fatalf("compile: %+v / %v", built, err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, built.OutputPath)
			cmd.Stdin = strings.NewReader(`{"input":"needle"}`)
			output, err := cmd.CombinedOutput()
			if !tc.survives {
				if err == nil || !strings.Contains(string(output), "selected entry panic") {
					t.Fatalf("selected panic hidden by native wrapper: %v\n%s", err, output)
				}
				return
			}
			if err != nil {
				t.Fatalf("arena survived but compiled executable failed: %v\n%s", err, output)
			}
			var response struct {
				Output string `json:"output"`
				Error  string `json:"error"`
			}
			if err := json.Unmarshal(output, &response); err != nil || response.Output != "chosen:needle" || response.Error != "" {
				t.Fatalf("different executed entry: %s / %v", output, err)
			}
		})
	}
}

func TestCompiledEntrySelectionControls(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"tie_first", `func Alpha(context.Context,string)(string,error){return "a",nil}; func Beta(context.Context,string)(string,error){return "b",nil}`, "Alpha"},
		{"wrong_signature_helper", `func Execute(context.Context,string,int)(string,error){return "bad",nil}; func Valid(context.Context,string)(string,error){return "ok",nil}`, "Valid"},
		{"method_not_callable", `type Holder struct{}; func (Holder) Execute(context.Context,string)(string,error){return "bad",nil}; func Valid(context.Context,string)(string,error){return "ok",nil}`, "Valid"},
		{"generic_not_inferable", `func Execute[T any](context.Context,string)(string,error){return "bad",nil}; func Valid(context.Context,string)(string,error){return "ok",nil}`, "Valid"},
		{"skip_test_helper", `func TestHelper(context.Context,string)(string,error){return "bad",nil}; func Valid(context.Context,string)(string,error){return "ok",nil}`, "Valid"},
		{"named_returns", `func Valid(ctx context.Context,input string)(out string,err error){return input,nil}`, "Valid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code := "package tools\nimport \"context\"\n" + tc.body
			compiler := NewToolCompiler(OuroborosConfig{})
			arena := &Thunderdome{}
			got, err := compiler.findEntryPoint(code)
			if err != nil || got != tc.want {
				t.Errorf("compiler=%q/%v want=%q", got, err, tc.want)
			}
			tested, _, err := arena.findEntryPointCall(code)
			if err != nil || tested != tc.want {
				t.Errorf("arena=%q/%v want=%q", tested, err, tc.want)
			}
		})
	}
}
