package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"codenerd/internal/broker"
	"codenerd/internal/prompt"

	"github.com/spf13/cobra"
)

// The broker has recorded a receipt for every inference call since it landed,
// the compiler has recorded every atom selection, and until now nothing read
// either back. Write-only instrumentation is indistinguishable from no
// instrumentation right up to the moment somebody needs the number.
//
// Everything here reads the workspace logs rather than process state, because a
// `nerd meter` invocation is a different process from the agent that did the
// spending, and the questions worth asking span sessions anyway.

var (
	meterWorkspace string
	meterJSON      bool
	meterTop       int
)

func meterWorkspaceRoot() (string, error) {
	if meterWorkspace != "" {
		return meterWorkspace, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve workspace: %w", err)
	}
	return cwd, nil
}

func loadReceipts() ([]broker.Receipt, error) {
	root, err := meterWorkspaceRoot()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(root, ".nerd", broker.DefaultReceiptLogName)
	receipts, truncated, err := broker.ReadReceiptLog(path)
	if err != nil {
		return nil, fmt.Errorf("read receipt log %s: %w", path, err)
	}
	if len(receipts) == 0 {
		return nil, fmt.Errorf("no receipts at %s — run an agent session in this workspace first", path)
	}
	if truncated > 0 && !meterJSON {
		fmt.Fprintf(os.Stderr, "note: %d log generation(s) ended on a truncated line; "+
			"records after the cut are not included\n", truncated)
	}
	return receipts, nil
}

// ---------------------------------------------------------------------------
// nerd meter
// ---------------------------------------------------------------------------

var meterCmd = &cobra.Command{
	Use:   "meter",
	Short: "Inspect what inference actually cost, and what the meter believes",
	Long: `Reads the receipt log written by the inference broker and reports spend by
purpose, per-model estimate accuracy, and admission refusals.

Every inference call in codeNERD passes through one metered boundary, which
writes a receipt to <workspace>/.nerd/meter/receipts.jsonl. A receipt records
what admission believed before the request went out, what the provider actually
billed, and the gap between them.

Estimate error is the number that says whether the meter can be trusted. On
Anthropic it is measured against an exact count_tokens call, so a near-zero
value proves the request measurement is sound. Elsewhere it is estimate versus
bill, which is still the only independent check there is.

Examples:
  nerd meter
  nerd meter --json
  nerd meter epochs
  nerd meter atoms --top 30`,
	RunE: func(cmd *cobra.Command, args []string) error {
		receipts, err := loadReceipts()
		if err != nil {
			return err
		}
		summary := summarizeReceipts(receipts)
		if meterJSON {
			return emitJSON(cmd.OutOrStdout(), summary)
		}
		printMeterSummary(cmd.OutOrStdout(), summary)
		return nil
	},
}

// PurposeSpend aggregates one purpose's receipts.
type PurposeSpend struct {
	Purpose  string  `json:"purpose"`
	Calls    int     `json:"calls"`
	Input    int64   `json:"input_tokens"`
	Output   int64   `json:"output_tokens"`
	Cached   int64   `json:"cached_tokens"`
	Refusals int     `json:"refusals"`
	Errors   int     `json:"errors"`
	Share    float64 `json:"share_of_total_tokens"`
}

// ModelAccuracy is one model's estimate-versus-bill record.
type ModelAccuracy struct {
	Model string `json:"model"`
	Calls int    `json:"calls"`
	// MeanAbsErrorPct is the headline rather than net bias. An estimator wrong
	// by 30% on every call, half over and half under, has a net bias near zero
	// and is not remotely trustworthy.
	MeanAbsErrorPct float64 `json:"mean_abs_error_pct"`
	MeanBiasPct     float64 `json:"mean_bias_pct"`
	WorstErrorPct   float64 `json:"worst_error_pct"`
	// ExactCounts is how many receipts were admitted on a provider-exact count
	// rather than an estimate.
	ExactCounts int `json:"exact_counts"`
}

