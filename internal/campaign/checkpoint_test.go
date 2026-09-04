package campaign

// Stale DeriveDone tests removed 2026-09-04: DeriveDone was never defined in
// package campaign, so this file broke `go vet ./internal/campaign` with
// "undefined: DeriveDone". The no-write and failed-build done gates it meant
// to pin are covered live by internal/session/turn_done_test.go
// (TestTurnDone_NoWriteCannotDeriveDone,
// TestTurnDone_FailedBuildCannotDeriveDone) against the real
// turn_done/2 policy. This stub keeps the filename reserved without
// contributing duplicate or placeholder tests.