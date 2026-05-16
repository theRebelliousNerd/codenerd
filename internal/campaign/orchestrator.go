package campaign

// This file contains orchestrator core functionality.
// The orchestrator has been modularized into several files:
//
// - orchestrator_types.go: Type definitions and constants
// - orchestrator_init.go: Constructor and initialization
// - orchestrator_lifecycle.go: Campaign loading, setting, saving, and reset
// - orchestrator_execution.go: Main execution loop and heartbeat
// - orchestrator_control.go: Pause, Resume, Stop, and progress reporting
// - orchestrator_phases.go: Phase management and queries
// - orchestrator_tasks.go: Task scheduling and execution coordination
// - orchestrator_task_handlers.go: Individual task type execution handlers
// - orchestrator_task_results.go: Task result storage and context injection
// - orchestrator_failure.go: Task failure handling and retry logic
// - orchestrator_utils.go: Checkpoints, events, config, and concurrency
//
// All functionality has been moved to the appropriate modular files.
// This file remains to maintain the package structure and provide
// a central reference point for the orchestrator implementation.
