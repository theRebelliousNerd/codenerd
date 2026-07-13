# Polystack verification report

**App:** `C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack`  
**When:** 2026-07-13  
**Verdict:** **REAL AND RUNNABLE** — all stacks PASS

Did not run `nerd.exe` (CLI owned by another agent). Built/ran services only.

---

## Summary

| Stack | Check | Result |
|-------|--------|--------|
| Go backend | `go build` + `go test` | **PASS** |
| Rust sidecar | `cargo check` + `cargo build --release` | **PASS** |
| Python sidecar | `python -m py_compile main.py` | **PASS** |
| Frontend | `npm run build` (node v25.5.0) | **PASS** |
| Live smoke | start services + HTTP probes + kill | **PASS** (5/5) |

**Failures:** none

---

## 1. Go backend (`backend/`)

Commands:

```text
go build -o polystack-backend.exe .
go test -v ./...
```

| Step | Exit | Notes |
|------|------|--------|
| build | 0 | Produces `backend/polystack-backend.exe` |
| test | 0 | `ok polystack-backend 0.183s` |

Tests:

- `TestHealthHandler` (GET ok / POST+PUT method not allowed) — PASS  
- `TestHealthHandler_WithCORS_GET` — PASS  

Artifact log: `go_log.txt`, `go_result.txt`

---

## 2. Rust sidecar (`sidecars/rust/`)

Commands:

```text
cargo check
cargo build --release
```

| Step | Exit | Notes |
|------|------|--------|
| cargo check | 0 | Finished dev profile (cached) ~0.12s |
| cargo build --release | 0 | Finished release ~0.06s |

Binary:

- `sidecars/rust/target/release/rust-sidecar.exe`

Artifact log: `rust_log.txt`, `rust_result.txt`

---

## 3. Python sidecar (`sidecars/python/`)

Command:

```text
python -m py_compile sidecars/python/main.py
```

| Step | Exit | Notes |
|------|------|--------|
| py_compile | 0 | Syntax OK |
| runtime | — | stdlib `ThreadingHTTPServer` only (no third-party required) |
| Python | 3.14.2 | |

Note: `requirements.txt` documents stdlib-only; compile does not need fastapi (import check incidental).

Artifact log: `python_log.txt`, `python_result.txt`

---

## 4. Frontend (`frontend/`)

| Step | Exit | Notes |
|------|------|--------|
| package.json | present | `polystack-frontend@0.0.1` |
| node | v25.5.0 | available |
| `npm run build` | 0 | `tsc -b && vite build`; 31 modules; ~1.46s |

Dist:

- `frontend/dist/index.html`
- `frontend/dist/assets/index-1IKiKrBJ.css` (1.18 kB)
- `frontend/dist/assets/index-D9pSghkG.js` (143.96 kB)

Artifact log: `frontend_log.txt`, `frontend_result.txt`

---

## 5. Live smoke (start → curl → kill)

Started (background, then killed after probes):

| Service | How | Port | Listen confirm |
|---------|-----|------|----------------|
| Python | `python .../sidecars/python/main.py` | 8082 | LISTEN |
| Rust | `target/release/rust-sidecar.exe` | 8081 | LISTEN |
| Go | `backend/polystack-backend.exe` | 8080 | LISTEN |

### Endpoint results

| Endpoint | Result | Body (abbrev) |
|----------|--------|----------------|
| `GET :8080/health` | **PASS** 200 | `ok` |
| `GET :8080/echo?msg=hi` | **PASS** 200 | `hi` |
| `GET :8080/status` | **PASS** 200 | health ok; both sidecars `ok:true` |
| `GET :8081/ping` | **PASS** 200 | `{"ok":true,"lang":"rust"}` |
| `GET :8082/transform?text=hi` | **PASS** 200 | `{"text": "HI", "lang": "python"}` |

`/status` aggregation (Go → sidecars):

```json
{
  "health": "ok",
  "sidecars": {
    "python_transform": {
      "body": "{\"text\": \"HI\", \"lang\": \"python\"}",
      "error": "",
      "ok": true,
      "status": 200,
      "url": "http://127.0.0.1:8082/transform?text=hi"
    },
    "rust_ping": {
      "body": "{\"ok\":true,\"lang\":\"rust\"}",
      "error": "",
      "ok": true,
      "status": 200,
      "url": "http://127.0.0.1:8081/ping"
    }
  }
}
```

Processes terminated after smoke. Artifact: `smoke.txt`

---

## Port map (from README)

| Service | Port | Role |
|---------|------|------|
| Go backend | 8080 | `/health`, `/echo`, `/status` |
| Rust sidecar | 8081 | `/ping` |
| Python sidecar | 8082 | `/transform` |

---

## Overall

- **App is real** — multi-language tree with source, tests, lockfiles, and build artifacts.  
- **App is runnable** — build/test/compile all green; live HTTP stack responds end-to-end including cross-service `/status`.  
- **No failures** observed in this verification pass.
