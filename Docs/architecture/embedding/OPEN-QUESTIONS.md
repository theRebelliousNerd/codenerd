# OPEN QUESTIONS — embedding

> Last verified: 2026-07-13  
> Real open questions (not rhetorical)

## Q1 — Single boot policy for unhealthy Ollama?

`system.factory` drops the engine on HealthCheck failure; `session_boot` keeps it.  
Which is intentional product behavior for interactive vs scripted Cortex?

## Q2 — Should Dimensions() ever become dynamic?

Hardcoding 768/3072 is simple and store-friendly, but wrong for alternate Ollama models.  
Is config-driven dimension + validation preferred over first-embed probe?

## Q3 — Is EmbedBatchJob a supported product path?

Implemented and commented for corpus-scale jobs, but poll/helpers live elsewhere (or not at all).  
Should tools own the full async lifecycle, or should this package export a `WaitBatchJob` helper?

## Q4 — Task type persistence completeness?

Store paths often persist task type for reflection. Are **all** write paths (MCP, campaign, init, tools) consistent enough that reflection never false-triggers reembed storms?

## Q5 — Default provider for new workspaces?

Default is Ollama. Cloud-first users always reconfigure. Is that still the right default for codeNERD distribution assumptions?

## Q6 — Shared HTTP transport with perception?

`genai.go` comments reference `internal/perception/transport.go` for 429 risk.  
Is GenAI embed client sharing that transport today, or still a separate SDK client? (Construct uses `genai.NewClient` with APIKey only.)

## Q7 — SIMD in release builds?

Is `-tags simd` part of any release pipeline? If not, is the AMD64 file dead weight in practice?

## Q8 — FindTopK retention?

Primary search is store-side. Is `FindTopK` still needed as a public helper, or only for tests/tools/perception convenience?

## Q9 — Multi-endpoint Ollama?

Only one endpoint per engine. Any plan for multi-host load balance, or always external reverse proxy?

## Q10 — Vertex AI embeddings?

SDK notes Developer API for CreateEmbeddings. Do we ever need Vertex backend config, or is Developer API permanent product choice?
