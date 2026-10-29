package codemodel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func dumpModel(t *testing.T, f *File) {
	t.Helper()
	t.Logf("parsed=%v err=%v lang=%s", f.Parsed, f.Err, f.Language)
	for _, e := range f.Elements {
		t.Logf("  elem %-12s %-22s recv=%q role=%q exp=%v lines=%d-%d sig=%q doc=%q\n    %q",
			e.Kind, e.Key, e.Receiver, e.Role, e.Exported, e.StartLine, e.EndLine, e.Signature, e.Doc, oneLine(f.Text(&e)))
	}
	for _, imp := range f.Imports {
		t.Logf("  import name=%s spec=%s path=%s resolved=%s level=%d locals=%v",
			imp.Name, imp.Spec, imp.Path, imp.Resolved, imp.Level, imp.Imported)
	}
	for _, c := range f.Calls {
		t.Logf("  call %s.%s jsx=%v bound=%v local=%v -> %s %s.%s line=%d",
			c.Qualifier, c.Name, c.JSX, c.Bound, c.Local, c.TargetFile, c.TargetRecv, c.TargetName, c.Line)
	}
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	if len(s) > 80 {
		return s[:80] + "..."
	}
	return s
}

func elem(t *testing.T, f *File, key string) *Element {
	t.Helper()
	e := f.Element(key)
	if e == nil {
		dumpModel(t, f)
		t.Fatalf("missing element %s", key)
	}
	return e
}

func mustParse(t *testing.T, path, abs, root, src string) *File {
	t.Helper()
	f, ok := ParseRoot(path, abs, root, src)
	if !ok || f == nil {
		t.Fatalf("ParseRoot %s: language has no element model", path)
	}
	return f
}

