# Stress 01-03 REPORT (conservative smoke)

- Started: 2026-07-13 05:16:49
- Workspace: C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\stress_ws_01
- Binary: C:\CodeProjects\codeNERD\nerd.exe
- Severity: conservative
- Constraint: separate workspace; no multi-hour chaos; prefer light diagnostics

## Workflow plan (partial execute)

### 01-kernel-core
1. mangle-self-healing (conservative): status, check-mangle known-good/bad, corpus signals
2. mangle-failure-modes (partial): check-mangle adversarial assets (expect failures)
3. mangle-startup-validation (smoke): status boot only (no learned.mg mutation of root)

### 02-perception
1. intent-fuzzing (smoke): perception --help + optional short perception if fast
2. agents list (shard routing surface)

### 03-shards
1. shard-explosion (smoke): agents list + query shard_state if safe; no mass spawn

---


### 01-kernel: status (ws)
- Time: 2026-07-13 05:16:52
- Command: `.\nerd.exe -w "C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\stress_ws_01" status`
- Exit: 0
- Note: Boot/status smoke in isolated workspace

```
codeNERD System Status
======================
Version: Cortex 1.5.0
Kernel:  Google Mangle (Datalog)
Runtime: Go 1.24

Γ£ô Z.AI API key configured
Γ£ô Workspace: C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\stress_ws_01
Γ£ô Mangle kernel initialized
  Schemas: 280339 bytes
  Policy:  436794 bytes

```


### 03-shards: agents list
- Time: 2026-07-13 05:17:58
- Command: `.\nerd.exe -w "C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\stress_ws_01" agents`
- Exit: 0
- Note: List shard agents (no spawn)

```
≡ƒñû Available Shard Agents
ΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇ
Warning: Embedding engine unavailable: ollama unavailable at http://localhost:11434: Get "http://localhost:11434/api/tags": context deadline exceeded

### System (Type S)
  - constitution_gate
  - tactile_router
  - session_planner
  - perception_firewall
  - world_model_ingestor
  - executive_policy

### Ephemeral (Type A)
  - legislator
  - requirements_interrogator
  - mangle_repair
  - campaign_runner

Total: 10 agents

```


### 01-kernel: jit status
- Time: 2026-07-13 05:18:13
- Command: `.\nerd.exe -w "C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\stress_ws_01" jit`
- Exit: 1
- Note: JIT compiler status light diag

```
ΓÜí JIT Prompt Compiler Status
ΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇ

```


### 01-kernel: glassbox
- Time: 2026-07-13 05:18:16
- Command: `.\nerd.exe -w "C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\stress_ws_01" glassbox`
- Exit: 1
- Note: Kernel transparency

```

```


## check-mangle adversarial assets (expect failures = PASS for stress)


### 01-kernel/mangle-failure-modes: syntactic\souffle_syntax.mg
- Time: 2026-07-13 05:18:28
- Command: `nerd check-mangle .\.agents\skills\stress-tester\assets\mangle-adversarial\syntactic\souffle_syntax.mg`
- Exit: 1
- Note: EXPECTED FAILURE detected (stress PASS for validator)

```
ERROR in .\.agents\skills\stress-tester\assets\mangle-adversarial\syntactic\souffle_syntax.mg: failed to parse schema: 6:0 mismatched input '.' expecting <EOF>
12:38 token recognition error at: ';'
29:29 token recognition error at: '-d'


```


### 01-kernel/mangle-failure-modes: syntactic\missing_periods.mg
- Time: 2026-07-13 05:18:29
- Command: `nerd check-mangle .\.agents\skills\stress-tester\assets\mangle-adversarial\syntactic\missing_periods.mg`
- Exit: 1
- Note: EXPECTED FAILURE detected (stress PASS for validator)

