//go:build jevlive

package turnjudge

// Live probe of the self-diagnosis verdict against the configured Jev:
//
//	go test -tags jevlive ./internal/turnjudge/ -run LiveDiagnosisVerdict -v
//
// Each case is a diagnosis report and the targets it covered, with the answer
// a person would give: does the report name a fix in codeNERD's own source
// for that target? The probe reports each probability against the kernel's
// thresholds (yes >= 0.70, no <= 0.30) and asserts only that the call worked:
// a probability is a measurement, not a contract. The key travels in the
// client's Authorization header and is never printed. The cases themselves are
// verdictCases (verdict_cases_test.go), shared with the offline tests.

import (
	"context"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/typesafe"
)

func TestLiveDiagnosisVerdict(t *testing.T) {
	cfg, err := config.LoadUserConfig("../../.nerd/config.json")
	if err != nil || cfg == nil || cfg.TypeSafe == nil {
		t.Skipf("no typesafe section in .nerd/config.json: %v", err)
	}
	r, err := cfg.TypeSafe.Resolve()
	if err != nil || r == nil {
		t.Skipf("typesafe section does not resolve: %v", err)
	}
	c, err := typesafe.NewClient(typesafe.Options{Model: r.Model, BaseURL: r.BaseURL, APIKey: r.APIKey, LongContext: r.LongContext, Timeout: r.Timeout})
	if err != nil {
		t.Fatal(err)
	}
	wrong, unsettled, total := 0, 0, 0
	for _, tc := range verdictCases {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		probs, err := JudgeDiagnosis(ctx, c, tc.lane, tc.report, tc.targets, 20)
		cancel()
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		for i, p := range probs {
			total++
			verdict := "ok"
			switch {
			case p > 0.30 && p < 0.70:
				verdict = "~ unsettled"
				unsettled++
			case tc.fix[i] && p < 0.70, !tc.fix[i] && p > 0.30:
				verdict = "x WRONG"
				wrong++
			}
			t.Logf("%-24s target=%d want_fix=%-5v p=%.3f  %s", tc.name, i, tc.fix[i], p, verdict)
		}
	}
	t.Logf("%d judgments: %d wrong, %d unsettled", total, wrong, unsettled)
}