// MeterSummary is the `nerd meter` readout.
type MeterSummary struct {
	Receipts     int             `json:"receipts"`
	Admitted     int             `json:"admitted"`
	Refused      int             `json:"refused"`
	Errored      int             `json:"errored"`
	InputTokens  int64           `json:"input_tokens"`
	OutputTokens int64           `json:"output_tokens"`
	CachedTokens int64           `json:"cached_tokens"`
	ByPurpose    []PurposeSpend  `json:"by_purpose"`
	ByModel      []ModelAccuracy `json:"by_model"`
	// RefusalsByCode says why turns did not happen. A refusal is the receipt an
	// operator must not have to go looking for: it means a turn did not happen,
	// and the user saw something else instead.
	RefusalsByCode map[string]int `json:"refusals_by_code,omitempty"`
	// Unattributed is spend that reached a provider with no purpose on the
	// context. It is a wiring bug, reported as a number rather than hidden.
	UnattributedTokens int64 `json:"unattributed_tokens"`
}

func summarizeReceipts(receipts []broker.Receipt) MeterSummary {
	s := MeterSummary{Receipts: len(receipts), RefusalsByCode: map[string]int{}}

	purposes := map[string]*PurposeSpend{}
	type modelAcc struct {
		calls      int
		sumAbs     float64
		sumBias    float64
		worst      float64
		exact      int
		errSamples int
	}
	models := map[string]*modelAcc{}

	for i := range receipts {
		r := &receipts[i]

		ps, ok := purposes[string(r.Purpose)]
		if !ok {
			ps = &PurposeSpend{Purpose: string(r.Purpose)}
			purposes[string(r.Purpose)] = ps
		}

		if !r.Decision.Allowed {
			s.Refused++
			ps.Refusals++
			s.RefusalsByCode[string(r.Decision.Code)]++
			// A refusal costs no tokens and must not be counted as spend;
			// pretending otherwise inflates the very number the meter exists
			// to make trustworthy.
			continue
		}

		s.Admitted++
		ps.Calls++
		ps.Input += r.Actual.InputTokens
		ps.Output += r.Actual.OutputTokens
		ps.Cached += r.Actual.CachedTokens
		s.InputTokens += r.Actual.InputTokens
		s.OutputTokens += r.Actual.OutputTokens
		s.CachedTokens += r.Actual.CachedTokens

		if r.Err != "" {
			s.Errored++
			ps.Errors++
		}

		ma, ok := models[r.Model]
		if !ok {
			ma = &modelAcc{}
			models[r.Model] = ma
		}
		ma.calls++
		if r.Estimated.Confidence == broker.ConfidenceExact {
			ma.exact++
		}
		if r.Actual.InputTokens > 0 && r.Estimated.Tokens > 0 {
			e := r.EstimateErrorPct
			ma.errSamples++
			ma.sumBias += e
			if e < 0 {
				e = -e
			}
			ma.sumAbs += e
			if e > ma.worst {
				ma.worst = e
			}
		}
	}

	totalTokens := s.InputTokens + s.OutputTokens
	for _, ps := range purposes {
		if totalTokens > 0 {
			ps.Share = float64(ps.Input+ps.Output) / float64(totalTokens) * 100
		}
		if ps.Purpose == string(broker.PurposeUnattributed) {
			s.UnattributedTokens = ps.Input + ps.Output
		}
		s.ByPurpose = append(s.ByPurpose, *ps)
	}
	sort.Slice(s.ByPurpose, func(i, j int) bool {
		li, lj := s.ByPurpose[i].Input+s.ByPurpose[i].Output, s.ByPurpose[j].Input+s.ByPurpose[j].Output
		if li != lj {
			return li > lj
		}
		return s.ByPurpose[i].Purpose < s.ByPurpose[j].Purpose
	})

	for model, ma := range models {
		acc := ModelAccuracy{Model: model, Calls: ma.calls, ExactCounts: ma.exact, WorstErrorPct: ma.worst}
		if ma.errSamples > 0 {
			acc.MeanAbsErrorPct = ma.sumAbs / float64(ma.errSamples)
			acc.MeanBiasPct = ma.sumBias / float64(ma.errSamples)
		}
		s.ByModel = append(s.ByModel, acc)
	}
	sort.Slice(s.ByModel, func(i, j int) bool {
		if s.ByModel[i].Calls != s.ByModel[j].Calls {
			return s.ByModel[i].Calls > s.ByModel[j].Calls
		}
		return s.ByModel[i].Model < s.ByModel[j].Model
	})

	return s
}

