# Schemas for the error-free gates compared (policy/gates.mg).
# Inputs are asserted by internal/gates (Snapshot.Facts, Compare) and the
# recurse loop; the conclusions are read by internal/gates.Compare, `nerd gates
# diff` and policy/recurse.mg.

Decl gate_snapshot(Snapshot) bound [/string].
Decl gate_measured(Snapshot, Gate, Status, Count) bound [/string, /name, /name, /number].
Decl gate_finding(Snapshot, Gate, Key) bound [/string, /name, /string].
Decl gate_comparison(Before, After) bound [/string, /string].
Decl gate_domain(Snapshot, Gate, Domain) bound [/string, /name, /string].

Decl gate_measurable(Status) bound [/name].
Decl gate_present(Snapshot, Gate) bound [/string, /name].
Decl gate_counted(Snapshot, Gate) bound [/string, /name].
Decl gate_same_domain(Before, After, Gate) bound [/string, /string, /name].
Decl gate_unadmitted(Before, After, Gate, Reason) bound [/string, /string, /name, /name].
Decl gate_opaque(Snapshot, Gate) bound [/string, /name].
Decl gate_comparison_has_gate(Before, After) bound [/string, /string].
Decl gate_comparison_empty(Before, After) bound [/string, /string].
Decl gate_comparison_unadmitted(Before, After) bound [/string, /string].
Decl gate_compared(Before, After, Gate, StatusBefore, StatusAfter, CountBefore, CountAfter) bound [/string, /string, /name, /name, /name, /number, /number].
Decl gate_regressed(Before, After, Gate) bound [/string, /string, /name].
Decl gate_improved(Before, After, Gate) bound [/string, /string, /name].
Decl gate_change(Before, After, Gate, Change) bound [/string, /string, /name, /name].
Decl gate_finding_new(Before, After, Gate, Key) bound [/string, /string, /name, /string].
Decl gate_finding_gone(Before, After, Gate, Key) bound [/string, /string, /name, /string].
Decl gate_comparison_regressed(Before, After) bound [/string, /string].
Decl cycle_keepable(Before, After) bound [/string, /string].
