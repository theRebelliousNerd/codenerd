package tools

import (
	"context"
	"testing"
)

func TestCampaignCheckFrom_WithoutACheck(t *testing.T) {
	if _, ok := CampaignCheckFrom(context.Background()); ok {
		t.Fatal("a context with no check must not yield one")
	}
	if _, ok := CampaignCheckFrom(nil); ok {
		t.Fatal("a nil context must not yield a check")
	}
}

func TestCampaignCheckFrom_RoundTrip(t *testing.T) {
	argv := []string{"go", "version"}
	ctx := WithCampaignCheck(context.Background(), CampaignCheck{
		CampaignID: "/campaign_c",
		TaskID:     "/task_1",
		Argv:       argv,
	})
	// The carrier copies: mutating the caller's slice must not rewrite the check.
	argv[0] = "mutated"
	got, ok := CampaignCheckFrom(ctx)
	if !ok {
		t.Fatal("a WithCampaignCheck context yielded no check")
	}
	if got.CampaignID != "/campaign_c" || got.TaskID != "/task_1" {
		t.Fatalf("check = %+v, want the stored campaign and task", got)
	}
	if len(got.Argv) != 2 || got.Argv[0] != "go" || got.Argv[1] != "version" {
		t.Fatalf("argv = %v, want [go version]", got.Argv)
	}
	// What the reader returns is a copy: a caller cannot rewrite the record.
	got.Argv[0] = "mutated"
	again, _ := CampaignCheckFrom(ctx)
	if again.Argv[0] != "go" {
		t.Fatal("the carrier handed out its own storage")
	}
}

func TestCampaignCheckFrom_EmptyArgvIsNoCheck(t *testing.T) {
	for _, argv := range [][]string{nil, {}, {""}} {
		ctx := WithCampaignCheck(context.Background(), CampaignCheck{
			CampaignID: "/c",
			TaskID:     "/t",
			Argv:       argv,
		})
		if _, ok := CampaignCheckFrom(ctx); ok {
			t.Fatalf("argv %q must fail closed as no check", argv)
		}
	}
}
