package session

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/gates"
	"codenerd/internal/typesafe"
)

// The child replaces all default HTTP dispatch with an in-memory transport
// before calling the real live fixture helper. No case, including explicit
// live admission, can reach a provider or a listening socket.
func TestLiveReportJudgeAdmissionChild(t *testing.T) {
	if os.Getenv("CODENERD_TEST_TYPESAFE_ADMISSION_CHILD") != "1" {
		return
	}
	t.Chdir(os.Getenv("CODENERD_TEST_TYPESAFE_ADMISSION_CWD"))
	calls := 0
	previous := http.DefaultTransport
	http.DefaultTransport = liveReportAdmissionTransport(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.URL.String() != "https://provider.invalid/v1/systemone" || req.Method != "POST" || req.Header.Get("Authorization") != "Bearer synthetic-offline-fixture" {
			return nil, fmt.Errorf("unexpected synthetic HTTP request")
		}
		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			return nil, err
		}
		if body["model"] != "typesafe-admission-fixture" {
			return nil, fmt.Errorf("configured model was not preserved")
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Request: req, Body: io.NopCloser(strings.NewReader(`{"model":"typesafe-admission-fixture","usage":{"input_tokens":1,"output_tokens":1},"answers":{"ready":{"type":"noul","noul":0.9}}}`))}, nil
	})
	t.Cleanup(func() {
		http.DefaultTransport = previous
		body, err := json.Marshal(map[string]any{"calls": calls, "admission": os.Getenv("CODENERD_LIVE_TYPESAFE")})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(os.Getenv("CODENERD_TEST_TYPESAFE_ADMISSION_RECEIPT"), body, 0600); err != nil {
			t.Fatal(err)
		}
	})
	var client interface {
		Ask(context.Context, any, map[string]typesafe.Question) (*typesafe.Response, error)
	}
	if os.Getenv("CODENERD_TEST_TYPESAFE_ADMISSION_PRODUCTION") == "1" {
		cfg, err := config.LoadUserConfig("../../.nerd/config.json")
		if err != nil {
			t.Fatal(err)
		}
		client, err = typesafe.FromConfig(cfg.TypeSafe)
		if err != nil {
			t.Fatal(err)
		}
	} else {
		resolved, _ := liveReportJudge(t)
		client = resolved.Asker
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	response, err := client.Ask(ctx, "synthetic state", map[string]typesafe.Question{"ready": typesafe.Noul{Instructions: "Is this synthetic?"}})
	if err != nil {
		t.Fatal(err)
	}
	probability, err := response.Noul("ready")
	if err != nil || probability != 0.9 || calls != 1 {
		t.Fatalf("configured fixture did not complete: p=%v calls=%d err=%v", probability, calls, err)
	}
}

type liveReportAdmissionTransport func(*http.Request) (*http.Response, error)