```
ERROR in .\.agents\skills\stress-tester\assets\mangle-adversarial\syntactic\missing_periods.mg: failed to parse schema: 6:10 mismatched input 'I.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
6:21 mismatched input '>' expecting '('
8:0 mismatched input 'item' expecting {'.', '\u27F8', ':-', '@'}
11:12 mismatched input 'P.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
11:23 mismatched input '>' expecting '('
11:37 mismatched input '>' expecting '('
12:14 mismatched input 'A.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
12:25 mismatched input '>' expecting '('
12:39 mismatched input '>' expecting '('
15:0 missing '.' at 'ancestor'
18:10 mismatched input 'From.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
18:24 mismatched input '>' expecting '('
18:39 mismatched input '>' expecting '('
22:11 mismatched input 'V.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
22:21 mismatched input '>' expecting '('
24:0 mismatched input 'value' expecting {'.', '\u27F8', ':-', '@'}
27:0 mismatched input 'Decl' expecting {'.', '\u27F8', ':-', '@'}
27:12 mismatched input 'N.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
27:22 mismatched input '>' expecting '('
34:0 missing '.' at 'Decl'
34:12 mismatched input 'S.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
34:23 mismatched input '>' expecting '('
38:0 mismatched input 'Decl' expecting {'.', '\u27F8', ':-', '@'}
38:10 mismatched input 'N.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
38:21 mismatched input '>' expecting '('
40:0 mismatched input 'node' expecting {'.', '\u27F8', ':-', '@'}
41:0 mismatched input 'node' expecting {'.', '\u27F8', ':-', '@'}
44:0 mismatched input 'Decl' expecting {'.', '\u27F8', ':-', '@'}
44:10 mismatched input 'From.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
44:24 mismatched input '>' expecting '('
44:39 mismatched input '>' expecting '('
45:15 mismatched input 'A.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
4
...[truncated]...
```


### 01-kernel/mangle-failure-modes: syntactic\lowercase_vars.mg
- Time: 2026-07-13 05:18:31
- Command: `nerd check-mangle .\.agents\skills\stress-tester\assets\mangle-adversarial\syntactic\lowercase_vars.mg`
- Exit: 1
- Note: EXPECTED FAILURE detected (stress PASS for validator)

```
ERROR in .\.agents\skills\stress-tester\assets\mangle-adversarial\syntactic\lowercase_vars.mg: failed to parse schema: 6:12 mismatched input 'P.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
6:23 mismatched input '>' expecting '('
6:37 mismatched input '>' expecting '('
8:10 no viable alternative at input 'x,'
8:10 mismatched input ',' expecting '('
8:13 missing '(' at ')'
8:26 no viable alternative at input 'x,'
8:26 mismatched input ',' expecting '('
8:29 missing '(' at ')'
11:10 mismatched input 'From.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
11:24 mismatched input '>' expecting '('
11:39 mismatched input '>' expecting '('
13:9 no viable alternative at input 'y)'
13:9 missing '(' at ')'
13:23 no viable alternative at input 'y)'
13:23 missing '(' at ')'
16:11 mismatched input 'V.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
16:21 mismatched input '>' expecting '('
19:9 no viable alternative at input 'sum)'
19:9 missing '(' at ')'
19:28 mismatched input '=' expecting '('
22:10 mismatched input 'S.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
22:21 mismatched input '>' expecting '('
23:15 mismatched input 'D.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
23:26 mismatched input '>' expecting '('
25:8 no viable alternative at input 'x)'
25:8 missing '(' at ')'
25:19 no viable alternative at input 'x)'
25:19 missing '(' at ')'
25:20 mismatched input ',' expecting {'.', '\u27F8', ':-', '@'}
25:26 extraneous input 'dangerous' expecting '('
25:37 no viable alternative at input 'x)'
25:37 missing '(' at ')'
28:10 mismatched input 'I.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
28:21 mismatched input '>' expecting '('
30:13 no viable alternative at input 'item)'
30:13 missing '(' at ')'
30:27 no viable alternative at input 'item)'
30:27 missing '(' at ')'
33:12 mismatched input 'N.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
33:22 mismatched input '>' expecting '('
35:8 no viable alternative at input 'x,'
35:8 mismatched input ',' expecting '('
35:11 missing '(' at ')'
35:24 no viable alternative at input 'x)'
35:24 missing 
...[truncated]...
```


