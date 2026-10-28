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
	type part struct {
		verdicts []FindingVerdict
		err      error
	}
	var parts []chan part
	size := max(opts.BatchSize, 1)
	for start := 0; start < len(items); start += size {
		batch := items[start:min(start+size, len(items))]
		ch := make(chan part, 1)
		parts = append(parts, ch)
		go func() {
			questions := make(map[string]typesafe.Question, len(batch))
			for i, it := range batch {
				questions[fmt.Sprintf("finding_%d", i)] = typesafe.Noul{
					Instructions: map[string]any{
						"question": "Does `code` show the problem that `finding` describes?",
						"finding":  it.f.Message,
						"file":     it.f.File,
						"code":     it.code,
					},
					Criteria: &typesafe.NoulCriteria{
						True:  "The code shown contains the problem the finding describes, at or near the cited line.",
						False: "The code shown does not contain it: the cited lines are about something else, or already handle what the finding says is missing.",
					},
				}
			}
			resp, err := a.Ask(typesafe.WithOperation(ctx, OpReviewChecks), map[string]any{"task": "Check reviewer findings against the code they cite."}, questions)
			if err != nil {
				ch <- part{err: err}
				return
			}
			out := make([]FindingVerdict, 0, len(batch))
			for i, it := range batch {
				p, err := resp.Noul(fmt.Sprintf("finding_%d", i))
				if err != nil {
					ch <- part{err: err}
					return
				}
				out = append(out, FindingVerdict{Finding: it.f, Probability: p})
			}
			ch <- part{verdicts: out}
		}()
	}
	var all []FindingVerdict
	for _, ch := range parts {
		p := <-ch
		if p.err != nil {
			return nil, p.err
		}
		all = append(all, p.verdicts...)
	}
	return all, nil
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
