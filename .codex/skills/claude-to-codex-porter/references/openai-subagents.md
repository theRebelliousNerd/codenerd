# OpenAI Codex Subagents Notes

Source: https://developers.openai.com/codex/subagents
Accessed: 2026-07-09

This file is a heading-aligned summary of the official Codex subagents docs, adapted
for local reference use in this repository.

## Subagents

- Codex can run subagent workflows and collect the results into one response.
- Custom agents let you define task-specific model settings and instructions.

## Availability

- Current Codex releases enable subagent workflows by default.
- Delegation can be requested by the user or required by applicable repository
  or skill instructions. A workflow may still adopt a stricter local policy.

## Typical Workflow

- Codex handles orchestration across agents: spawning, routing follow-up work,
  waiting for results, and closing threads.
- Subagent workflows cost more tokens than a comparable single-agent run.

## Managing Subagents

- The CLI exposes `/agent` for inspecting and switching active threads.
- The parent workflow can steer, stop, or close running subagents.

## Approvals and Sandbox Controls

- Subagents inherit the current sandbox policy by default.
- Parent-session live runtime overrides are reapplied when children are spawned.
- Custom agents can still set narrower sandbox expectations such as read-only mode.

## Custom Agents

- Built-in agents include `default`, `worker`, and `explorer`.
- The docs describe custom agents as standalone TOML files under `~/.codex/agents/`
  for personal use or `.codex/agents/` for project scope.
- Each file defines one custom agent.
- Custom agents are source-of-truth TOML files. The `name` field, not the file
  name, is the stable agent identity.

## Global Settings

The docs call out these `[agents]` config keys:

- `max_threads`
- `max_depth`
- `job_max_runtime_seconds`

The docs warn that increasing depth can multiply token use and reduce predictability.

## Custom Agent File Schema

The official required fields are:

- `name`
- `description`
- `developer_instructions`

The docs also mention optional fields such as:

- `nickname_candidates`
- `model`
- `model_reasoning_effort`
- `sandbox_mode`
- `mcp_servers`
- `skills.config`

The `name` field is the source of truth, even if the filename differs.

## Current Model Guidance

- Current official guidance starts demanding ambiguous agents with `gpt-5.6`
  and uses `gpt-5.6-terra` for faster or lighter supporting work.
- Preserve an intentional repository pin, including older-model workflows, when
  migration is not a model-upgrade request.
- Reasoning levels are model-dependent; current documented values can include
  `ultra`, `max`, `xhigh`, `high`, `medium`, `low`, `minimal`, and `none`.
- Do not hard-code a model/effort pair in a porter. Re-check current docs and the
  target repository's proven settings for every migration.
- Omit model fields when the runtime default is the intended speed,
  intelligence, and cost policy.

codeNERD's local Claude-label conversion baseline is `opus` -> `gpt-5.6`,
`sonnet` -> `gpt-5.6-terra`, and `haiku` -> `gpt-5.6-terra`. This is a
repository migration policy layered on top of current official model support,
not a universal OpenAI product rule.

## Display Nicknames

- `nickname_candidates` is optional.
- Nicknames are presentation-only and do not change how Codex identifies the agent.

## Example Custom Agents

The docs recommend narrow, opinionated agents with a tool surface that matches the
job. The example pattern splits work into focused agents such as:

- a read-only explorer
- a reviewer
- a docs researcher tied to an MCP server

Migration implication for this repo:

- When porting a Claude agent into `.codex/agents/`, convert it into a narrow Codex
  TOML agent rather than leaving it as a free-form Markdown sidecar.
