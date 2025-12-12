# Ouroboros Go Safety Policy
# Defines minimal safety constraints for runtime-generated Go code.
# File: internal/core/defaults/go_safety.mg

Decl ast_import(FileName, ImportPath) descr [mode("-", "-")].
Decl ast_call(FuncName, Callee) descr [mode("-", "-")].
Decl ast_goroutine_spawn(TargetFunc, LineNum) descr [mode("-", "-")].
Decl ast_uses_context_cancellation(LineNum) descr [mode("-")].
Decl ast_assignment(VarName, Value) descr [mode("-", "-")].
Decl allowed_package(PkgName) descr [mode("-")].
Decl violation(Reason) descr [mode("-")].

# Rule 1: Only allow imports explicitly on the allowlist.
violation(P) :-
    ast_import(_, P),
    !allowed_package(P).

# Rule 2: Goroutines must be tied to a cancelable context.
violation(Line) :-
    ast_goroutine_spawn(_, Line),
    !ast_uses_context_cancellation(Line).

# Rule 3: Prohibit panic for generated code; force error returns instead.
violation(Func) :-
    ast_call(Func, /panic).