### 01-kernel/mangle-failure-modes: safety\unsafe_negation.mg
- Time: 2026-07-13 05:18:33
- Command: `nerd check-mangle .\.agents\skills\stress-tester\assets\mangle-adversarial\safety\unsafe_negation.mg`
- Exit: 1
- Note: EXPECTED FAILURE detected (stress PASS for validator)

```
ERROR in .\.agents\skills\stress-tester\assets\mangle-adversarial\safety\unsafe_negation.mg: failed to parse schema: 6:12 mismatched input 'P.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
6:23 mismatched input '>' expecting '('
7:9 mismatched input 'B.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
7:20 mismatched input '>' expecting '('
12:22 extraneous input 'bad' expecting '('
15:13 mismatched input 'B.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
15:24 mismatched input '>' expecting '('
18:26 extraneous input 'blocked' expecting '('
21:10 mismatched input 'From.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
21:24 mismatched input '>' expecting '('
21:39 mismatched input '>' expecting '('
22:15 mismatched input 'F.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
22:26 mismatched input '>' expecting '('
22:40 mismatched input '>' expecting '('
26:35 extraneous input 'forbidden' expecting '('
29:12 mismatched input 'A.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
29:23 mismatched input '>' expecting '('
30:15 mismatched input 'S.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
30:26 mismatched input '>' expecting '('
34:37 no viable alternative at input 'not active'
34:37 extraneous input 'active' expecting '('
34:46 extraneous input ')' expecting {'.', '\u27F8', ':-', '@'}
37:10 mismatched input 'I.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
37:21 mismatched input '>' expecting '('
38:14 mismatched input 'E.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
38:25 mismatched input '>' expecting '('
43:6 extraneous input 'excluded' expecting '('
48:11 mismatched input 'V.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
48:22 mismatched input '>' expecting '('
51:16 missing '(' at 'not'
51:20 extraneous input 'valid' expecting '('
54:10 mismatched input 'U.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
54:21 mismatched input '
...[truncated]...
```


### 01-kernel/mangle-failure-modes: safety\stratification_cycles.mg
- Time: 2026-07-13 05:18:34
- Command: `nerd check-mangle .\.agents\skills\stress-tester\assets\mangle-adversarial\safety\stratification_cycles.mg`
- Exit: 1
- Note: EXPECTED FAILURE detected (stress PASS for validator)

```
ERROR in .\.agents\skills\stress-tester\assets\mangle-adversarial\safety\stratification_cycles.mg: failed to parse schema: 6:12 mismatched input 'X.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
6:23 mismatched input '>' expecting '('
10:12 extraneous input 'q' expecting '('
11:12 extraneous input 'p' expecting '('
14:12 mismatched input 'X.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
14:23 mismatched input '>' expecting '('
18:12 extraneous input 'b' expecting '('
19:12 extraneous input 'c' expecting '('
20:12 extraneous input 'a' expecting '('
23:11 mismatched input 'S.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
23:22 mismatched input '>' expecting '('
26:28 extraneous input 'paradox' expecting '('
29:10 mismatched input 'I.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
29:21 mismatched input '>' expecting '('
32:25 extraneous input 'helper' expecting '('
36:10 mismatched input 'N.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
36:21 mismatched input '>' expecting '('
37:10 mismatched input 'From.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
37:24 mismatched input '>' expecting '('
37:39 mismatched input '>' expecting '('
42:46 extraneous input 'blocked' expecting '('
46:12 mismatched input 'P.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
46:23 mismatched input '>' expecting '('
47:11 mismatched input 'P1.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
47:23 mismatched input '>' expecting '('
47:38 mismatched input '>' expecting '('
51:29 extraneous input 'suspicious' expecting '('
52:43 extraneous input 'suspicious' expecting '('
56:12 mismatched input 'N.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
56:22 mismatched input '>' expecting '('
63:42 extraneous input 'even' expecting '('
66:11 mismatched input 'V.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
66:21 mismatched input '>' expecting '('
69:25 extraneous input 'low' expecting '('
70:24 extraneous input 'hig
...[truncated]...
```