func TestPythonModule(t *testing.T) {
	root := t.TempDir()
	write := func(rel, src string) string {
		t.Helper()
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		return abs
	}
	write("pkg/services.py", "class User:\n    def get(cls, id):\n        return id\n")
	write("billing/invoice.py", "class Invoice:\n    pass\n")
	src := `"""Users service."""

from __future__ import annotations

from .services import User
from ..billing.invoice import Invoice
import os.path as osp

@router.get("/users/{id}")
class UserView:
    """A view of a user."""

    def __init__(self, user: User) -> None:
        self.user = user

    @staticmethod
    def _hidden(self):
        return self

    async def fetch(self, request):
        """Load the user."""
        return await User.get(request.id)

class Outer:
    class Inner:
        def ping(self):
            return 1

# comment belongs to helper
def helper(n):
    return n

def shadow():
    User = "local"
    return User

def real():
    return User.get(1)

def comp(xs):
    y = [use_auth for use_auth in xs]
    return use_auth()

def use_auth():
    return 1

ALL_CAPS = 1
_private = 2

__all__ = ["UserView", "helper", "ALL_CAPS"]
`
	abs := write("pkg/views.py", src)
	f := mustParse(t, "pkg/views.py", abs, root, src)
	if !f.Parsed {
		dumpModel(t, f)
		t.Fatalf("parse: %v", f.Err)
	}
	view := elem(t, f, "UserView")
	if !strings.Contains(f.Text(view), `@router.get("/users/{id}")`) || !strings.Contains(f.Text(view), "class UserView") {
		t.Fatalf("decorator belongs to the class span:\n%s", f.Text(view))
	}
	if view.Doc != "A view of a user." || !view.Exported || view.Kind != KindClass {
		dumpModel(t, f)
		t.Fatalf("UserView doc/export/kind: %+v", view)
	}
	fetch := elem(t, f, "UserView.fetch")
	if !strings.Contains(fetch.Signature, "async def fetch") || fetch.Doc != "Load the user." || !fetch.Exported || fetch.Kind != KindMethod || fetch.Receiver != "UserView" {
		dumpModel(t, f)
		t.Fatalf("fetch: sig=%q doc=%q exp=%v kind=%s recv=%s", fetch.Signature, fetch.Doc, fetch.Exported, fetch.Kind, fetch.Receiver)
	}
	if !strings.Contains(f.Text(fetch), "async def fetch") || strings.Contains(f.Text(fetch), "@staticmethod") {
		t.Fatalf("fetch span:\n%s", f.Text(fetch))
	}
	hidden := elem(t, f, "UserView._hidden")
	if hidden.Exported || !strings.Contains(f.Text(hidden), "@staticmethod") {
		t.Fatalf("hidden: exp=%v text=%s", hidden.Exported, f.Text(hidden))
	}
	if elem(t, f, "Outer.Inner").Kind != KindClass || elem(t, f, "Outer.Inner.ping").Receiver != "Outer.Inner" {
		dumpModel(t, f)
		t.Fatal("nested class ref")
	}
	helper := elem(t, f, "helper")
	if !strings.HasPrefix(f.Text(helper), "# comment belongs to helper") || !helper.Exported {
		t.Fatalf("helper doc comment / export:\n%s exp=%v", f.Text(helper), helper.Exported)
	}
	if elem(t, f, "ALL_CAPS").Kind != KindConst || !elem(t, f, "ALL_CAPS").Exported {
		t.Fatal("ALL_CAPS")
	}
	if elem(t, f, "_private").Exported || elem(t, f, "use_auth").Exported {
		t.Fatal("names absent from __all__ are not exported")
	}
	h := f.Header()
	if h == nil || !strings.Contains(f.Text(h), "Users service") || !strings.Contains(f.Text(h), "import os.path as osp") || strings.Contains(f.Text(h), "class UserView") {
		dumpModel(t, f)
		t.Fatalf("header: %v", h)
	}
	var userImp, invImp *Import
	for i := range f.Imports {
		imp := &f.Imports[i]
		if imp.Spec == ".services" {
			userImp = imp
		}
		if imp.Spec == "..billing.invoice" {
			invImp = imp
		}
	}
	if userImp == nil || userImp.Resolved != "pkg/services.py" || userImp.Level != 1 || userImp.Path != "pkg/services.py" {
		dumpModel(t, f)
		t.Fatalf("services import: %+v", userImp)
	}
	if invImp == nil || invImp.Resolved != "billing/invoice.py" || invImp.Level != 2 {
		dumpModel(t, f)
		t.Fatalf("invoice import: %+v", invImp)
	}
	var got *Call
	for i := range f.Calls {
		c := &f.Calls[i]
		if c.Name == "get" && c.Qualifier == "User" && c.Line == lineOf(src, "return await User.get") {
			got = c
		}
	}
	if got == nil || !got.Bound || got.Local || got.TargetFile != "pkg/services.py" || got.TargetRecv != "User" || got.TargetName != "get" {
		dumpModel(t, f)
		t.Fatalf("User.get call: %+v", got)
	}
	var compCall *Call
	for i := range f.Calls {
		c := &f.Calls[i]
		if c.Name == "use_auth" && c.Qualifier == "" {
			compCall = c
		}
	}
	if compCall == nil || !compCall.Bound || compCall.Local || compCall.TargetName != "use_auth" {
		dumpModel(t, f)
		t.Fatalf("comp must call the module function, not the loop variable: %+v", compCall)
	}
	// The comprehension variable is not a use of the module function.
	sites, err := RenameSites(f, "use_auth", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, site := range sites {
		text := src[site.Start:site.End]
		if text != "use_auth" {
			t.Fatalf("site %d is %q", site.Start, text)
		}
	}
	out, err := ApplyRename(f, "use_session", sites)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.File.Source, "for use_session in") || !strings.Contains(out.File.Source, "for use_auth in") {
		t.Fatalf("comprehension variable must stay:\n%s", out.File.Source)
	}
	if !strings.Contains(out.File.Source, "def use_session") || !strings.Contains(out.File.Source, "return use_session()") {
		t.Fatalf("declaration and real call rename:\n%s", out.File.Source)
	}
	if !strings.Contains(out.File.Source, `User = "local"`) {
		t.Fatal("unrelated local disappeared")
	}
}