func printMeterSummary(w io.Writer, s MeterSummary) {
	fmt.Fprintf(w, "\nInference meter — %d receipts (%d admitted, %d refused, %d errored)\n",
		s.Receipts, s.Admitted, s.Refused, s.Errored)
	fmt.Fprintf(w, "Billed: %s input, %s output, %s of the input read from cache\n\n",
		humanTokens(s.InputTokens), humanTokens(s.OutputTokens), humanTokens(s.CachedTokens))

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "PURPOSE\tCALLS\tINPUT\tOUTPUT\tCACHED\tSHARE\tREFUSED\tERRORS")
	for _, p := range s.ByPurpose {
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\t%.1f%%\t%d\t%d\n",
			p.Purpose, p.Calls, humanTokens(p.Input), humanTokens(p.Output),
			humanTokens(p.Cached), p.Share, p.Refusals, p.Errors)
	}
	_ = tw.Flush()

	fmt.Fprintln(w, "\nEstimate accuracy — mean absolute error, not net bias:")
	tw = tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "MODEL\tCALLS\tEXACT\tMEAN ABS ERR\tMEAN BIAS\tWORST")
	for _, m := range s.ByModel {
		fmt.Fprintf(tw, "%s\t%d\t%d\t%.1f%%\t%+.1f%%\t%.1f%%\n",
			m.Model, m.Calls, m.ExactCounts, m.MeanAbsErrorPct, m.MeanBiasPct, m.WorstErrorPct)
	}
	_ = tw.Flush()

	if len(s.RefusalsByCode) > 0 {
		fmt.Fprintln(w, "\nRefusals — each one is a turn that did not happen:")
		codes := make([]string, 0, len(s.RefusalsByCode))
		for c := range s.RefusalsByCode {
			codes = append(codes, c)
		}
		sort.Strings(codes)
		for _, c := range codes {
			fmt.Fprintf(w, "  %-22s %d\n", c, s.RefusalsByCode[c])
		}
	}

	if s.UnattributedTokens > 0 {
		fmt.Fprintf(w, "\n%s of spend reached a provider with no purpose tagged on the context.\n"+
			"That is a wiring gap, not a category of work: some subsystem is spending off the books.\n",
			humanTokens(s.UnattributedTokens))
	}
	fmt.Fprintln(w)
}

// ---------------------------------------------------------------------------
// nerd meter epochs
// ---------------------------------------------------------------------------