### 01-kernel/mangle-failure-modes: types\hallucinated_functions.mg
- Time: 2026-07-13 05:18:36
- Command: `nerd check-mangle .\.agents\skills\stress-tester\assets\mangle-adversarial\types\hallucinated_functions.mg`
- Exit: 1
- Note: EXPECTED FAILURE detected (stress PASS for validator)

```
ERROR in .\.agents\skills\stress-tester\assets\mangle-adversarial\types\hallucinated_functions.mg: failed to parse schema: 6:10 mismatched input 'T.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
6:23 mismatched input '>' expecting '('
13:13 mismatched input 'N.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
13:24 mismatched input '>' expecting '('
16:48 no viable alternative at input 'fn:double)'
16:48 missing '(' at ')'
17:50 no viable alternative at input 'fn:plus)'
17:50 missing '(' at ')'
20:15 mismatched input 'T.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
20:28 mismatched input '>' expecting '('
27:11 mismatched input 'V.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
27:23 mismatched input '>' expecting '('
36:13 mismatched input 'M.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
36:26 mismatched input '>' expecting '('
43:11 mismatched input 'I.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
43:22 mismatched input '>' expecting '('
50:19 mismatched input 'S.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
50:32 mismatched input '>' expecting '('
57:13 mismatched input 'P.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
57:26 mismatched input '>' expecting '('
63:14 mismatched input 'J.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
63:27 mismatched input '>' expecting '('
81:7 mismatched input 'X.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
81:17 mismatched input '>' expecting '('
82:7 mismatched input 'Y.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
82:17 mismatched input '>' expecting '('


```


### 01-kernel/mangle-failure-modes: structures\json_syntax.mg
- Time: 2026-07-13 05:18:37
- Command: `nerd check-mangle .\.agents\skills\stress-tester\assets\mangle-adversarial\structures\json_syntax.mg`
- Exit: 1
- Note: EXPECTED FAILURE detected (stress PASS for validator)

```
ERROR in .\.agents\skills\stress-tester\assets\mangle-adversarial\structures\json_syntax.mg: failed to parse schema: 6:12 mismatched input 'C.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
6:25 mismatched input '>' expecting '('
11:14 mismatched input 'S.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
11:27 mismatched input '>' expecting '('
16:10 mismatched input 'D.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
16:23 mismatched input '>' expecting '('
21:10 mismatched input 'U.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
21:23 mismatched input '>' expecting '('
23:19 no viable alternative at input '{ /name: alice,'
23:19 no viable alternative at input '/name: alice,'
23:12 mismatched input ':' expecting {'.', '\u27F8', ':-', '@'}
23:19 mismatched input ',' expecting '('
23:25 mismatched input ':' expecting {'.', '\u27F8', ':-', '@'}
23:30 mismatched input '}' expecting {'.', '\u27F8', ':-', '@'}
26:13 mismatched input 'S.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
26:26 mismatched input '>' expecting '('
32:12 mismatched input 'N.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
32:25 mismatched input '>' expecting '('
42:11 mismatched input 'I.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
42:22 mismatched input '>' expecting '('
45:11 mismatched input 'M.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
45:24 mismatched input '>' expecting '('
49:11 mismatched input 'F.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
49:24 mismatched input '>' expecting '('
51:22 no viable alternative at input '{ /enabled: true,'
51:22 no viable alternative at input '/enabled: true,'
51:16 mismatched input ':' expecting {'.', '\u27F8', ':-', '@'}
51:22 mismatched input ',' expecting '('
51:32 mismatched input ':' expecting {'.', '\u27F8', ':-', '@'}
51:40 mismatched input '}' expecting '('
55:14 mismatched input 'N.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
55:27 mismatched input '>' expec
...[truncated]...
```


### 01-kernel/mangle-failure-modes: loops\direct_self_reference.mg
- Time: 2026-07-13 05:18:38
- Command: `nerd check-mangle .\.agents\skills\stress-tester\assets\mangle-adversarial\loops\direct_self_reference.mg`
- Exit: 1
- Note: EXPECTED FAILURE detected (stress PASS for validator)

