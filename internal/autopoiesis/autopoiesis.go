// Package autopoiesis implements self-modification capabilities for codeNERD.
// Autopoiesis (from Greek: self-creation) enables the system to:
// 1. Detect when tasks require campaign orchestration (complex multi-phase work)
// 2. Generate new tools when existing capabilities are insufficient
// 3. Create persistent agents when ongoing monitoring/learning is needed
//
// The Orchestrator façade is split by concern across the autopoiesis_*.go
// files: types, orchestrator construction, kernel integration, delegation,
// agents, analysis, tools, feedback, profiles and helpers. The engines behind
// it live beside them (ouroboros.go, toolgen.go, registry.go, feedback.go,
// thunderdome.go, panic_maker.go). Every policy decision crosses the kernel
// boundary (ShouldGenerateTool, QueryNextAction in autopoiesis_kernel.go);
// with no kernel attached the engines are libraries, not an agent.
//
// This file used to carry a modularization note with a line count per file,
// which drifted as the files grew and was quoted as fact; the list above names
// the files and not their sizes for that reason.
package autopoiesis