var meterEpochsCmd = &cobra.Command{
	Use:   "epochs",
	Short: "Calls-per-epoch distribution (Gate A / Q1)",
	Long: `Segments the receipt log into epochs and reports how many calls each one
served.

An epoch is a maximal run of calls, inside one session, that shared a cacheable
prefix — the tool definitions and system prompt. Every prefix-cache strategy is
an amortization bet: you pay a premium to write a prefix into the provider's
cache and earn it back on each later call that reads it. Break-even is
(write-read)/(1-read), a call count independent of prefix size, derived per
provider rather than assumed.

If the median epoch is one call long, that bet cannot be won, and any rebuild
controller built on it is dead weight however elegant it is.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		receipts, err := loadReceipts()
		if err != nil {
			return err
		}
		hist := broker.Histogram(broker.Segment(receipts))
		if meterJSON {
			return emitJSON(cmd.OutOrStdout(), hist)
		}
		printEpochHistogram(cmd.OutOrStdout(), hist)
		return nil
	},
}

func printEpochHistogram(w io.Writer, h broker.EpochHistogram) {
	if h.Epochs == 0 {
		fmt.Fprintln(w, "\nNo admitted calls in the receipt log; nothing to segment.")
		return
	}

	fmt.Fprintf(w, "\nCalls per epoch — %d epochs over %d admitted calls\n", h.Epochs, h.Calls)
	fmt.Fprintf(w, "min %d  p50 %d  p90 %d  max %d  mean %.2f\n\n", h.Min, h.P50, h.P90, h.Max, h.Mean)

	widest := 0
	for _, b := range h.Buckets {
		if b.Count > widest {
			widest = b.Count
		}
	}
	for _, b := range h.Buckets {
		bar := ""
		if widest > 0 {
			bar = strings.Repeat("█", b.Count*40/widest)
		}
		fmt.Fprintf(w, "  %-6s %5d  %5.1f%%  %s\n", b.Label, b.Count, b.Pct, bar)
	}

	fmt.Fprintf(w, "\n%d of %d epochs served exactly one call (%.1f%%).\n",
		h.Singletons, h.Epochs, h.SingletonPct)
	if h.NoPrefixCalls > 0 {
		fmt.Fprintf(w, "%d calls (%.1f%%) had no cacheable prefix at all — no system prompt and no tools.\n",
			h.NoPrefixCalls, h.NoPrefixPct)
	}

	fmt.Fprintln(w, "\nAmortization, per provider:")
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "PROVIDER\tEPOCHS\tCALLS\tBREAK-EVEN\tPAYING\tEXPIRED")
	for _, pv := range h.ByProvider {
		be := fmt.Sprintf("%.2f calls", pv.BreakEven)
		if pv.BreakEven > 1e9 {
			be = "never (no discount)"
		}
		fmt.Fprintf(tw, "%s\t%d\t%d\t%s\t%d (%.1f%%)\t%d\n",
			pv.Provider, pv.Epochs, pv.Calls, be, pv.PayingEpochs, pv.PayingPct, pv.ExpiredEpochs)
	}
	_ = tw.Flush()
	fmt.Fprintln(w, "\nEXPIRED counts epochs whose span outran the provider's cache TTL: long enough\n"+
		"to look profitable, evicted between calls, so they were not.")
	fmt.Fprintln(w)
}

// ---------------------------------------------------------------------------
// nerd meter atoms
// ---------------------------------------------------------------------------

var meterAtomsCmd = &cobra.Command{
	Use:   "atoms",
	Short: "Prompt-atom co-use analysis (Gate A / Q2)",
	Long: `Reports which prompt atoms are selected together in compilations belonging to
turns that succeeded.

The statistic is lift — the ratio of observed co-occurrence to what independent
selection would produce — not raw co-occurrence. A skeleton atom present in
every prompt co-occurs with everything more than any real pair does, so a
count-ranked list puts the least informative atom at the top of every row.

This exists to test an assumption rather than to confirm one: the lane taxonomy
of architecture / implementation / verification is how human teams are
organized, which is a fact about org charts and not evidence about how
information clusters. CATEGORY ALIGNMENT is the answer. High means atoms are
already used along the taxonomy. Low means they are not, and copying an org
chart into an information architecture would describe the data even worse.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := meterWorkspaceRoot()
		if err != nil {
			return err
		}
		path := filepath.Join(root, ".nerd", prompt.DefaultSelectionLogName)

		rec, truncated, err := prompt.LoadSelections(path)
		if err != nil {
			return fmt.Errorf("read selection log %s: %w", path, err)
		}
		if truncated > 0 && !meterJSON {
			fmt.Fprintf(os.Stderr, "note: %d log generation(s) ended on a truncated line\n", truncated)
		}

		report := rec.Report(prompt.DefaultCoUseParams(), rec.Categories())
		if report.SuccessSelections == 0 {
			return fmt.Errorf("no settled selections at %s — run agent turns in this workspace first", path)
		}
		if meterJSON {
			return emitJSON(cmd.OutOrStdout(), report)
		}
		printCoUseReport(cmd.OutOrStdout(), report, meterTop)
		return nil
	},
}