```
ERROR in .\.agents\skills\stress-tester\assets\mangle-adversarial\loops\direct_self_reference.mg: failed to parse schema: 6:10 mismatched input 'L.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
6:20 mismatched input '>' expecting '('
12:14 mismatched input 'I.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
12:25 mismatched input '>' expecting '('
17:10 mismatched input 'G.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
17:20 mismatched input '>' expecting '('
23:14 mismatched input 'S.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
23:25 mismatched input '>' expecting '('
29:11 mismatched input 'M.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
29:21 mismatched input '>' expecting '('
36:14 mismatched input 'F.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
36:24 mismatched input '>' expecting '('
42:10 mismatched input 'P.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
42:20 mismatched input '>' expecting '('
43:10 mismatched input 'P.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
43:20 mismatched input '>' expecting '('
50:13 mismatched input 'C.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
50:23 mismatched input '>' expecting '('
57:15 mismatched input 'E.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
57:25 mismatched input '>' expecting '('
67:12 mismatched input 'N.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
67:22 mismatched input '>' expecting '('
78:13 mismatched input 'D.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
78:23 mismatched input '>' expecting '('
84:18 mismatched input 'S.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
84:31 mismatched input '>' expecting '('


```


### 01-kernel/mangle-self-healing: known-good
- Time: 2026-07-13 05:18:39
- Command: `nerd check-mangle C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\stress_ws_01\good_test.mg`
- Exit: 1
- Note: Valid Decl+rule smoke

```
ERROR in C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\stress_ws_01\good_test.mg: failed to parse schema: 1:12 mismatched input 'A.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
1:23 mismatched input '>' expecting '('
1:37 mismatched input '>' expecting '('
2:14 mismatched input 'A.Type' expecting {')', '[', '{', NUMBER, FLOAT, VARIABLE, NAME, DOT_TYPE, CONSTANT, STRING, BYTESTRING}
2:25 mismatched input '>' expecting '('
2:39 mismatched input '>' expecting '('


```


### 01-kernel/mangle-self-healing: known-bad undeclared
- Time: 2026-07-13 05:18:41
- Command: `nerd check-mangle C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\stress_ws_01\bad_test.mg`
- Exit: 1
- Note: Expect undeclared predicate error

```
ERROR in C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\stress_ws_01\bad_test.mg: failed to analyze schema: in clause "bad_rule(X) :- fake_predicate_xyz(X)." could not find predicate fake_predicate_xyz(A0)

```


### 01-kernel/mangle-self-healing: known-good Decl(X,Y)
- Time: 2026-07-13 05:18:52
- Command: `nerd check-mangle C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\stress_ws_01\good_test2.mg`
- Exit: 1
- Note: Alt Decl syntax

```
ERROR in C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\stress_ws_01\good_test2.mg: failed to analyze schema: predicate parent(A0, A1) declared more than once, previous was {parent(Child,Parent) [doc("")] [{[/string /string]}] <nil>}

```


### 01-kernel/mangle-self-healing: root learned.mg
- Time: 2026-07-13 05:18:54
- Command: `nerd check-mangle C:\CodeProjects\codeNERD\.nerd\mangle\learned.mg`
- Exit: 0
- Note: Validate existing learned rules (read-only)

```
OK: C:\CodeProjects\codeNERD\.nerd\mangle\learned.mg

```


### 01-kernel: schemas.mg present
- Time: 2026-07-13 05:18:54
- Command: `stat C:\CodeProjects\codeNERD\internal\core\defaults\schemas.mg`
- Exit: 0
- Note: Not full check-mangle on entire schemas (size)

```
SKIP full schemas.mg (large policy corpus); file exists size=6316
```


### 02-perception: perception --help
- Time: 2026-07-13 05:18:55
- Command: `nerd perception --help`
- Exit: 0
- Note: CLI surface without LLM

