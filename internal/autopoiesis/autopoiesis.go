// Package autopoiesis implements self-modification capabilities for codeNERD.
// Autopoiesis (from Greek: self-creation) enables the system to:
// 1. Detect when tasks require campaign orchestration (complex multi-phase work)
// 2. Generate new tools when existing capabilities are insufficient
// 3. Create persistent agents when ongoing monitoring/learning is needed
package autopoiesis

import (
	"time"
)

// =============================================================================
// MODULARIZATION NOTE
// =============================================================================
// This file previously contained 2295 lines. It has been modularized into:
//
// - autopoiesis_types.go        - Core types, config, interfaces (~170 lines)
// - autopoiesis_orchestrator.go - Main orchestrator initialization (~180 lines)
// - autopoiesis_kernel.go       - Kernel integration, fact assertion (~380 lines)
// - autopoiesis_delegation.go   - Kernel-mediated delegations (~180 lines)
// - autopoiesis_agents.go       - Agent creation and management (~210 lines)
// - autopoiesis_analysis.go     - Analysis and action execution (~220 lines)
// - autopoiesis_tools.go        - Tool generation wrappers (~190 lines)
// - autopoiesis_feedback.go     - Feedback, learning, traces (~380 lines)
// - autopoiesis_profiles.go     - Quality profiles (~360 lines)
// - autopoiesis_helpers.go      - Helper functions (~140 lines)
//
// All files remain in package autopoiesis and work together seamlessly.
// =============================================================================

// Re-export commonly used time for convenience (used in multiple files)
var _ = time.Now