func printCoUseReport(w io.Writer, r prompt.CoUseReport, top int) {
	if top <= 0 {
		top = 20
	}

	fmt.Fprintf(w, "\nAtom co-use — %d successful selections, %d failed, %d distinct atoms\n",
		r.SuccessSelections, r.FailureSelections, r.DistinctAtoms)
	fmt.Fprintf(w, "thresholds: support >= %d, lift >= %.2f, ubiquity >= %.0f%%\n",
		r.Params.MinSupport, r.Params.MinLift, r.Params.UbiquityThreshold*100)

	if r.PendingTurns > 0 || r.DroppedSelections > 0 || r.UnattributedSelections > 0 || r.Truncated {
		fmt.Fprintf(w, "gaps: %d turns pending an outcome, %d selections dropped, %d untagged",
			r.PendingTurns, r.DroppedSelections, r.UnattributedSelections)
		if r.Truncated {
			fmt.Fprint(w, ", pair matrix truncated")
		}
		fmt.Fprintln(w)
	}

	if len(r.Ubiquitous) > 0 {
		fmt.Fprintf(w, "\n%d skeleton atoms appear in nearly every prompt and carry no clustering\n"+
			"information; they are excluded from the graph:\n  %s\n",
			len(r.Ubiquitous), strings.Join(truncateList(r.Ubiquitous, 8), ", "))
	}

	fmt.Fprintf(w, "\nStrongest associations (top %d of %d):\n", min(top, len(r.Pairs)), len(r.Pairs))
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "LIFT\tJACCARD\tJOINT\tSAME CAT\tA\tB")
	for i, p := range r.Pairs {
		if i >= top {
			break
		}
		same := "no"
		if p.SameCategory {
			same = "yes"
		}
		fmt.Fprintf(tw, "%.2f\t%.2f\t%d\t%s\t%s\t%s\n", p.Lift, p.Jaccard, p.Joint, same, p.A, p.B)
	}
	_ = tw.Flush()

	fmt.Fprintf(w, "\nClusters (%d), largest first:\n", len(r.Clusters))
	for i, c := range r.Clusters {
		if i >= top {
			fmt.Fprintf(w, "  ... and %d more\n", len(r.Clusters)-top)
			break
		}
		fmt.Fprintf(w, "  [%d atoms] %s purity %.0f%%: %s\n",
			len(c.Atoms), c.ModalCategory, c.Purity*100, strings.Join(truncateList(c.Atoms, 6), ", "))
	}

	fmt.Fprintf(w, "\nCATEGORY ALIGNMENT: %.1f%%\n", r.CategoryAlignment*100)
	fmt.Fprintf(w, "%d of %d reported pairs join two different categories.\n",
		r.CrossCategoryPairs, len(r.Pairs))
	fmt.Fprintf(w, "%d atoms clustered, %d well-sampled atoms formed no association at all.\n",
		r.ClusteredAtoms, r.IsolatedAtoms)
	fmt.Fprintln(w, "\nA low alignment is the interesting result: it says information is used\n"+
		"across the taxonomy rather than along it, and that a lane taxonomy copied\n"+
		"from an org chart would fit the data even worse than the categories do.")
	fmt.Fprintln(w)
}

func truncateList(items []string, n int) []string {
	if len(items) <= n {
		return items
	}
	out := make([]string, 0, n+1)
	out = append(out, items[:n]...)
	return append(out, fmt.Sprintf("… +%d more", len(items)-n))
}

func humanTokens(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func init() {
	meterCmd.PersistentFlags().StringVar(&meterWorkspace, "workspace", "",
		"Workspace root (defaults to the current directory)")
	meterCmd.PersistentFlags().BoolVar(&meterJSON, "json", false, "Emit JSON instead of a table")
	meterAtomsCmd.Flags().IntVar(&meterTop, "top", 20, "How many pairs and clusters to show")

	meterCmd.AddCommand(meterEpochsCmd, meterAtomsCmd)
}