func lineOf(src, frag string) int {
	i := strings.Index(src, frag)
	if i < 0 {
		return -1
	}
	return strings.Count(src[:i], "\n") + 1
}

func TestReplacePythonMethod(t *testing.T) {
	src := "class Box:\n    def keep(self):\n        return 1\n\n    def change(self):\n        return 1\n"
	f, ok := Parse("box.py", src)
	if !ok || !f.Parsed {
		t.Fatal(f.Err)
	}
	change := elem(t, f, "Box.change")
	keep := elem(t, f, "Box.keep")
	keepText := f.Text(keep)
	neu := strings.Replace(f.Text(change), "return 1", "return 2", 1)
	out, err := Apply(f, Change{Start: change.Start, End: change.End, Text: neu}, []string{change.Key}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !out.File.Parsed {
		t.Fatal(out.File.Err)
	}
	if out.File.Text(out.File.Element("Box.keep")) != keepText {
		t.Fatalf("sibling method changed:\n%s", out.File.Source)
	}
	if !strings.Contains(out.File.Text(out.File.Element("Box.change")), "return 2") {
		t.Fatalf("method not replaced:\n%s", out.File.Source)
	}
	// Bytes outside the class are untouched (there are none before it). The
	// gap between the methods is not part of either method.
	if !strings.Contains(out.File.Source, "def keep(self):\n        return 1\n") {
		t.Fatalf("keep bytes:\n%s", out.File.Source)
	}
}

func TestTSXComponent(t *testing.T) {
	root := t.TempDir()
	write := func(rel, src string) {
		t.Helper()
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	hooks := "export function useAuth() {\n  return 1;\n}\n"
	write("hooks.tsx", hooks)
	write("Panel.tsx", "export function Panel(props: { id: string }) {\n  return <span>{props.id}</span>;\n}\n")
	write("other.ts", "export function useAuth() {\n  return 2;\n}\n")
	card := `import { useAuth } from "./hooks";
import Panel from "./Panel";
import { useAuth as ua } from "./hooks";

export interface UserCardProps {
  id: string;
}

/** A card. */
export function UserCard({ id }: UserCardProps) {
  const session = useAuth();
  return <Panel id={id}><span>hi</span></Panel>;
}

export const Badge = () => <span>ok</span>;

export default function App() {
  return <UserCard id="a" />;
}

export function Shadow() {
  const useAuth = 1;
  return useAuth;
}

export function Alias() {
  return ua();
}
`
	cardAbs := filepath.Join(root, "card.tsx")
	if err := os.WriteFile(cardAbs, []byte(card), 0o644); err != nil {
		t.Fatal(err)
	}
	hooksAbs := filepath.Join(root, "hooks.tsx")
	otherAbs := filepath.Join(root, "other.ts")
	f := mustParse(t, "card.tsx", cardAbs, root, card)
	if !f.Parsed {
		dumpModel(t, f)
		t.Fatal(f.Err)
	}
	props := elem(t, f, "UserCardProps")
	if props.Kind != KindInterface || !props.Exported {
		dumpModel(t, f)
		t.Fatalf("props: %+v", props)
	}
	cardEl := elem(t, f, "UserCard")
	if cardEl.Kind != KindFunction || cardEl.Role != "component" || cardEl.Doc != "A card." || !cardEl.Exported {
		dumpModel(t, f)
		t.Fatalf("UserCard: kind=%s role=%s doc=%q exp=%v", cardEl.Kind, cardEl.Role, cardEl.Doc, cardEl.Exported)
	}
	if !strings.HasPrefix(f.Text(cardEl), "/** A card. */") {
		t.Fatalf("jsdoc span:\n%s", f.Text(cardEl))
	}
	badge := elem(t, f, "Badge")
	if badge.Role != "component" || badge.Kind != KindFunction {
		dumpModel(t, f)
		t.Fatalf("Badge: %+v", badge)
	}
	app := elem(t, f, "App")
	if app.Role != "component" || !app.defaultExport || app.namedExport {
		dumpModel(t, f)
		t.Fatalf("App: role=%s default=%v named=%v", app.Role, app.defaultExport, app.namedExport)
	}
	var panel, widget, span int
	for _, c := range f.Calls {
		if c.JSX && c.Name == "Panel" {
			panel++
			if !c.Bound || c.TargetFile != "Panel.tsx" {
				dumpModel(t, f)
				t.Fatalf("Panel jsx: %+v", c)
			}
		}
		if c.JSX && c.Name == "UserCard" {
			widget++
		}
		if c.Name == "span" {
			span++
		}
	}
	if panel != 1 || widget != 1 || span != 0 {
		dumpModel(t, f)
		t.Fatalf("jsx calls panel=%d userCard=%d span=%d", panel, widget, span)
	}
	var auth *Call
	for i := range f.Calls {
		c := &f.Calls[i]
		if c.Name == "useAuth" && !c.JSX {
			auth = c
		}
	}
	if auth == nil || !auth.Bound || auth.Local || auth.TargetFile != "hooks.tsx" || auth.TargetName != "useAuth" {
		dumpModel(t, f)
		t.Fatalf("useAuth call: %+v", auth)
	}
	var hooksImp, panelImp, aliasImp *Import
	for i := range f.Imports {
		imp := &f.Imports[i]
		switch {
		case imp.Spec == "./hooks" && len(imp.Imported) > 0 && !imp.Imported[0].Aliased:
			hooksImp = imp
		case imp.Spec == "./Panel":
			panelImp = imp
		case imp.Spec == "./hooks" && len(imp.Imported) > 0 && imp.Imported[0].Aliased:
			aliasImp = imp
		}
	}
	if hooksImp == nil || hooksImp.Resolved != "hooks.tsx" || hooksImp.Path != "hooks.tsx" {
		dumpModel(t, f)
		t.Fatalf("hooks import: %+v", hooksImp)
	}
	if panelImp == nil || panelImp.Resolved != "Panel.tsx" {
		dumpModel(t, f)
		t.Fatalf("panel import: %+v", panelImp)
	}
	if aliasImp == nil || !aliasImp.Imported[0].Aliased || aliasImp.Imported[0].Local != "ua" {
		dumpModel(t, f)
		t.Fatalf("alias import: %+v", aliasImp)
	}

	hf := mustParse(t, "hooks.tsx", hooksAbs, root, hooks)
	if elem(t, hf, "useAuth").Role != "hook" {
		dumpModel(t, hf)
		t.Fatal("useAuth is a hook")
	}
	of := mustParse(t, "other.ts", otherAbs, root, "export function useAuth() {\n  return 2;\n}\n")
	sites, err := RenameSites(hf, "useAuth", []*File{f, of})
	if err != nil {
		t.Fatal(err)
	}
	byFile := map[string]int{}
	for _, site := range sites {
		byFile[site.Path]++
		if site.Path == "other.ts" {
			t.Fatal("a different useAuth must not be renamed")
		}
	}
	if byFile["hooks.tsx"] == 0 || byFile["card.tsx"] == 0 {
		t.Fatalf("sites: %+v", sites)
	}
	hout, err := ApplyRename(hf, "useSession", sites)
	if err != nil {
		t.Fatal(err)
	}
	cout, err := ApplyRename(f, "useSession", sites)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(hout.File.Source, "function useSession") {
		t.Fatalf("decl:\n%s", hout.File.Source)
	}
	cs := cout.File.Source
	if !strings.Contains(cs, "useSession") || !strings.Contains(cs, "useSession as ua") {
		t.Fatalf("import specifier:\n%s", cs)
	}
	if strings.Contains(cs, "const useSession") || !strings.Contains(cs, "const useAuth = 1") {
		t.Fatalf("shadowed local must stay:\n%s", cs)
	}
	if strings.Contains(cs, "ua(") && strings.Contains(cs, "useSession()") {
		// ua() stays; the real call changes
	}
	if !strings.Contains(cs, "return ua()") {
		t.Fatalf("alias local stays:\n%s", cs)
	}
	if strings.Contains(cs, "useAuth()") {
		t.Fatalf("the real call must change:\n%s", cs)
	}
	if of.Source != "export function useAuth() {\n  return 2;\n}\n" && strings.Contains(of.Source, "useSession") {
		t.Fatal("other file changed")
	}
}

func TestTSPathAlias(t *testing.T) {
	root := t.TempDir()
	write := func(rel, src string) {
		t.Helper()
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("base.json", "{\n  \"compilerOptions\": {\n    \"baseUrl\": \".\",\n    \"paths\": { \"@old/*\": [\"old/*\"] }\n  }\n}\n")
	write("tsconfig.json", "{\n  \"extends\": \"./base.json\",\n  \"compilerOptions\": {\n    \"baseUrl\": \".\",\n    \"paths\": { \"@app/*\": [\"src/*\"] }\n  }\n}\n")
	write("src/widget.ts", "export class Widget {\n  constructor(public id: string) {}\n}\n")
	write("src/config.ts", "export function readConfig(): string { return \"x\"; }\n")
	src := `import { Widget } from "@app/widget";
import { readConfig } from "@app/config";

export interface Service {
  name: string;
}

export type ID = string;

export enum Mode {
  On = "on",
  Off = "off",
}

export namespace Api {
  export function ping(): ID {
    return readConfig();
  }
}

export const load = (id: ID): Widget => {
  return new Widget(id);
};
`
	abs := filepath.Join(root, "src", "service.ts")
	write("src/service.ts", src)
	f := mustParse(t, "src/service.ts", abs, root, src)
	if !f.Parsed {
		dumpModel(t, f)
		t.Fatal(f.Err)
	}
	if elem(t, f, "Service").Kind != KindInterface || elem(t, f, "ID").Kind != KindType || elem(t, f, "Mode").Kind != KindEnum {
		dumpModel(t, f)
		t.Fatal("type elements")
	}
	if elem(t, f, "Api").Kind != KindNamespace || elem(t, f, "Api.ping").Kind != KindFunction || elem(t, f, "Api.ping").Receiver != "Api" {
		dumpModel(t, f)
		t.Fatal("namespace function")
	}
	var widget, cfg *Import
	for i := range f.Imports {
		imp := &f.Imports[i]
		if imp.Spec == "@app/widget" {
			widget = imp
		}
		if imp.Spec == "@app/config" {
			cfg = imp
		}
	}
	if widget == nil || widget.Resolved != "src/widget.ts" || widget.Path != "src/widget.ts" {
		dumpModel(t, f)
		t.Fatalf("widget: %+v", widget)
	}
	if cfg == nil || cfg.Resolved != "src/config.ts" {
		dumpModel(t, f)
		t.Fatalf("config: %+v", cfg)
	}
	var read *Call
	for i := range f.Calls {
		c := &f.Calls[i]
		if c.Name == "readConfig" {
			read = c
		}
	}
	if read == nil || !read.Bound || read.TargetFile != "src/config.ts" || read.TargetName != "readConfig" {
		dumpModel(t, f)
		t.Fatalf("readConfig: %+v", read)
	}
	var ctor *Call
	for i := range f.Calls {
		c := &f.Calls[i]
		if c.Name == "Widget" {
			ctor = c
		}
	}
	if ctor == nil || !ctor.Bound || ctor.TargetFile != "src/widget.ts" {
		dumpModel(t, f)
		t.Fatalf("Widget: %+v", ctor)
	}
}
