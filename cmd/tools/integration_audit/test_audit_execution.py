import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

SCRIPT_DIR = Path(__file__).resolve().parent
sys.path.insert(0, str(SCRIPT_DIR))

import audit_execution as audit


def write_go(root: Path, relative: str, source: str = "package p\n") -> Path:
    path = root / relative
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(source, encoding="utf-8")
    return path


class ExecutionAuditorTests(unittest.TestCase):
    """Regression tests for the tracked integration audit engine."""
    def make_workspace(self):
        temp = tempfile.TemporaryDirectory()
        root = Path(temp.name)
        (root / "go.mod").write_text("module fixture\n", encoding="utf-8")
        self.addCleanup(temp.cleanup)
        return root

    def test_sanitize_go_source_preserves_offsets_and_blanks_non_code(self):
        source = 'package p\n// NewServer()\nvar s = "make(chan int)"\n/* x.Run(ctx) */\nfunc f() {}\n'
        sanitized = audit.sanitize_go_source(source)
        self.assertEqual(len(source), len(sanitized))
        self.assertEqual(source.count("\n"), sanitized.count("\n"))
        self.assertNotIn("NewServer", sanitized)
        self.assertNotIn("make(chan", sanitized)
        self.assertNotIn("x.Run", sanitized)
        self.assertIn("func f", sanitized)

    def test_component_filter_uses_segments_and_excludes_tool_worktrees(self):
        root = self.make_workspace()
        expected = {
            write_go(root, "internal/core/a.go"),
            write_go(root, "internal/tools/core/b.go"),
        }
        write_go(root, "internal/campaign/core_helper.go")
        write_go(root, ".claude/worktrees/stale/internal/core/stale.go")

        auditor = audit.ExecutionAuditor(str(root), component="core")
        self.assertEqual(set(auditor._collect_go_files()), expected)

    def test_composite_literal_satisfies_typed_field_assignment(self):
        root = self.make_workspace()
        source = """package p
type Service struct { client *Client }
type Client struct{}
func NewService(client *Client) *Service { return &Service{client: client} }
func (s *Service) Run() { if s.client != nil {} }
"""
        path = write_go(root, "internal/p/service.go", source)
        auditor = audit.ExecutionAuditor(str(root))
        auditor._audit_field_assignments([path])
        self.assertEqual(auditor.result.findings, [])

    def test_unassigned_typed_field_is_warning_with_line(self):
        root = self.make_workspace()
        source = """package p
type Service struct { client *Client }
type Client struct{}
func (s *Service) Run() { if s.client != nil {} }
"""
        path = write_go(root, "internal/p/service.go", source)
        auditor = audit.ExecutionAuditor(str(root))
        auditor._audit_field_assignments([path])
        self.assertEqual(len(auditor.result.findings), 1)
        finding = auditor.result.findings[0]
        self.assertEqual(finding.severity, audit.Severity.WARNING)
        self.assertEqual(finding.line, 4)
        self.assertIn("Service.client", finding.message)

    def test_promoted_embedded_field_uses_embedded_type_assignment(self):
        root = self.make_workspace()
        source = """package p
type Base struct { kernel *Kernel }
type System struct { *Base }
type Kernel struct{}
func (b *Base) SetKernel(kernel *Kernel) { b.kernel = kernel }
func (s *System) Run() { if s.kernel != nil {} }
"""
        path = write_go(root, "internal/p/system.go", source)
        auditor = audit.ExecutionAuditor(str(root))
        auditor._audit_field_assignments([path])
        self.assertEqual(auditor.result.findings, [])

    def test_context_cancel_free_function_is_recognized(self):
        root = self.make_workspace()
        source = """package p
func f() {
    ctx, cancel := context.WithCancel(parent)
    defer cancel()
    _ = ctx
}
"""
        path = write_go(root, "internal/p/context.go", source)
        auditor = audit.ExecutionAuditor(str(root))
        auditor._audit_object_execution([path])
        self.assertEqual(auditor.result.findings, [])

    def test_return_in_prior_function_does_not_suppress_unrun_server(self):
        root = self.make_workspace()
        source = """package p
func earlier() error { return nil }
func build() {
    server := NewServer()
}
"""
        path = write_go(root, "internal/p/server.go", source)
        auditor = audit.ExecutionAuditor(str(root))
        auditor._audit_object_execution([path])
        self.assertEqual(len(auditor.result.findings), 1)
        self.assertEqual(auditor.result.findings[0].severity, audit.Severity.ERROR)

    def test_constructor_multi_return_tracks_object_not_error_and_start_prefix(self):
        root = self.make_workspace()
        source = """package p
func build() {
    orch, err := NewOrchestrator()
    _ = err
    orch.StartKernelListener()
}
"""
        path = write_go(root, "internal/p/orchestrator.go", source)
        auditor = audit.ExecutionAuditor(str(root))
        auditor._audit_object_execution([path])
        self.assertEqual(auditor.result.findings, [])

    def test_constructor_assigned_directly_to_field_is_not_local_creation(self):
        root = self.make_workspace()
        source = """package p
func build(ctx *Context) { ctx.orch = NewOrchestrator() }
"""
        path = write_go(root, "internal/p/orchestrator.go", source)
        auditor = audit.ExecutionAuditor(str(root))
        auditor._audit_object_execution([path])
        self.assertEqual(auditor.result.findings, [])

    def test_constructor_returned_inside_composite_literal_transfers_ownership(self):
        root = self.make_workspace()
        source = """package p
func build() message {
    orch, err := NewOrchestrator()
    _ = err
    return message{
        orchestrator: orch,
    }
}
"""
        path = write_go(root, "internal/p/orchestrator.go", source)
        auditor = audit.ExecutionAuditor(str(root))
        auditor._audit_object_execution([path])
        self.assertEqual(auditor.result.findings, [])

    def test_goroutine_check_uses_actual_enclosing_function_and_tea_cmd(self):
        root = self.make_workspace()
        source = """package p
func (m Model) Run(ctx context.Context) { child.Run(ctx) }
func (m Model) helper(ctx context.Context) { child.Run(ctx) }
func (m Model) command() tea.Cmd { return func() tea.Msg { child.Run(ctx); return nil } }
"""
        path = write_go(root, "internal/p/model.go", source)
        auditor = audit.ExecutionAuditor(str(root))
        auditor._audit_goroutine_spawning([path])
        self.assertEqual(len(auditor.result.findings), 1)
        self.assertEqual(auditor.result.findings[0].line, 3)

    def test_message_handlers_accept_multi_qualified_pointer_and_assertion(self):
        root = self.make_workspace()
        source = """package p
type fooMsg struct{}
type barMsg struct{}
type bazMsg struct{}
func update(msg any) {
    switch msg.(type) { case fooMsg, *pkg.barMsg: }
    if _, ok := msg.(pkg.bazMsg); ok {}
}
"""
        path = write_go(root, "internal/p/messages.go", source)
        auditor = audit.ExecutionAuditor(str(root))
        auditor._audit_message_handlers([path])
        self.assertEqual(auditor.result.findings, [])

    def test_json_mode_is_machine_readable_and_errors_exit_nonzero(self):
        root = self.make_workspace()
        write_go(
            root,
            "internal/p/server.go",
            "package p\nfunc build() {\n    server := NewServer()\n}\n",
        )
        completed = subprocess.run(
            [sys.executable, str(SCRIPT_DIR / "audit_execution.py"), str(root), "--json"],
            text=True,
            capture_output=True,
            check=False,
        )
        payload = json.loads(completed.stdout)
        self.assertEqual(completed.returncode, 1)
        self.assertGreater(payload["stats"]["errors"], 0)
        self.assertIn("EXECUTION WIRING AUDIT", completed.stderr)
        self.assertNotIn("EXECUTION WIRING AUDIT", completed.stdout)


if __name__ == "__main__":
    unittest.main()
