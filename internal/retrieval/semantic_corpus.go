package retrieval

import (
	"path/filepath"
	"sort"
)

// boundedCorpus picks at most limit paths from the eligible set for the
// semantic pass, round-robin across directories: the first file of every
// directory, then the second of every directory, and so on.
//
// The corpus used to be the first `limit` paths in lexicographic order. That is
// a blind spot independent of the task: every file past the cutoff -- whatever
// "z" directory the work happens to be in -- was unreachable to the semantic
// tier no matter how relevant, and one new early path displaced an included
// file from the far end. Spreading the bound over directories gives every
// package a share of it, so a large repository is sampled across its modules
// instead of down its alphabet. The choice is deterministic (sorted
// directories, sorted files), so the sample is stable between calls and the
// vector cache stays warm. paths must be sorted.
func boundedCorpus(paths []string, limit int) []string {
	if limit <= 0 || len(paths) <= limit {
		return paths
	}
	byDir := make(map[string][]string)
	var dirs []string
	for _, p := range paths {
		d := filepath.Dir(p)
		if _, seen := byDir[d]; !seen {
			dirs = append(dirs, d)
		}
		byDir[d] = append(byDir[d], p)
	}
	sort.Strings(dirs)

	picked := make([]string, 0, limit)
	for round := 0; len(picked) < limit; round++ {
		took := false
		for _, d := range dirs {
			files := byDir[d]
			if round >= len(files) {
				continue
			}
			picked = append(picked, files[round])
			took = true
			if len(picked) == limit {
				break
			}
		}
		if !took {
			break
		}
	}
	sort.Strings(picked)
	return picked
}
