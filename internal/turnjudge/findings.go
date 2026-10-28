package turnjudge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/types"
	"codenerd/internal/typesafe"
)

// Reviewer findings checked against the code they cite (TypeSafe's citation
// check pattern): for each finding with a file and a line, Jev reads the lines
// around the citation and judges whether they show the problem the finding
// describes. A refuted finding is flagged for validation (review_suspect,
// policy/jev_evidence.mg); nothing is deleted or hidden on Jev's word.

// Finding is one reviewer finding with its citation.
type Finding struct {
	File    string
	Line    int
	Message string
}

// FindingVerdict is a finding with Jev's probability that the cited code
// shows its problem.
type FindingVerdict struct {
	Finding
	Probability float64
}

// FindingOptions are the check's tuning (typesafe.batch_size,
// typesafe.finding_context_lines): questions per request, and lines either
// side of a citation, enough for the statement and its surroundings.
type FindingOptions struct {
	BatchSize    int
	ContextLines int
}

// CheckFindings judges every finding that cites a readable file and line, in
// parallel batches. Findings without a usable citation are not judged. Any
// failed batch fails the check: a partial verdict never flags a review.
func CheckFindings(ctx context.Context, a Asker, workDir string, findings []Finding, opts FindingOptions) ([]FindingVerdict, error) {
	type item struct {
		f    Finding
		code string
	}
	var items []item
	for _, f := range findings {
		if f.Line <= 0 || strings.TrimSpace(f.File) == "" || strings.TrimSpace(f.Message) == "" {
			continue
		}
		if code, ok := citedLines(workDir, f.File, f.Line, opts.ContextLines); ok {
			items = append(items, item{f, code})
		}
	}
	parts, err := inBatches(items, opts.BatchSize, func(batch []item) ([]FindingVerdict, error) {
		questions := make(map[string]typesafe.Question, len(batch))
		for i, it := range batch {
			questions[fmt.Sprintf("finding_%d", i)] = findingQuestion(it.f, it.code)
		}
		resp, err := a.Ask(typesafe.WithOperation(ctx, OpReviewChecks), findingState, questions)
		if err != nil {
			return nil, err
		}
		out := make([]FindingVerdict, 0, len(batch))
		for i, it := range batch {
			p, err := resp.Noul(fmt.Sprintf("finding_%d", i))
			if err != nil {
				return nil, err
			}
			out = append(out, FindingVerdict{Finding: it.f, Probability: p})
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	var all []FindingVerdict
	for _, p := range parts {
		all = append(all, p...)
	}
	return all, nil
}

// findingState is what every question of a request shares; each finding and
// its code ride in its own question.
var findingState = map[string]any{
	"context": "Source code that a code review cites, each excerpt with the review finding that cites it and the line it cites.",
}

// findingQuestion asks one judgment: is the described problem in the cited
// code. Lines are numbered as in the file, and the cited line is stated.
func findingQuestion(f Finding, code string) typesafe.Question {
	return typesafe.Noul{
		Instructions: map[string]any{
			"question":   "Does `code` contain the problem that `finding` describes?",
			"finding":    f.Message,
			"file":       f.File,
			"cited_line": f.Line,
			"code":       code,
		},
		Criteria: &typesafe.NoulCriteria{
			True:  "The statement at or next to `cited_line` has the problem the finding describes.",
			False: "The statement at `cited_line` does something else, or already guards against what the finding says is missing.",
		},
	}
}

// citedLines returns the lines around line in file, numbered as in the file.
func citedLines(workDir, file string, line, around int) (string, bool) {
	path := file
	if !filepath.IsAbs(path) {
		path = filepath.Join(workDir, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	lines := strings.Split(string(data), "\n")
	if line > len(lines) {
		return "", false
	}
	from := max(1, line-around)
	to := min(len(lines), line+around)
	var b strings.Builder
	for n := from; n <= to; n++ {
		fmt.Fprintf(&b, "%d: %s\n", n, lines[n-1])
	}
	return b.String(), true
}

// FindingFacts projects verdicts as review_finding_check(ReviewID, File,
// Line, Percent) for policy/jev_evidence.mg.
func FindingFacts(reviewID string, verdicts []FindingVerdict) []types.Fact {
	out := make([]types.Fact, 0, len(verdicts))
	for _, v := range verdicts {
		out = append(out, types.Fact{Predicate: "review_finding_check", Args: []any{
			types.MangleString(reviewID), types.MangleString(v.File), int64(v.Line), types.PercentFromRatio(v.Probability),
		}})
	}
	return out
}

// Refuted returns the verdicts the kernel derives as refuted for reviewID
// (review_finding_refuted, policy/jev_evidence.mg). The kernel applies the
// threshold; this only joins its rows back to the findings.
func Refuted(k Querier, reviewID string, verdicts []FindingVerdict) ([]FindingVerdict, error) {
	rows, err := k.Query("review_finding_refuted")
	if err != nil {
		return nil, err
	}
	type key struct {
		file string
		line int64
	}
	refuted := map[key]bool{}
	for _, r := range rows {
		if len(r.Args) != 3 || types.ExtractString(r.Args[0]) != reviewID {
			continue
		}
		line, _ := types.ExtractInt64(r.Args[2])
		refuted[key{types.ExtractString(r.Args[1]), line}] = true
	}
	var out []FindingVerdict
	for _, v := range verdicts {
		if refuted[key{v.File, int64(v.Line)}] {
			out = append(out, v)
		}
	}
	return out, nil
}

// Querier is the kernel read Refuted needs.
type Querier interface {
	Query(predicate string) ([]types.Fact, error)
}
