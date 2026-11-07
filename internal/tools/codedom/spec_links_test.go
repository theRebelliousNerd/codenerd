package codedom

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/tools"
)

type fakeConfirmer struct {
	calls [][2]string
	err   error
}

func (f *fakeConfirmer) ConfirmSpecLink(spec, code string) (string, error) {
	f.calls = append(f.calls, [2]string{spec, code})
	if f.err != nil {
		return "", f.err
	}
	return fmt.Sprintf("Confirmed %s %s", spec, code), nil
}

func TestConfirmSpecLinkIsRegisteredWithAWriteEffect(t *testing.T) {
	registry := tools.NewRegistry()
	if err := RegisterAll(registry); err != nil {
		t.Fatal(err)
	}
	tool := registry.Get("confirm_spec_link")
	if tool == nil {
		t.Fatal("confirm_spec_link is not in the codedom catalog")
	}
	if effect, err := tool.DeclaredEffect(); err != nil || effect != tools.EffectWrite {
		t.Fatalf("it persists the link confirmations, so it declares a write: %v %v", effect, err)
	}
	if capability := tools.BuiltinCapability("confirm_spec_link"); capability != tools.CapabilityUnique {
		t.Fatalf("capability manifest: %q", capability)
	}
}

func TestConfirmSpecLinkPassesBothRefsToTheConfirmer(t *testing.T) {
	f := &fakeConfirmer{}
	RegisterSpecLinkConfirmer(f)
	t.Cleanup(func() { RegisterSpecLinkConfirmer(nil) })

	out, err := ConfirmSpecLinkTool().Execute(context.Background(), map[string]any{
		"spec_ref": " Docs/x.md:Install > Linux ", "code_ref": "internal/core.Eval",
	})
	if err != nil || !strings.Contains(out, "Confirmed Docs/x.md:Install > Linux internal/core.Eval") {
		t.Fatalf("out=%q err=%v", out, err)
	}
	if len(f.calls) != 1 || f.calls[0] != [2]string{"Docs/x.md:Install > Linux", "internal/core.Eval"} {
		t.Fatalf("calls: %v", f.calls)
	}

	for _, args := range []map[string]any{{"spec_ref": "Docs/x.md:A"}, {"code_ref": "pkg.F"}, {}} {
		if _, err := ConfirmSpecLinkTool().Execute(context.Background(), args); err == nil || !strings.Contains(err.Error(), "required") {
			t.Errorf("args %v: both refs are required: %v", args, err)
		}
	}
	if len(f.calls) != 1 {
		t.Fatalf("a call without both refs must not reach the confirmer: %v", f.calls)
	}

	f.err = fmt.Errorf("no link between them")
	if _, err := ConfirmSpecLinkTool().Execute(context.Background(), map[string]any{"spec_ref": "a.md:A", "code_ref": "p.F"}); err == nil || !strings.Contains(err.Error(), "no link between") {
		t.Fatalf("the confirmer's refusal must reach the model: %v", err)
	}
}

func TestConfirmSpecLinkWithoutAConfirmerSaysSo(t *testing.T) {
	RegisterSpecLinkConfirmer(nil)
	_, err := ConfirmSpecLinkTool().Execute(context.Background(), map[string]any{"spec_ref": "a.md:A", "code_ref": "p.F"})
	if err == nil || !strings.Contains(err.Error(), "not tracked") {
		t.Fatalf("err: %v", err)
	}
}
