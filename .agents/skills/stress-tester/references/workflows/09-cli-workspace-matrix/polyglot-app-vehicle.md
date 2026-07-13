# Workflow: Polyglot App Vehicle (Go + React + Rust + Python)

## Purpose

Drive **codeNERD live** by building a multi-language app. The app domain is intentionally trivial; the product under test is the agent CLI.

## Target Layout

```text
<workspace>/
  backend/          # Go HTTP :8080  /health /echo /status
  frontend/         # React+Vite+TS
  sidecars/rust/    # Cargo HTTP :8081 /ping
  sidecars/python/  # stdlib HTTP :8082 /transform
  README.md
  SPEC.md
```

Reference live workspace used 2026-07:
`.nerd/live_feature_matrix/polystack`

## Procedure (serial)

```powershell
$APP = "C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack"
# Ensure engine has working LLM (api+xai or xai-oauth after nerd auth grok)

nerd init -w $APP
nerd scan -w $APP

nerd create "Implement backend/ Go HTTP :8080 with /health /echo /status (fetch sidecars)" -w $APP --timeout 25m
nerd create "Implement frontend/ React+Vite+TS calling Go API" -w $APP --timeout 25m
nerd create "Implement sidecars/rust Cargo GET /ping on :8081" -w $APP --timeout 25m
nerd create "Implement sidecars/python stdlib /transform on :8082" -w $APP --timeout 25m
nerd create "Write root README with run instructions" -w $APP --timeout 15m

nerd spawn tester "Add Go tests for /health" -w $APP --timeout 15m
nerd analyze backend -w $APP
nerd explain backend/main.go -w $APP
nerd review backend -w $APP
nerd shadow "delete backend/main.go" -w $APP
nerd security backend -w $APP
nerd whatif "rename backend to api" -w $APP
```

## External Build Gates (prove code is real)

```powershell
cd $APP/backend; go build .; go test ./...
cd $APP/sidecars/rust; cargo check
python -m py_compile $APP/sidecars/python/main.py
# optional: npm run build in frontend/
```

## Live HTTP Gate

Start sidecars + backend; assert:

- `GET :8080/health` → `ok`
- `GET :8080/echo?msg=hi` → `hi`
- `GET :8081/ping` → JSON ok/lang rust
- `GET :8082/transform?text=hi` → uppercased JSON
- `GET :8080/status` → aggregates sidecars (tolerate sidecar down)

## Surfaces Exercised

create, spawn, scan, init, analyze, explain, review, shadow, security, whatif, VirtualStore write_file, safety fallback, workspace isolation, multi-language scaffolding.

## Pass Criteria

- [ ] All four stacks exist as real files under `-w`
- [ ] Go build + test green
- [ ] Rust cargo check green
- [ ] Python compiles
- [ ] No monorepo pollution from write_file
- [ ] At least one spawn path mutates code (e.g. main_test.go)
