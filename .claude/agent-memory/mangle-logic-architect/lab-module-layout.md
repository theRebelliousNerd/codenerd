---
name: lab-module-layout
description: _mangle_lab is its own Go module (mangle_lab) since 2026-10-28; import mangle_lab/harness and run tests from inside it
metadata:
  type: reference
---

As of 2026-10-28 `_mangle_lab/go.mod` declares `module mangle_lab` with `replace codenerd => ../`. Labs import `mangle_lab/harness` (the old template's `codenerd/_mangle_lab/harness` fails: "main module does not contain package"), and run with `(cd _mangle_lab && go test -v ./<slug>/)`. The mangle skill moved to the MangleLSP plugin (`/home/rebelliousnerd/codeProjects/MangleLSP/skills/mangle-programming`); edits there from a codeNERD session were denied by the permission classifier, so skill changes go in the report.

**How to apply:** check this before scaffolding a lab or editing the skill. See [[mangle-playbook]].