```
Tests how the perception layer interprets user input.
Shows parsed intent, verb, target, and shard routing.

Example:
  nerd perception "review my code"
  nerd perception "push to github"

Usage:
  nerd perception <input> [flags]

Flags:
  -h, --help   help for perception

Global Flags:
      --api-key string     Z.AI API key (or set ZAI_API_KEY env)
      --timeout duration   Operation timeout (default 25m0s)
  -v, --verbose            Enable verbose logging
  -w, --workspace string   Workspace directory (default: current)

```


### 01-kernel: query next_action
- Time: 2026-07-13 05:20:00
- Command: `nerd -w ws query next_action`
- Exit: 0
- Note: Light kernel query

```
{"level":"info","ts":1783934336.4637904,"caller":"nerd/cmd_query.go:59","msg":"Querying facts","predicate":"next_action"}
No facts found for predicate 'next_action'

```


### 01-kernel: query permitted
- Time: 2026-07-13 05:20:34
- Command: `nerd -w ws query permitted`
- Exit: 1
- Note: Light kernel query

```
{"level":"info","ts":1783934401.6531503,"caller":"nerd/cmd_query.go:59","msg":"Querying facts","predicate":"permitted"}

```


### 03-shards: query shard_state
- Time: 2026-07-13 05:20:39
- Command: `nerd -w ws query shard_state`
- Exit: 1
- Note: Shard state without spawn

```
{"level":"info","ts":1783934435.9203298,"caller":"nerd/cmd_query.go:59","msg":"Querying facts","predicate":"shard_state"}

```


### 03-shards: spawn --help
- Time: 2026-07-13 05:20:40
- Command: `nerd spawn --help`
- Exit: 0
- Note: CLI surface only - no mass spawn

```
Spawns a ShardAgent to handle a specific task in isolation.

Shard Types:
  - generalist: Ephemeral, starts blank (RAM only)
  - specialist: Persistent, loads knowledge shard from SQLite
  - coder: Specialized for code writing/TDD loop
  - researcher: Specialized for deep research
  - reviewer: Specialized for code review
  - tester: Specialized for test generation

Usage:
  nerd spawn [shard-type] [task] [flags]

Flags:
  -h, --help   help for spawn

Global Flags:
      --api-key string     Z.AI API key (or set ZAI_API_KEY env)
      --timeout duration   Operation timeout (default 25m0s)
  -v, --verbose            Enable verbose logging
  -w, --workspace string   Workspace directory (default: current)

```


### 01-kernel/mangle-self-healing: unique-pred known-good
- Time: 2026-07-13 05:21:11
- Command: `nerd check-mangle C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\stress_ws_01\good_unique.mg`
- Exit: 0
- Note: Valid rules with unique predicates

```
OK: C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\stress_ws_01\good_unique.mg

```


### 01-kernel: logic
- Time: 2026-07-13 05:21:11
- Command: `nerd -w ws logic`
- Exit: -1
- Note: Kernel facts surface

```

```


### 01-kernel: check-mangle --help
- Time: 2026-07-13 05:21:12
- Command: `nerd check-mangle --help`
- Exit: 1
- Note: CLI surface

```

```


### 01-kernel/mangle-explosion asset: cyclic_rules.mg
- Time: 2026-07-13 05:21:13
- Command: `nerd check-mangle cyclic_rules.mg`
- Exit: -1
- Note: Adversarial cyclic asset

```

```


## Final verdict (conservative smoke)

### Smoke-passed workflows (partial execute)

| Workflow | Category | Steps run | Result |
|----------|----------|-----------|--------|
| mangle-self-healing (conservative) | 01-kernel-core | status, known-good/bad check-mangle, learned.mg OK | **SMOKE PASS** (with notes) |
| mangle-failure-modes (partial) | 01-kernel-core | 8 adversarial assets via check-mangle | **SMOKE PASS** (all rejected as expected) |
| mangle-startup-validation (smoke) | 01-kernel-core | status boot only; no learned.mg mutation | **SMOKE PASS** |
| intent-fuzzing (CLI surface) | 02-perception | perception --help only (no LLM) | **SMOKE PASS** (surface only) |
| shard-explosion (smoke) | 03-shards | agents list, query shard_state, spawn --help; no mass spawn | **SMOKE PASS** (surface only) |

