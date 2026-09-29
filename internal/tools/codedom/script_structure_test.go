package codedom_test

import (
	"strings"
	"testing"
)

// Script languages take the same structural contract as Go: lookup, outline,
// callers, callees, importers, unreferenced, and a span-exact edit that
// re-parses. The fixtures are the shapes those tools used to refuse.
func TestScriptStructure_PythonTypeScriptAndJavaScript(t *testing.T) {
	reg, root := structWorkspace(t, map[string]string{
		"pkg/services.py": "class User:\n    def __init__(self, id):\n        self.id = id\n\n    def get(self, id):\n        return id\n",
		"pkg/views.py": `"""Users."""
from .services import User

class UserView:
    def fetch(self):
        return helper(User.get(1))

def helper(n):
    return n

def orphan_py():
    return 0
`,
		"box.py":    "class Box:\n    def keep(self):\n        return 1\n\n    def change(self):\n        return 1\n",
		"hooks.tsx": "export function useAuth() {\n  return 1;\n}\n",
		"Panel.tsx": "export function Panel(props: { id: string }) {\n  return <span>{props.id}</span>;\n}\n",
		"other.ts":  "export function useAuth() {\n  return 2;\n}\n",
		"card.tsx": `import { useAuth } from "./hooks";
import Panel from "./Panel";
import { useAuth as ua } from "./hooks";

/** A card. */
export function UserCard({ id }: { id: string }) {
  const session = useAuth();
  return <Panel id={id}><span>hi</span></Panel>;
}

export function Shadow() {
  const useAuth = 1;
  return useAuth;
}

export function Alias() {
  return ua();
}
`,
		"more.tsx":     "import { useAuth } from \"./hooks\";\nexport function More() {\n  return useAuth();\n}\n",
		"greet.js":     "export function greet(name) {\n  return name;\n}\n",
		"main.js":      "import { greet } from \"./greet.js\";\nexport function run() {\n  return greet(\"a\");\n}\nexport function orphanJs() {\n  return 1;\n}\n",
		"widget.jsx":   "import { greet } from \"./greet.js\";\nexport function Badge() {\n  return <span>{greet(\"b\")}</span>;\n}\n",
		"consumer.cjs": "const { greet } = require(\"./greet.js\");\nfunction show() {\n  return greet(\"c\");\n}\n",
	})

	outline := run(t, reg, "package_outline", map[string]any{"path": "pkg"})
	for _, want := range []string{"pkg.User", "pkg.UserView", "pkg.UserView.fetch", "pkg.helper", "class"} {
		if !strings.Contains(outline, want) {
			t.Fatalf("package_outline missing %s:\n%s", want, outline)
		}
	}
	if strings.Contains(outline, "box.py") || strings.Contains(outline, "billing") {
		t.Fatalf("outline of a directory is the files directly in it:\n%s", outline)
	}

	symbols := run(t, reg, "find_symbol", map[string]any{"name": "User.get|helper|Badge|useAuth", "kind": ""})
	if !strings.Contains(symbols, "pkg.User.get") || !strings.Contains(symbols, "pkg.helper") {
		t.Fatalf("find_symbol:\n%s", symbols)
	}
	hooks := run(t, reg, "find_symbol", map[string]any{"name": "useAuth", "kind": "hook"})
	if !strings.Contains(hooks, "hooks.tsx") || !strings.Contains(hooks, "hook") {
		t.Fatalf("useAuth is a hook:\n%s", hooks)
	}
	badge := run(t, reg, "find_symbol", map[string]any{"name": "Badge", "kind": "component"})
	if !strings.Contains(badge, "widget.jsx") || !strings.Contains(badge, "component") {
		t.Fatalf("Badge is a component:\n%s", badge)
	}

	callers := run(t, reg, "callers_of", map[string]any{"symbol": "pkg.User.get"})
	if !strings.Contains(callers, "pkg/views.py") || !strings.Contains(callers, "exact") || !strings.Contains(callers, "fetch") {
		t.Fatalf("callers of User.get:\n%s", callers)
	}
	callees := run(t, reg, "callees_of", map[string]any{"symbol": "pkg.UserView.fetch"})
	if !strings.Contains(callees, "User.get") || !strings.Contains(callees, "pkg/services.py") || !strings.Contains(callees, "helper") {
		t.Fatalf("callees of fetch:\n%s", callees)
	}
	importers := run(t, reg, "importers_of", map[string]any{"package": "pkg/services.py"})
	if !strings.Contains(importers, "pkg/views.py") || !strings.Contains(importers, "pkg/services.py") {
		t.Fatalf("importers of the service module:\n%s", importers)
	}
	hookImporters := run(t, reg, "importers_of", map[string]any{"package": "hooks.tsx"})
	if !strings.Contains(hookImporters, "card.tsx") || !strings.Contains(hookImporters, "more.tsx") {
		t.Fatalf("importers of hooks.tsx:\n%s", hookImporters)
	}

	unref := run(t, reg, "unreferenced_symbols", map[string]any{"path": "pkg"})
	if !strings.Contains(unref, "orphan_py") || strings.Contains(unref, "helper") || strings.Contains(unref, "__init__") {
		t.Fatalf("unreferenced under pkg:\n%s", unref)
	}
	jsUnref := run(t, reg, "unreferenced_symbols", map[string]any{"path": "."})
	if !strings.Contains(jsUnref, "orphanJs") || strings.Contains(jsUnref, "greet.js") {
		t.Fatalf("unreferenced javascript:\n%s", jsUnref)
	}

	jsCallers := run(t, reg, "callers_of", map[string]any{"symbol": "greet"})
	if !strings.Contains(jsCallers, "main.js") || !strings.Contains(jsCallers, "widget.jsx") || !strings.Contains(jsCallers, "consumer.cjs") || !strings.Contains(jsCallers, "exact") {
		t.Fatalf("callers of greet:\n%s", jsCallers)
	}
	jsxCallees := run(t, reg, "callees_of", map[string]any{"symbol": "Badge"})
	if !strings.Contains(jsxCallees, "greet") || strings.Contains(jsxCallees, " span") {
		t.Fatalf("JSX callee is greet, not the intrinsic span:\n%s", jsxCallees)
	}

	replaced := run(t, reg, "replace_element", map[string]any{
		"path": "box.py", "ref": "Box.change",
		"source": "def change(self):\n        return 2",
	})
	if !strings.Contains(replaced, "box.py now parses") {
		t.Fatalf("replace_element:\n%s", replaced)
	}
	box := readRel(t, root, "box.py")
	if !strings.Contains(box, "def keep(self):\n        return 1\n") || !strings.Contains(box, "return 2") || strings.Count(box, "return 1") != 1 {
		t.Fatalf("method replace must keep the sibling byte-for-byte:\n%s", box)
	}

	created := run(t, reg, "create_file", map[string]any{
		"path": "pkg/extra.py", "source": "def added():\n    return 3\n",
	})
	if !strings.Contains(created, "pkg.added") && !strings.Contains(created, "added") {
		t.Fatalf("create_file:\n%s", created)
	}

	delErr := runErr(reg, "delete_element", map[string]any{"path": "hooks.tsx", "ref": "useAuth"})
	if delErr == nil || !strings.Contains(delErr.Error(), "uses remain") || !strings.Contains(delErr.Error(), "card.tsx") {
		t.Fatalf("a used hook is not deleted, and the use is named: %v", delErr)
	}
	if strings.Contains(readRel(t, root, "hooks.tsx"), "useSession") || !strings.Contains(readRel(t, root, "hooks.tsx"), "function useAuth") {
		t.Fatal("refused delete must not write")
	}

	miss := runErr(reg, "repoint", map[string]any{
		"from": "hooks.tsx:useAuth", "to": "useSession",
		"paths": []any{"hooks.tsx", "card.tsx"},
	})
	if miss == nil || !strings.Contains(miss.Error(), "more.tsx") {
		t.Fatalf("a use outside the write set must be refused naming its file: %v", miss)
	}
	if strings.Contains(readRel(t, root, "card.tsx"), "useSession") {
		t.Fatal("refused repoint must not write")
	}

	renamed := run(t, reg, "repoint", map[string]any{
		"from": "hooks.tsx:useAuth", "to": "useSession",
		"paths": []any{"hooks.tsx", "card.tsx", "more.tsx"},
	})
	if !strings.Contains(renamed, "hooks.tsx") || !strings.Contains(renamed, "card.tsx") || !strings.Contains(renamed, "more.tsx") {
		t.Fatalf("repoint answer:\n%s", renamed)
	}
	hooksSrc := readRel(t, root, "hooks.tsx")
	cardSrc := readRel(t, root, "card.tsx")
	moreSrc := readRel(t, root, "more.tsx")
	otherSrc := readRel(t, root, "other.ts")
	if !strings.Contains(hooksSrc, "function useSession") || strings.Contains(hooksSrc, "useAuth") {
		t.Fatalf("declaration was not renamed:\n%s", hooksSrc)
	}
	if !strings.Contains(cardSrc, "useSession as ua") || !strings.Contains(cardSrc, "useSession()") || !strings.Contains(cardSrc, "return ua()") || !strings.Contains(cardSrc, "const useAuth = 1") {
		t.Fatalf("alias and shadow must survive the rename:\n%s", cardSrc)
	}
	if strings.Contains(cardSrc, "useAuth()") || strings.Contains(cardSrc, "{ useAuth ") {
		t.Fatalf("the resolved name was left in place:\n%s", cardSrc)
	}
	if !strings.Contains(moreSrc, "useSession") || strings.Contains(moreSrc, "useAuth") {
		t.Fatalf("the other importer was not renamed:\n%s", moreSrc)
	}
	if !strings.Contains(otherSrc, "function useAuth") || strings.Contains(otherSrc, "useSession") {
		t.Fatalf("a same-named declaration in another file is not this name:\n%s", otherSrc)
	}

	aliasCallers := run(t, reg, "callers_of", map[string]any{"symbol": "hooks.tsx:useSession"})
	if !strings.Contains(aliasCallers, "card.tsx") || !strings.Contains(aliasCallers, "exact") {
		t.Fatalf("aliased call still reaches the renamed hook:\n%s", aliasCallers)
	}
}
