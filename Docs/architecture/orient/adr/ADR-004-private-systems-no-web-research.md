---
doc-class: governance
subsystem: orient
implementation-status: accepted-not-implemented
last-verified: 2026-09-29
verified-against: e056692c
supersedes: []
---

# ADR-004: Private and Proprietary Systems Receive No Public Web Research

## Context

Today, codeNERD's research tools (`internal/tools/research/web_search.go:138`, `web_fetch.go:100`, `context7.go:293`) blindly query external public endpoints (DuckDuckGo, raw GitHub URLs, public search APIs) whenever research is requested.

In real-world enterprise and startup codebases, projects frequently depend on proprietary internal systems: in-house graph engines, custom security frameworks, private RPC microservices, or proprietary client libraries. When standard agents attempt to "research" these dependencies, they commit two critical violations:
1. **Confidentiality Breach**: Proprietary package names, internal system architectures, and corporate identifiers are transmitted to public search engines.
2. **Hallucination Induction**: When a public search for a private name returns irrelevant public results (or generic keyword matches), the model attempts to synthesize documentation from unrelated software, corrupting its knowledge base.

## Decision

1. **Deductive Identification of Private Systems**:
   The Go sensor (`internal/orient/deps.go`) inventories dependencies from manifests, install configurations (`file:`, `git:`, `--extra-index-url`), vendored packages, and local agent skills. It verifies registry presence using cached public metadata lookups.
2. **Strict Privacy Derivation in Mangle**:
   Policy file `internal/orient/deps.mg` derives `dependency_private(Name, Why)` whenever a system is absent from public registries, sourced locally, or described exclusively by internal skills.
3. **Mandatory Research Suppression**:
   The rule defining external research topics (`orient_research_topic(Name, Topic)`) is constrained by a hard negation guard:
   ```prolog
   # Public web research is strictly forbidden for private dependencies
   orient_research_topic(Agent, Topic) :-
       agent_needs_topic(Agent, Topic),
       !is_private_topic(Topic).
   ```
4. **Internal Knowledge Routing**:
   Knowledge for private systems must be extracted exclusively from repository-internal sources: embedded skills (`agent_source`), local markdown documentation, vendored client sources, and environment examples.

## Consequences

- **Positive**: Complete protection against external data leakage; zero hallucinations caused by spurious search results; forces specialist agents to ground themselves in authentic internal code.
- **Negative**: If an unusual public open-source library is erroneously classified as private, the agent will not search the web for it unless corrected by the operator.
- **Mitigation**: Low-confidence privacy classifications trigger a first-boot clarification question, allowing the operator to explicitly declare a package public.

## Witness

**Witness:** `predicate:dependency_private/2` and `test:TestPrivateDep_SuppressesWebResearch`

| Claim | Witness (Code Evidence as of 2026-09-29) |
|---|---|
| Web research tools query public internet today | `web_search.go:138` (DuckDuckGo) and `context7.go:293` (`http.DefaultClient.Do`). |
| Target Mangle predicate defined | `dependency_private(Name, Why)` in `internal/orient/deps.mg`. |
| Proving regression test | `TestPrivateDep_SuppressesWebResearch` verifying that an unlisted package asserted with `dependency_source(..., /path, ...)` derives `dependency_private` and generates 0 `orient_research_topic` facts. |

## Status (Derived, Not Asserted)

**`accepted-not-implemented`**. Derivation: The decision is approved, but `internal/orient/deps.mg` has not landed in commit `e056692c`. Status flips to `implemented` once the witness test passes.