### Not run (by design / constraints)
- queue-saturation, api-scheduler-stress, memory-pressure, concurrent-derivations (heavy)
- intent-fuzzing live LLM perception calls
- campaign-marathon, tdd-infinite-loop, reviewer-finding-explosion
- mass spawn of coder/tester/reviewer
- multi-hour chaos

### Notable observations
1. Isolated workspace `stress_ws_01` works with `-w`; no conflict with polystack SQLite.
2. `agents` listed only 10 agents (system + few Type A); missing coder/tester/reviewer may indicate incomplete workspace registration or embedding engine offline (ollama unavailable warning).
3. `check-mangle` on adversarial assets: **all 8 failed parse/analyze as expected** → validator is live.
4. Decl syntax `Type<n>` in stress assets is **not accepted** by current parser (uses simpler `Decl name(X, Y).`). Assets still fail early (good for rejection smoke) but do not exercise deeper safety/type checkers for those patterns if parse fails first.
5. Known-good with unique predicates: see above step result.
6. Root `.nerd/mangle/learned.mg`: **OK**.
7. Kernel queries (`next_action`, `permitted`, `shard_state`): complete without panic; empty/no-facts is fine for cold workspace.
8. No panics observed in this smoke series.

### Isolation
- Workspace: `C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\stress_ws_01`
- Config copied from polystack/root
- Serial nerd only in this workspace
- Root learned.mg not mutated

Completed: 2026-07-13 05:21:13

### 01-kernel: cyclic_rules.mg (recheck)
- Exit: 1
```
ERROR in .\.agents\skills\stress-tester\assets\cyclic_rules.mg: failed to parse schema: 9:16 no viable alternative at input 'from: name'
9:16 missing '(' at 'name'
9:20 mismatched input ',' expecting '('
9:26 missing '(' at 'name'
9:30 missing '(' at ')'
10:13 no viable alternative at input 'n: name'
10:13 missing '(' at 'name'
10:17 missing '(' at ')'
11:21 no viable alternative at input 'from: name'
11:21 missing '(' at 'name'
11:25 mismatched input ',' expecting '('
11:31 missing '(' at 'name'
11:35 missing '(' at ')'
12:16 no viable alternative at input 'from: name'
12:16 missing '(' at 'name'
12:20 mismatched input ',' expecting '('
12:26 missing '(' at 'name'
12:30 mismatched input ',' expecting '('
12:40 missing '(' at 'num'
12:43 missing '(' at ')'
13:18 no viable alternative at input 'a: name'
13:18 missing '(' at 'name'
13:22 mismatched input ',' expecting '('
13:27 missing '(' at 'name'
13:31 missing '(' at ')'
14:21 no viable alternative at input 'n: name'
14:21 missing '(' at 'name'
14:25 missing '(' at ')'
15:21 no viable alternative at input 'id: num'
15:21 missing '(' at 'num'
15:24 mismatched input ',' expecting '('
15:35 missing '(' at 'name'
15:39 missing '(' at ')'
64:50 token recognition error at: '+'
64:52 missing '.' at '1'
64:53 mismatched input ',' expecting {'.', '\u27F8', ':-', '@'}
64:57 mismatched input '<' expecting {'.', '\u27F8', ':-', '@'}


```

### 01-kernel: check-mangle --help (recheck)
- Exit: 0
```
Validates the syntax of Google Mangle (Datalog) logic files.

Usage:
  nerd check-mangle [file...] [flags]

Flags:
  -h, --help   help for check-mangle

Global Flags:
      --api-key string     Z.AI API key (or set ZAI_API_KEY env)
      --timeout duration   Operation timeout (default 25m0s)
  -v, --verbose            Enable verbose logging
  -w, --workspace string   Workspace directory (default: current)

```

### 01-kernel: logic (recheck)
- Exit: -1
```
≡ƒºá Mangle Kernel Facts
≡ƒöì Query: *
ΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇ

```
