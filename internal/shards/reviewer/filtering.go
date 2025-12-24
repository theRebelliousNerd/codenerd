// Package reviewer implements the Reviewer ShardAgent per §7.0 Sharding.
// This file contains Mangle-based filtering and persistence helpers.
package reviewer

import (
	"fmt"
	"strings"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/store"
)

// =============================================================================
// PERSISTENCE & FILTERING HELPERS
// =============================================================================

// assertFileFacts asserts file topology facts to the kernel (e.g., for test file detection).
func (r *ReviewerShard) assertFileFacts(filePath string) {
	if r.kernel == nil {
		return
	}

	isTest := strings.HasSuffix(filePath, "_test.go") || strings.Contains(filePath, "test/")
	testAtom := core.MangleAtom("/false")
	if isTest {
		testAtom = core.MangleAtom("/true")
	}

	// file_topology(Path, Hash, Language, LastModified, IsTestFile)
	// Using placeholders for Hash/Time as they aren't critical for suppression rules yet
	fact := core.Fact{
		Predicate: "file_topology",
		Args:      []interface{}{filePath, "unknown_hash", r.detectLanguage(filePath), "unknown_time", testAtom},
	}
	_ = r.kernel.Assert(fact)
}

// filterFindingsWithMangle uses Mangle rules to determine which findings should be suppressed.
// Instead of reconstructing findings from query results (which loses metadata like line numbers),
// we query for suppression decisions and filter the original findings list.
func (r *ReviewerShard) filterFindingsWithMangle(findings []ReviewFinding) ([]ReviewFinding, error) {
	if r.kernel == nil {
		return findings, nil
	}

	// 1. Assert raw findings to kernel for rule evaluation
	for _, f := range findings {
		fact := core.Fact{
			Predicate: "raw_finding",
			Args:      []interface{}{f.File, f.Line, f.Severity, f.Category, f.RuleID, f.Message},
		}
		_ = r.kernel.Assert(fact)
	}

	// 2. Query for suppressed findings (File, Line, RuleID, Reason)
	suppressedResults, err := r.kernel.Query("suppressed_finding")
	if err != nil {
		logging.ReviewerDebug("Mangle suppression query failed: %v", err)
		return findings, nil // Return original findings on error
	}

	// 3. Build suppression index: key = "file:line:ruleID"
	suppressed := make(map[string]string) // key -> reason
	for _, res := range suppressedResults {
		if len(res.Args) < 3 {
			continue
		}
		file, _ := res.Args[0].(string)
		line := toStartInt(res.Args[1])
		ruleID, _ := res.Args[2].(string)
		reason := ""
		if len(res.Args) >= 4 {
			reason, _ = res.Args[3].(string)
		}
		key := fmt.Sprintf("%s:%d:%s", file, line, ruleID)
		suppressed[key] = reason
	}

	// 4. Filter original findings, keeping all metadata intact
	active := make([]ReviewFinding, 0, len(findings))
	for _, f := range findings {
		key := fmt.Sprintf("%s:%d:%s", f.File, f.Line, f.RuleID)
		if reason, isSuppressed := suppressed[key]; isSuppressed {
			logging.ReviewerDebug("Suppressed finding [%s] at %s:%d - reason: %s",
				f.RuleID, f.File, f.Line, reason)
			continue
		}
		active = append(active, f)
	}

	if len(active) < len(findings) {
		logging.ReviewerDebug("Mangle filtering: %d -> %d findings (%d suppressed)",
			len(findings), len(active), len(findings)-len(active))
	}

	return active, nil
}

// persistFindingsToStore stores findings in the LocalStore.
// Note: This is named differently from persistFindings in persistence.go to avoid conflict.
func (r *ReviewerShard) persistFindingsToStore(findings []ReviewFinding) {
	if r.virtualStore == nil || r.virtualStore.GetLocalDB() == nil {
		return
	}
	localDB := r.virtualStore.GetLocalDB()
	root := r.reviewerConfig.WorkingDir

	for _, f := range findings {
		// Use the DTO defined in store package
		sf := store.StoredReviewFinding{
			FilePath:    f.File,
			Line:        f.Line,
			Severity:    f.Severity,
			Category:    f.Category,
			RuleID:      f.RuleID,
			Message:     f.Message,
			ProjectRoot: root,
		}
		_ = localDB.StoreReviewFinding(sf)
	}
}
