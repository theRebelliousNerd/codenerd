package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"codenerd/internal/campaign"

	"github.com/spf13/cobra"
)

// Operator surface for the campaign durability journal and assault evidence.
//
// Both already existed on disk and neither was reachable. `.nerd/campaigns`
// holds an append-only journal with per-event checksums and a snapshot commit
// protocol, plus (for assault runs) a directory of per-batch JSONL results —
// and the only way to ask "did the last snapshot commit?" or "what actually
// failed?" was to read the files by hand.

var (
	campaignJournalCampaignID string
	campaignJournalJSON       bool
	campaignJournalLimit      int
	campaignReportJSON        bool
	campaignReportStdout      bool
)

// campaignJournalCmd groups journal inspection subcommands.
var campaignJournalCmd = &cobra.Command{
	Args:  cobra.NoArgs,
	Use:   "journal",
	Short: "Inspect the durable campaign journal",
	Long: `Inspect the append-only journal that makes campaigns resumable.

Every campaign save writes a journal event BEFORE the snapshot and a second one
after the atomic rename commits. That pairing is what lets a killed process tell
a committed snapshot from a half-written one.

Examples:
  nerd campaign journal verify
  nerd campaign journal verify --campaign campaign_ab12cd34
  nerd campaign journal replay --limit 20`,
	RunE: parentGroupRunE,
}

var campaignJournalVerifyCmd = &cobra.Command{
	Use:   "verify",
	Args:  cobra.NoArgs,
	Short: "Verify journal integrity and snapshot consistency",
	RunE:  runCampaignJournalVerify,
}

var campaignJournalReplayCmd = &cobra.Command{
	Use:   "replay",
	Args:  cobra.NoArgs,
	Short: "Replay a campaign's recorded progress history",
	RunE:  runCampaignJournalReplay,
}

// campaignReportCmd exports an aggregated assault report.
var campaignReportCmd = &cobra.Command{
	Use:   "report",
	Args:  cobra.NoArgs,
	Short: "Export an aggregated assault summary report",
	Long: `Aggregate an adversarial assault campaign's evidence into one report.

Assault runs persist one JSONL results file per batch plus per-stage logs. This
joins them into per-stage and per-target summaries, worst offenders first, and
writes summary.md and summary.json next to the evidence.

Examples:
  nerd campaign report
  nerd campaign report --campaign campaign_ab12cd34
  nerd campaign report --stdout`,
	RunE: runCampaignReport,
}

func init() {
	campaignJournalVerifyCmd.Flags().StringVar(&campaignJournalCampaignID, "campaign", "",
		"Campaign ID to inspect (default: the most recently written journal)")
	campaignJournalVerifyCmd.Flags().BoolVar(&campaignJournalJSON, "json", false,
		"Emit the verification result as JSON")

	campaignJournalReplayCmd.Flags().StringVar(&campaignJournalCampaignID, "campaign", "",
		"Campaign ID to replay (default: the most recently written journal)")
	campaignJournalReplayCmd.Flags().BoolVar(&campaignJournalJSON, "json", false,
		"Emit the replay as JSON")
	campaignJournalReplayCmd.Flags().IntVar(&campaignJournalLimit, "limit", 0,
		"Show only the last N snapshot points (0 = all)")

	campaignJournalCmd.AddCommand(campaignJournalVerifyCmd, campaignJournalReplayCmd)

	campaignReportCmd.Flags().StringVar(&campaignJournalCampaignID, "campaign", "",
		"Campaign ID to report on (default: the most recently written journal)")
	campaignReportCmd.Flags().BoolVar(&campaignReportJSON, "json", false,
		"Emit the summary as JSON on stdout")
	campaignReportCmd.Flags().BoolVar(&campaignReportStdout, "stdout", false,
		"Print the Markdown report instead of only writing it")

	// campaignCmd lives in cmd_campaign.go in this same package; registering
	// here keeps main.go untouched.
	campaignCmd.AddCommand(campaignJournalCmd, campaignReportCmd)
}

func campaignWorkspace() string {
	cwd := workspace
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	return cwd
}

// resolveCampaignID falls back to the most recently written journal, which is
// almost always the campaign the operator just ran.
func resolveCampaignID(cwd string) (string, error) {
	if strings.TrimSpace(campaignJournalCampaignID) != "" {
		return strings.TrimPrefix(strings.TrimSpace(campaignJournalCampaignID), "/"), nil
	}
	ids, err := campaign.ListCampaignJournals(cwd)
	if err != nil {
		return "", err
	}
	if len(ids) == 0 {
		return "", fmt.Errorf("no campaign journals found under %s/.nerd/campaigns", cwd)
	}
	return ids[0], nil
}

func emitJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func runCampaignJournalVerify(cmd *cobra.Command, _ []string) error {
	cmd.SilenceUsage = true
	cwd := campaignWorkspace()

	id, err := resolveCampaignID(cwd)
	if err != nil {
		return err
	}

	result, err := campaign.VerifyCampaignJournal(cwd, id)
	if err != nil {
		return err
	}

	if campaignJournalJSON {
		if jerr := emitJSON(result); jerr != nil {
			return jerr
		}
	} else {
		fmt.Print(campaign.RenderJournalVerification(result))
	}

	// A non-zero exit is what makes this usable from a script or a CI gate.
	if !result.Healthy {
		return fmt.Errorf("journal verification found %d problem(s) in %s", len(result.Problems), id)
	}
	return nil
}

func runCampaignJournalReplay(cmd *cobra.Command, _ []string) error {
	cmd.SilenceUsage = true
	cwd := campaignWorkspace()

	id, err := resolveCampaignID(cwd)
	if err != nil {
		return err
	}

	replay, err := campaign.ReplayCampaignJournal(cwd, id, campaignJournalLimit)
	if err != nil {
		return err
	}

	if campaignJournalJSON {
		return emitJSON(replay)
	}
	fmt.Print(campaign.RenderJournalReplay(replay))
	return nil
}

func runCampaignReport(cmd *cobra.Command, _ []string) error {
	cmd.SilenceUsage = true
	cwd := campaignWorkspace()

	id, err := resolveCampaignID(cwd)
	if err != nil {
		return err
	}

	summary, err := campaign.BuildAssaultSummary(cwd, id)
	if err != nil {
		return err
	}

	path, err := campaign.WriteAssaultSummary(cwd, summary)
	if err != nil {
		return err
	}

	switch {
	case campaignReportJSON:
		if jerr := emitJSON(summary); jerr != nil {
			return jerr
		}
	case campaignReportStdout:
		fmt.Print(campaign.RenderAssaultSummaryMarkdown(summary))
	default:
		fmt.Printf("Assault summary written to %s\n", path)
		fmt.Printf("  stage runs : %d (%d passed, %d failed, %d killed)\n",
			summary.TotalRuns, summary.Passed, summary.Failed, summary.Killed)
		fmt.Printf("  targets    : %d failing of %d discovered\n", summary.TargetsBad, summary.Targets)
		if summary.Incomplete {
			fmt.Printf("  WARNING    : only %d of %d batches produced results; this is a partial run\n",
				summary.BatchesRun, summary.Batches)
		}
	}
	return nil
}
