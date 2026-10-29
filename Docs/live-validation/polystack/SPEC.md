# Polystack Live Test App

Purpose: vehicle for stress-testing codeNERD. App domain is intentionally trivial.

## Components
1. **backend/** (Go): HTTP API on :8080 — health, echo, and aggregate status from sidecars
2. **frontend/** (React+Vite): single page that calls the Go API and shows status
3. **sidecars/rust/**: small HTTP service on :8081 providing a ping + cpu tick
4. **sidecars/python/**: small HTTP service on :8082 providing a text transform endpoint

## Success for codeNERD (not the app)
- codeNERD creates and edits files via create/run/spawn/fix/fix
- scan, perception, review, analyze, explain, shadow, dream, campaign all invoked
- No panic / debug_program_ERROR.mg for runs
