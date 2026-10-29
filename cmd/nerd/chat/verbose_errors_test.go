package chat

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/transparency"
)

// transparency.verbose_errors is honored by the error panel. The flag was
// reported by /transparency status and read by no error surface, so the
// classifier's categories and recovery guides reached nobody.
func TestErrorPanel_WhenVerboseErrorsOn_ShouldShowCategoryAndRemediation(t *testing.T) {
	cause := errors.New("dial tcp 10.0.0.1:443: connection refused")

	verbose := NewTestModel(WithSize(100, 50))
	cfg := config.DefaultTransparencyConfig()
	cfg.Enabled = true
	cfg.VerboseErrors = true
	verbose.transparencyMgr = transparency.NewTransparencyManager(cfg)

	updated, _ := verbose.Update(errorMsg(cause))
	got := updated.(Model).err
	if got == nil {
		t.Fatal("the failed turn left no error on the panel")
	}
	if !strings.Contains(got.Error(), "[NET]") || !strings.Contains(got.Error(), "Suggested fixes:") {
		t.Fatalf("verbose errors on, panel shows no category or remediation:\n%s", got.Error())
	}
	if !errors.Is(got, cause) {
		t.Fatal("the classified error no longer unwraps to the cause")
	}

	// Transparency off (the default): the error is shown as raised.
	plain := NewTestModel(WithSize(100, 50))
	updated, _ = plain.Update(errorMsg(cause))
	if got := updated.(Model).err; got == nil || got.Error() != cause.Error() {
		t.Fatalf("transparency off, panel error = %v, want the raw %q", got, cause)
	}
}

// Standing rule: a recovery step that names a slash command names one the
// chat actually has. The guides pointed at /logs and /config set-model, which
// do not exist, and the panel now shows them to people.
func TestRecoverySteps_ShouldNameOnlyRegisteredCommands(t *testing.T) {
	command := regexp.MustCompile(`(?:^|\s)(/[a-z][a-z-]*)(?:\s+([a-z][a-z-]*))?`)
	var steps []string
	for c := transparency.ErrorCategorySafety; c <= transparency.ErrorCategoryUnknown; c++ {
		steps = append(steps, transparency.GetRecoveryGuide(c)...)
	}
	// The heuristic branches carry their own step lists; reach each one.
	for _, msg := range []string{
		"blocked by constitution", "bad configuration", "rate limit hit", "kernel stratification",
		"shard spawn failed", "no such file", "connection reset", "timed out", "something odd",
	} {
		steps = append(steps, transparency.ClassifyError(errors.New(msg)).Remediation...)
	}

	for _, step := range steps {
		for _, m := range command.FindAllStringSubmatch(step, -1) {
			info := FindCommand(m[1])
			if info == nil {
				t.Errorf("recovery step %q names %s, which is not a chat command", step, m[1])
				continue
			}
			// "/config wizard" style: when the command takes a subcommand menu
			// and the step names one, it must be on the menu.
			prose := map[string]bool{"to": true, "for": true, "with": true, "and": true, "or": true}
			if sub := m[2]; sub != "" && !prose[sub] && strings.Contains(info.Usage, "[") && strings.Contains(info.Usage, "|") {
				usage := info.Usage[strings.Index(info.Usage, "[")+1:]
				if !strings.Contains("|"+strings.TrimSuffix(usage, "]")+"|", "|"+sub+"|") {
					t.Errorf("recovery step %q names %s %s; %s takes %s", step, m[1], sub, m[1], info.Usage)
				}
			}
		}
	}
}
