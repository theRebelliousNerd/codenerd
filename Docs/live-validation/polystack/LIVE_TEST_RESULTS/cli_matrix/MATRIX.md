# codeNERD Live Feature Matrix - Polystack App
Generated: 2026-07-13T04:05:28.7457080-04:00
Workspace: C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack

| id | command | status | exit | duration |
|----|---------|--------|------|----------|
| 00_status | `nerd status -w C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack` | PASS | 0 | 2s |
| 00_agents | `nerd agents -w C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack` | FAIL | -1 | 64s |
| 00_jit | `nerd jit -w C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack` | PASS | 0 | 19s |
| 00_glassbox | `nerd glassbox -w C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack` | PASS | 0 | 5s |
| 00_transparency | `nerd transparency -w C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack` | PASS | 0 | 5s |
| 00_reflection | `nerd reflection -w C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack` | PASS | 0 | 1s |
| 00_embedding_stats | `nerd embedding stats -w C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack` | PASS | 0 | 1s |
| 00_sessions | `nerd sessions -w C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack` | PASS | 0 | 2s |
| 00_logs | `nerd logs -w C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack` | PASS | 0 | 1s |
| 00_mcp | `nerd mcp -w C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack` | PASS | 0 | 1s |
| 00_memory | `nerd memory -w C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack` | PASS | 0 | 1s |
| 00_logic | `nerd logic -w C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack` | PASS | 0 | 6s |
| 00_query | `nerd query permitted(A). -w C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack` | FAIL | -1 | 4s |
| 00_check_mangle | `nerd check-mangle -w C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack` | FAIL | -1 | 2s |
| 00_autopoiesis | `nerd autopoiesis -w C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack` | PASS | 0 | 1s |
| 00_tool | `nerd tool -w C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack` | FAIL | -1 | 1s |
| 00_knowledge | `nerd knowledge -w C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack` | PASS | 0 | 5s |
| 00_campaign_list | `nerd campaign list -w C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack` | PASS | 0 | 2s |
| 00_perception | `nerd perception test hello world -w C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack` | TIMEOUT | -999 | 301s |
| 00_why | `nerd why -w C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack` | PASS | 0 | 1s |
| 00_test_context | `nerd test-context -w C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack` | FAIL | -1 | 5s |
| 01_init | `nerd init -w C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack` | PASS | 0 | 19s |
| 01_northstar | `nerd northstar -w C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack` | PASS | 0 | 1s |
| 01_scan | `nerd scan -w C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack` | PASS | 0 | 2s |
| 02_create_full | `nerd create Create a polyglot monorepo under this workspace for a trivial status board: (1) backend/ Go module polystack-backend HTTP :8080 with GET /health, GET /echo?msg=, GET /status that fetches sidecars; go.mod+main.go; CORS. (2) frontend/ React+Vite+TS package.json vite.config.ts index.html src/main.tsx src/App.tsx calling backend /status and /health. (3) sidecars/rust Cargo project HTTP :8081 GET /ping JSON ok true lang rust. (4) sidecars/python main.py stdlib HTTP :8082 GET /transform?text= uppercases text JSON; requirements.txt if needed. (5) Root README.md how to run each. Write real files; keep minimal but buildable. -w C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack --timeout 25m` | PASS | 0 | 23s |
| 02_create_go | `nerd create Implement or complete backend/ Go HTTP API on :8080 with /health /echo /status as in SPEC.md. Ensure go.mod and main.go exist and compile. -w C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack --timeout 25m` | PASS | 0 | 7s |

## Phase2 retry after workspace fix 2026-07-13T04:32:02.8711811-04:00

| 02b_go_status | create | PASS | hung_or_exit | 109s |
| 02b_react | create | PASS | hung_or_exit | 197s |
| 02b_readme | create | PASS | hung_or_exit | 88s |
| 02b_rust | create | PASS | | 87s |
| 02b_python | create | PASS | | 71s |

## Phase3 2026-07-13T04:42:56.6593935-04:00

| 03_scan | scan | PASS | len=518 | 4s |
| 03_analyze | analyze | PASS | len=6423 | 65s |
| 03_explain | explain | PASS | len=4017 | 64s |
| 03_review | review | PASS | len=246 | 59s |
| 03_shadow | shadow | PASS | len=4645 | 59s |
| 03_spawn_tester | spawn | PASS | len=649 | 97s |
| 03_security | security | PASS | len=248 | 63s |
| 03_whatif | whatif | PASS | len=335 | 59s |

## Phase4 remaining surfaces + hangfix 2026-07-13T04:52:59.7852916-04:00

| 04_hang_probe_create | create | PASS+STILL_HUNG | exit_clean=False post_result~91s | 106s |
| 04_dream | dream | PASS | exit_clean=False post_result~92s | 94s |
| 04_campaign_start | campaign | PASS | exit_clean=False post_result~92s | 96s |
| 04_campaign_status | campaign | PASS | exit_clean=True post_result~-1s | 4s |
| 04_tool_list | tool | PASS | exit_clean=True post_result~-1s | 10s |
| 04_browser | browser | PASS | exit_clean=True post_result~-1s | 2s |
| 04_dom | dom | PASS | exit_clean=True post_result~-1s | 2s |
| 04_run_makefile | run | PASS | exit_clean=False post_result~91s | 114s |
| 04_fix_typo | fix | PASS | exit_clean=False post_result~92s | 118s |
| 04_refactor_comment | refactor | PASS | exit_clean=True post_result~40s | 70s |
| 04_define_agent | define-agent | FAIL | exit_clean=True post_result~-1s | 2s |
| 04_check_mangle | check-mangle | PASS | exit_clean=True post_result~-1s | 2s |
| 04_perception | perception | PASS | exit_clean=False post_result~-1s | 303s |
| 04_test_cmd | test | PASS | exit_clean=False post_result~92s | 112s |
| 04_knowledge | knowledge | FAIL | exit_clean=True post_result~-1s | 8s |
| 04_mcp_list | mcp | PASS | exit_clean=False post_result~-1s | 123s |
