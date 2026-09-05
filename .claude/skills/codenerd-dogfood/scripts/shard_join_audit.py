#!/usr/bin/env python3
"""Find Mangle rules that can never fire in production because their body
joins facts that live in different kernel shards.

The Cortex kernel (internal/core/cortex_kernel.go) evaluates the full
policy program once per shard, over that shard's facts only. A runtime
fact is routed to the shard that owns its predicate
(internal/shards/registration.go manifests) or to the catch-all "cortex"
shard when unowned. Facts written in the .mg files themselves (program
EDB) are present in every shard. No shard exports predicates to another,
so a rule whose body needs an owned fact from shard A and an unowned fact
from the catch-all can only be satisfied in a single-store kernel — the
one every unit test boots. Item 33 (phase_category vs campaign_phase)
was the first instance; this script finds the rest.

Usage: python shard_join_audit.py [repo_root]
Exit 1 when any split join is found.
"""
from __future__ import annotations

import os
import re
import sys
from collections import defaultdict

ROOT = sys.argv[1] if len(sys.argv) > 1 else os.getcwd()
DEFAULTS = os.path.join(ROOT, "internal", "core", "defaults")
MANIFEST = os.path.join(ROOT, "internal", "shards", "registration.go")
ALL = frozenset({"ALL"})


# Rules in these files run inside a per-compilation scope cloned from the
# catch-all shard (internal/system/factory_adapters.go NewCompilationScope):
# a single store holding the selector's ephemeral atom facts. The shard
# model does not apply to them.
SCOPE_EVALUATED = {"jit_compiler.mg", "jit_selection.mg", "jit_logic.mg"}

# Known residue, each with a reason. A finding is (file basename, head).
# Anything not listed here fails the audit. Remove an entry when the rule
# is restructured or the architect decides its facts should move.
ACCEPTED = {
    # Constitution override paths: signed_approval / admin_override /
    # has_active_override / candidate_action live in the catch-all while
    # pending_action lives in the policy shard. These rules have never
    # fired on the production kernel; homing the override facts would make
    # a dormant permission path live. Architect's decision, not a routing
    # fix. Deny-side effects (permission_denied, action_denied,
    # blocked_learned_action_count) only over-deny.
    ("constitution.mg", "permitted"),
    ("constitution.mg", "final_action"),
    ("constitution.mg", "permission_denied"),
    ("constitution.mg", "action_denied"),
    ("constitution.mg", "blocked_learned_action_count"),
    # Campaign quality check joins campaign_task with file_topology and
    # negates test_coverage (both world). Needs restructuring (world-side
    # helper predicate), not sharing of a large family. Dormant Path B.
    ("campaign_rules.mg", "quality_violation_detected"),
    # coder_quality_mode(/normal) :- !in_campaign_context() has no positive
    # atom, so it fires in every shard; the campaign shard also derives
    # /strict when a campaign is active and the fan-out returns both.
    # Needs a positive anchor in the rule. No Go reader today.
    ("coder_campaign.mg", "coder_quality_mode"),
    ("coder_workflow.mg", "coder_quality_mode"),
    # selection_policy.mg derives include_in_context for campaign
    # documents; final_context_include then negates exclude_from_context
    # (world file facts) in the campaign shard. Documents are never
    # generated/vendor/binary files, so the vacuous negation is harmless.
    ("coder_context.mg", "final_context_include"),
    ("coder_workflow.mg", "final_context_include"),
}


def mg_files() -> list[str]:
    out = []
    for sub in ("", "schema", "policy"):
        d = os.path.join(DEFAULTS, sub)
        if not os.path.isdir(d):
            continue
        for name in sorted(os.listdir(d)):
            if name.endswith(".mg") and name not in SCOPE_EVALUATED:
                out.append(os.path.join(d, name))
    return out


def strip_comments(text: str) -> str:
    lines = []
    for line in text.splitlines():
        out, in_str = [], False
        i = 0
        while i < len(line):
            c = line[i]
            if c == '"':
                in_str = not in_str
            if c == "#" and not in_str:
                break
            out.append(c)
            i += 1
        lines.append("".join(out))
    return "\n".join(lines)


def split_clauses(text: str) -> list[str]:
    clauses, cur, in_str = [], [], False
    i = 0
    while i < len(text):
        c = text[i]
        cur.append(c)
        if c == '"':
            in_str = not in_str
        elif c == "." and not in_str:
            nxt = text[i + 1] if i + 1 < len(text) else "\n"
            if nxt in " \n\r\t":
                clauses.append("".join(cur).strip())
                cur = []
        i += 1
    rest = "".join(cur).strip()
    if rest:
        clauses.append(rest)
    return [c for c in clauses if c]


