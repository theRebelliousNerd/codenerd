package orient

import (
	"context"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/types"
)

// historyDelta shares the whole-history parser. Only the new commits are read.
func historyDelta(ctx context.Context, root, from, to string, shallow bool) ([]types.Fact, error) {
	cmd := gitCmd(ctx, root, "-c", "core.quotePath=false", "-c", "safe.directory=*", "--no-pager", "log",
		"--name-status", "--find-renames", "--format=format:"+commitMarker+"%H|%ct", from+".."+to)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	rows, _, parseErr := factsFromGitLog(stdout, shallow)
	if parseErr != nil {
		_ = cmd.Process.Kill()
	}
	waitErr := cmd.Wait()
	if parseErr != nil {
		return nil, parseErr
	}
	if waitErr != nil {
		return nil, fmt.Errorf("orientation history delta: %w: %s", waitErr, stderr.String())
	}
	return rows, nil
}

func mergeHistory(base, delta []types.Fact) []types.Fact {
	type deltaFile struct {
		first, last, commits int64
		hasHistory           bool
		days                 map[int64]struct{}
	}
	files := map[string]*deltaFile{}
	counts := map[monthKey]int{}
	var first, last int64
	var total int
	shallow := false
	for _, batch := range [][]types.Fact{base, delta} {
		for _, row := range batch {
			switch row.Predicate {
			case "repo_file_history":
				p := types.ExtractString(row.Args[0])
				a, b := number(row.Args[1]), number(row.Args[2])
				r := files[p]
				if r == nil {
					r = &deltaFile{first: a, last: b, days: map[int64]struct{}{}}
					files[p] = r
				}
				if !r.hasHistory || a < r.first {
					r.first = a
				}
				if !r.hasHistory || b > r.last {
					r.last = b
				}
				r.hasHistory = true
				// Counts are disjoint because the range excludes the oriented head.
				r.commits += number(row.Args[3])
			case "repo_file_day":
				p := types.ExtractString(row.Args[0])
				r := files[p]
				if r == nil {
					r = &deltaFile{days: map[int64]struct{}{}}
					files[p] = r
				}
				r.days[number(row.Args[1])] = struct{}{}
			case "repo_month":
				m, _ := time.Parse("2006-01", types.ExtractString(row.Args[1]))
				counts[monthKey{m.Year(), m.Month()}] += int(number(row.Args[2]))
			case "repo_span":
				if number(row.Args[2]) == 0 {
					continue
				}
				a, b := number(row.Args[0]), number(row.Args[1])
				if total == 0 || a < first {
					first = a
				}
				if b > last {
					last = b
				}
				total += int(number(row.Args[2]))
				shallow = shallow || types.ExtractString(row.Args[3]) == "/yes"
			}
		}
	}
	rows := spanFacts(first, last, total, shallow)
	for p, r := range files {
		rows = append(rows, types.Fact{Predicate: "repo_file_history", Args: []any{p, r.first, r.last, r.commits, int64(len(r.days))}})
		for day := range r.days {
			rows = append(rows, types.Fact{Predicate: "repo_file_day", Args: []any{p, day}})
		}
	}
	births := map[string]*fileRec{}
	for p, r := range files {
		births[p] = &fileRec{first: r.first, last: r.last}
	}
	return append(rows, monthFacts(first, last, counts, births)...)
}
