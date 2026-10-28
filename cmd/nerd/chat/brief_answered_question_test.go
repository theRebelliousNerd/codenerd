package chat

import (
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// A brief question Jev judges answered stops holding the work (brief_ready),
// and the brief shown to the model agrees: it is not rendered as open.
func TestBrief_AnAnsweredQuestionIsNotShownAsOpen(t *testing.T) {
	m := admissionModel(t, config.WorkModeYolo, &perception.Brief{Goal: "fix a README typo", OpenQuestions: []string{"which typo?", "which branch?"}, Ready: true})
	if err := config.EnsureParams(m.kernel, (&config.TypeSafeConfig{}).Params()); err != nil {
		t.Fatal(err)
	}
	if err := m.kernel.AssertBatch([]core.Fact{
		{Predicate: "jev_judgment", Args: []any{types.MangleString("answered_0"), int64(94)}},
		{Predicate: "jev_judgment_about", Args: []any{types.MangleString("answered_0"), types.MangleString("which typo?")}},
	}); err != nil {
		t.Fatal(err)
	}
	lines := injectedLines(t, m)
	if strings.Contains(lines, "Brief open question: which typo?") {
		t.Fatalf("an answered question is still shown as open:\n%s", lines)
	}
	for _, want := range []string{"Brief question the user's latest message answers: which typo?", "Brief open question: which branch?"} {
		if !strings.Contains(lines, want) {
			t.Fatalf("injected brief lacks %q:\n%s", want, lines)
		}
	}
}