PRED_RE = re.compile(r"(?<![\w:/.])(!?)\s*([a-z_][a-zA-Z0-9_]*)\s*\(")
HEAD_RE = re.compile(r"^\s*([a-z_][a-zA-Z0-9_]*)\s*\(")


def parse_manifest() -> tuple[dict[str, str], set[str]]:
    src = open(MANIFEST, encoding="utf-8").read()
    owners: dict[str, str] = {}
    table = src.split("func SharedPredicates()", 1)[0]
    for m in re.finditer(r'Domain:\s*"(\w+)".*?OwnedPredicates:\s*(?:\[\]string\{(.*?)\}|nil)', table, re.S):
        domain, body = m.group(1), m.group(2) or ""
        for p in re.findall(r'"(\w+)"', strip_go_comments(body)):
            owners[p] = domain
    shared: set[str] = set()
    m = re.search(r"func SharedPredicates\(\) \[\]string \{\s*return \[\]string\{(.*?)\}\s*\}", src, re.S)
    if m:
        shared = set(re.findall(r'"(\w+)"', strip_go_comments(m.group(1))))
    return owners, shared


def strip_go_comments(s: str) -> str:
    return re.sub(r"//[^\n]*", "", s)


def main() -> int:
    owners, shared_preds = parse_manifest()
    decls: set[str] = set()
    program_edb: set[str] = set()
    rules: list[tuple[str, str, list[tuple[bool, str]], str]] = []  # (file, head, [(neg, pred)], text)
    for path in mg_files():
        text = strip_comments(open(path, encoding="utf-8").read())
        rel = os.path.relpath(path, ROOT).replace("\\", "/")
        for clause in split_clauses(text):
            if clause.startswith("Decl "):
                m = re.match(r"Decl\s+([a-z_][a-zA-Z0-9_]*)", clause)
                if m:
                    decls.add(m.group(1))
                continue
            if clause.startswith("Package") or clause.startswith("Use"):
                continue
            hm = HEAD_RE.match(clause)
            if not hm:
                continue
            head = hm.group(1)
            if ":-" not in clause:
                program_edb.add(head)
                continue
            body = clause.split(":-", 1)[1]
            body = body.split("|>", 1)[0]  # transforms only see the body's bindings
            preds = [(neg == "!", p) for neg, p in PRED_RE.findall(body)]
            rules.append((rel, head, preds, clause))

    derived = {r[1] for r in rules}
    rules_by_head: dict[str, list[int]] = defaultdict(list)
    for i, r in enumerate(rules):
        rules_by_head[r[1]].append(i)

    def edb_presence(p: str) -> frozenset[str]:
        if p in program_edb and p not in derived:
            return ALL
        if p in shared_preds:
            return ALL
        return frozenset({owners.get(p, "cortex")})

    # Fixpoint over rules: presence(derived) = union over rules of the
    # intersection of body presences (positive atoms only).
    presence: dict[str, frozenset[str]] = {}
    for p in decls | program_edb:
        if p not in derived:
            presence[p] = edb_presence(p)
    for p in derived:
        presence[p] = frozenset()
        if p in program_edb:
            presence[p] = ALL

    def meet(a: frozenset[str], b: frozenset[str]) -> frozenset[str]:
        if "ALL" in a:
            return b
        if "ALL" in b:
            return a
        return a & b

    def join(a: frozenset[str], b: frozenset[str]) -> frozenset[str]:
        if "ALL" in a or "ALL" in b:
            return ALL
        return a | b

    changed = True
    while changed:
        changed = False
        for _, head, preds, _ in rules:
            cur: frozenset[str] = ALL
            for neg, p in preds:
                if neg:
                    continue
                cur = meet(cur, presence.get(p, edb_presence(p)))
            new = join(presence[head], cur)
            if new != presence[head]:
                presence[head] = new
                changed = True

    split, blind_neg = [], []
    for rel, head, preds, clause in rules:
        pos = [(p, presence.get(p, edb_presence(p))) for neg, p in preds if not neg]
        cur: frozenset[str] = ALL
        for _, pr in pos:
            cur = meet(cur, pr)
        if not cur:
            homes = ", ".join(f"{p}@{'/'.join(sorted(pr)) or '∅'}" for p, pr in pos if pr != ALL)
            split.append((rel, head, homes, clause))
            continue
        for neg, p in preds:
            if not neg:
                continue
            pr = presence.get(p, edb_presence(p))
            if "ALL" in pr:
                continue
            # The rule fires in every shard of `cur`; in any of them where
            # the negated fact cannot exist, the negation is vacuous.
            if "ALL" in cur or not (cur <= pr):
                blind_neg.append((rel, head, p, "/".join(sorted(pr)), "every shard" if "ALL" in cur else "/".join(sorted(cur))))

    def accepted(rel: str, head: str) -> bool:
        return (os.path.basename(rel), head) in ACCEPTED

    accepted_split = [s for s in split if accepted(s[0], s[1])]
    accepted_neg = [b for b in blind_neg if accepted(b[0], b[1])]
    split = [s for s in split if not accepted(s[0], s[1])]
    blind_neg = [b for b in blind_neg if not accepted(b[0], b[1])]

    print(f"policy corpus: {len(rules)} rules, {len(derived)} derived predicates, "
          f"{len(program_edb)} program-EDB predicates, {len(owners)} owned predicates, {len(shared_preds)} shared predicates")
    print(f"accepted residue: {len(accepted_split)} split joins, {len(accepted_neg)} blind negations (see ACCEPTED)")
    print()
    if split:
        print(f"SPLIT JOINS ({len(split)}): body facts live in different shards; the rule never fires in production")
        for rel, head, homes, clause in split:
            one = " ".join(clause.split())
            if len(one) > 160:
                one = one[:157] + "..."
            print(f"- {rel}: {head}  [{homes}]\n    {one}")
        print()
    if blind_neg:
        print(f"BLIND NEGATIONS ({len(blind_neg)}): negated fact is owned by a shard the rule never evaluates in, so the negation always succeeds")
        for rel, head, p, pr, cur in blind_neg:
            print(f"- {rel}: {head}  !{p} lives in {pr}, rule evaluates in {cur}")
        print()
    if split or blind_neg:
        # Trace every split rule to its runtime-EDB leaves (through derived
        # predicates, positive atoms only) and propose a home: when the leaves
        # span exactly one owned domain plus the catch-all, the catch-all
        # leaves belong in that domain; when they span two or more owned
        # domains, only sharing or restructuring can fix the rule.
        leaf_cache: dict[str, frozenset[str]] = {}

        def leaves(p: str, stack: frozenset[str] = frozenset()) -> frozenset[str]:
            if p in leaf_cache:
                return leaf_cache[p]
            if p not in derived:
                out = frozenset() if edb_presence(p) == ALL else frozenset({p})
                leaf_cache[p] = out
                return out
            if p in stack:
                return frozenset()
            out: set[str] = set()
            for i in rules_by_head[p]:
                for neg, q in rules[i][2]:
                    if not neg:
                        out |= leaves(q, stack | {p})
            leaf_cache[p] = frozenset(out)
            return leaf_cache[p]

        proposals: dict[str, dict[str, int]] = defaultdict(lambda: defaultdict(int))
        unfixable: list[tuple[str, str, str]] = []
        for rel, head, preds, _ in split:
            pass
        for rel, head, preds, clause in rules:
            pos = [p for neg, p in preds if not neg]
            pr: frozenset[str] = ALL
            for p in pos:
                pr = meet(pr, presence.get(p, edb_presence(p)))
            if pr:
                continue
            leaf_set: set[str] = set()
            for p in pos:
                leaf_set |= leaves(p)
            by_domain: dict[str, set[str]] = defaultdict(set)
            for leaf in leaf_set:
                by_domain[owners.get(leaf, "cortex")].add(leaf)
            owned_domains = [d for d in by_domain if d != "cortex"]
            if len(owned_domains) == 1 and by_domain.get("cortex"):
                for leaf in by_domain["cortex"]:
                    proposals[owned_domains[0]][leaf] += 1
            elif len(owned_domains) >= 2:
                unfixable.append((rel, head, ", ".join(f"{d}:{'/'.join(sorted(v))}" for d, v in sorted(by_domain.items()))))
        if proposals:
            print("PROPOSED RE-HOMING (catch-all leaf -> domain, rules it unblocks):")
            for domain in sorted(proposals):
                print(f"  {domain}:")
                for leaf, n in sorted(proposals[domain].items(), key=lambda kv: (-kv[1], kv[0])):
                    print(f"    {leaf:40s} {n}")
        if unfixable:
            print()
            print(f"NEEDS SHARING OR RESTRUCTURE ({len(unfixable)}): leaves span two or more owned domains")
            for rel, head, spread in unfixable:
                print(f"- {rel}: {head}  [{spread}]")
    if not split and not blind_neg:
        print("OK: every rule's body facts share a shard (beyond the accepted residue)")
    return 1 if (split or blind_neg) else 0


if __name__ == "__main__":
    sys.exit(main())
