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

# Rule 4: Prohibit process-exiting calls. `log` is allowlisted, so log.Fatal
# would otherwise let generated code kill the host. os.Exit/syscall.Exit are
# defense in depth for configs that admit os/syscall.
violation(Func) :-
    ast_call(Func, "log.Fatal").
violation(Func) :-
    ast_call(Func, "log.Fatalf").
violation(Func) :-
    ast_call(Func, "log.Fatalln").
violation(Func) :-
    ast_call(Func, "os.Exit").
violation(Func) :-
    ast_call(Func, "syscall.Exit").