func (f liveReportAdmissionTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestLiveReportJudgeAdmission(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, flag, configMode              string
		gate, forward, explicit, production bool
		wantCalls                           int
	}{
		{name: "unset"},
		{name: "zero", flag: "0"},
		{name: "other_live_switch_only"},
		{name: "true_is_not_explicit_one", flag: "true"},
		{name: "whitespace_is_not_explicit_one", flag: " 1"},
		{name: "explicit_live", flag: "1", wantCalls: 1},
		{name: "missing_config", flag: "1", configMode: "missing"},
		{name: "unresolved_key", flag: "1", configMode: "missing_key"},
		{name: "ordinary_inference_without_live_test_admission", production: true, wantCalls: 1},
		{name: "gate_drops_ambient_live_admission", flag: "1", gate: true},
		{name: "gate_explicit_env_admission", flag: "1", gate: true, explicit: true, wantCalls: 1},
		{name: "gate_owner_allowlists_admission", flag: "1", gate: true, forward: true, wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// The gate cases set the ambient environment process-wide (t.Setenv), so
			// they run serially; every other case only reads it and spawns its own child.
			if !tc.gate {
				t.Parallel()
			}
			root := t.TempDir()
			cwd := filepath.Join(root, "internal/session")
			if err := os.MkdirAll(cwd, 0700); err != nil {
				t.Fatal(err)
			}
			section := map[string]any{"model": "typesafe-admission-fixture", "base_url": "https://provider.invalid", "api_key": "synthetic-offline-fixture"}
			if tc.configMode == "missing_key" {
				delete(section, "api_key")
				section["api_key_env"] = "CODENERD_TEST_TYPESAFE_MISSING_KEY"
			}
			settings := map[string]any{"typesafe": section}
			if tc.forward {
				settings["execution"] = map[string]any{"allowed_env_vars": []string{"CODENERD_LIVE_TYPESAFE"}}
			}
			if tc.configMode != "missing" {
				if err := os.MkdirAll(filepath.Join(root, ".nerd"), 0700); err != nil {
					t.Fatal(err)
				}
				body, err := json.Marshal(settings)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, ".nerd", "config.json"), body, 0600); err != nil {
					t.Fatal(err)
				}
			}
			receipt := filepath.Join(root, "receipt.json")
			values := map[string]string{"CODENERD_TEST_TYPESAFE_ADMISSION_CHILD": "1", "CODENERD_TEST_TYPESAFE_ADMISSION_CWD": cwd, "CODENERD_TEST_TYPESAFE_ADMISSION_RECEIPT": receipt, "CODENERD_TEST_TYPESAFE_MISSING_KEY": ""}
			if tc.production {
				values["CODENERD_TEST_TYPESAFE_ADMISSION_PRODUCTION"] = "1"
			} else {
				values["CODENERD_TEST_TYPESAFE_ADMISSION_PRODUCTION"] = ""
			}
			argv := []string{self, "-test.run=^TestLiveReportJudgeAdmissionChild$", "-test.v", "-test.count=1"}
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
			defer cancel()
			var output string
			if tc.gate {
				t.Setenv("CODENERD_LIVE_TYPESAFE", tc.flag)
				if tc.explicit {
					values["CODENERD_LIVE_TYPESAFE"] = "1"
				}
				result := gates.Run(ctx, root, gates.Gate{ID: "synthetic-live-admission", Kind: gates.Test, Argv: argv, Env: values}, "")
				output = result.Output
				if !result.Passed || result.Unverified() {
					t.Fatalf("gate child failed: exit=%d err=%v output=%s", result.ExitCode, result.Err, output)
				}
			} else {
				values["CODENERD_LIVE_TYPESAFE"] = tc.flag
				values["CODENERD_LIVE_LLM"] = "1" // another provider's switch is insufficient
				cmd := exec.CommandContext(ctx, self, argv[1:]...)
				cmd.Dir = root
				for _, entry := range os.Environ() {
					key, _, _ := strings.Cut(entry, "=")
					if _, replace := values[key]; !replace {
						cmd.Env = append(cmd.Env, entry)
					}
				}
				for key, value := range values {
					cmd.Env = append(cmd.Env, key+"="+value)
				}
				raw, err := cmd.CombinedOutput()
				output = string(raw)
				if err != nil {
					t.Fatalf("child: %v\n%s", err, output)
				}
			}
			data, err := os.ReadFile(receipt)
			if err != nil {
				t.Fatalf("missing dispatch receipt: %v\n%s", err, output)
			}
			var got struct {
				Calls     int    `json:"calls"`
				Admission string `json:"admission"`
			}
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			skipped := strings.Contains(output, "--- SKIP: TestLiveReportJudgeAdmissionChild")
			if got.Calls != tc.wantCalls || skipped != (tc.wantCalls == 0) {
				t.Fatalf("HTTP dispatch=%d want=%d skipped=%v admission=%q\n%s", got.Calls, tc.wantCalls, skipped, got.Admission, output)
			}
			if tc.gate && !tc.forward && !tc.explicit && got.Admission != "" {
				t.Fatal("gate forwarded unlisted ambient admission")
			}
		})
	}
}
