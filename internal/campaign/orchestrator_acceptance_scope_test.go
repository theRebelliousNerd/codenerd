package campaign

import (
	"reflect"
	"testing"
)

func TestCampaignWriteSet_RemediationScope(t *testing.T) {
	workspace := `C:/CodeProjects/codenerd`
	tests := []struct {
		name      string
		writeSets [][]string
		want      []string
	}{
		{
			name: "excludes campaign reports under nerd",
			writeSets: [][]string{
				{`.nerd/campaigns/7b853890/artifacts/task_7b853890_5_0.md`, `Docs/architecture/features/00-INDEX.md`},
			},
			want: []string{`Docs/architecture/features/00-INDEX.md`},
		},
		{
			name: "dedups as-written and lower-cased absolute to one spelling",
			writeSets: [][]string{
				{`Docs/architecture/features/00-INDEX.md`, `c:/codeprojects/codenerd/docs/architecture/features/00-index.md`},
			},
			want: []string{`Docs/architecture/features/00-INDEX.md`},
		},
		{
			name: "dedups case-insensitive relative pair preferring case-preserving",
			writeSets: [][]string{
				{`Docs/architecture/features/README.md`, `docs/architecture/features/readme.md`},
			},
			want: []string{`Docs/architecture/features/README.md`},
		},
		{
			name: "never resolves to a nerd report first",
			writeSets: [][]string{
				{`.nerd/campaigns/7b853890/artifacts/task_7b853890_5_3.md`, `.nerd/campaigns/7b853890/artifacts/task_7b853890_5_4.md`, `Docs/architecture/features/README.md`},
			},
			want: []string{`Docs/architecture/features/README.md`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Campaign{}
			for _, ws := range tt.writeSets {
				c.Phases = append(c.Phases, Phase{Tasks: []Task{{WriteSet: append([]string(nil), ws...)}}})
			}
			got := campaignWriteSet(workspace, c)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("campaignWriteSet() = %q, want %q", got, tt.want)
			}
			for _, p := range got {
				if len(p) >= 6 && (p[:6] == ".nerd/" || p == ".nerd") {
					t.Fatalf("scope contains campaign report %q", p)
				}
			}
		})
	}
}

func TestCanonicalRemediationPath_ExcludesNerd(t *testing.T) {
	got, _ := canonicalRemediationPath(`C:/CodeProjects/codenerd`, `.nerd/campaigns/7b853890/artifacts/task_7b853890_5_0.md`)
	if got != "" {
		t.Fatalf("canonicalRemediationPath(.nerd report) = %q, want empty", got)
	}
}

func TestCampaignWriteSet_AbsoluteFirstPrefersRelative(t *testing.T) {
	workspace := `C:/CodeProjects/codenerd`
	c := &Campaign{Phases: []Phase{{Tasks: []Task{{WriteSet: []string{
		`c:/codeprojects/codenerd/docs/architecture/features/00-index.md`,
		`Docs/architecture/features/00-INDEX.md`,
	}}}}}}
	got := campaignWriteSet(workspace, c)
	want := []string{`Docs/architecture/features/00-INDEX.md`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("absolute-first dedup = %q, want %q", got, want)
	}
}

func TestCampaignWriteSet_LowerFirstPrefersMixedCase(t *testing.T) {
	workspace := `C:/CodeProjects/codenerd`
	c := &Campaign{Phases: []Phase{{Tasks: []Task{{WriteSet: []string{
		`docs/architecture/features/readme.md`,
		`Docs/architecture/features/README.md`,
	}}}}}}
	got := campaignWriteSet(workspace, c)
	want := []string{`Docs/architecture/features/README.md`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lower-first dedup = %q, want %q", got, want)
	}
}

func TestCampaignWriteSet_MixedFirstKeepsMixed(t *testing.T) {
	workspace := `C:/CodeProjects/codenerd`
	c := &Campaign{Phases: []Phase{{Tasks: []Task{{WriteSet: []string{
		`Docs/architecture/features/README.md`,
		`docs/architecture/features/readme.md`,
	}}}}}}
	got := campaignWriteSet(workspace, c)
	want := []string{`Docs/architecture/features/README.md`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mixed-first dedup = %q, want %q", got, want)
	}
}

func TestCampaignWriteSet_SortsMultiple(t *testing.T) {
	workspace := `C:/CodeProjects/codenerd`
	c := &Campaign{Phases: []Phase{{Tasks: []Task{{WriteSet: []string{
		`Docs/architecture/features/README.md`,
		`Docs/architecture/features/00-INDEX.md`,
		`adr/ADR-001-features-scope.md`,
	}}}}}}
	got := campaignWriteSet(workspace, c)
	want := []string{`adr/ADR-001-features-scope.md`, `Docs/architecture/features/00-INDEX.md`, `Docs/architecture/features/README.md`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sorted scope = %q, want %q", got, want)
	}
}

func TestCanonicalRemediationPath_EdgeBranches(t *testing.T) {
	workspace := `C:/CodeProjects/codenerd`
	tests := []struct {
		name      string
		workspace string
		raw       string
		want      string
		wantAbs   bool
	}{
		{name: "empty", workspace: workspace, raw: ``, want: ``, wantAbs: false},
		{name: "blank", workspace: workspace, raw: `   `, want: ``, wantAbs: false},
		{name: "nul byte", workspace: workspace, raw: "a\x00b", want: ``, wantAbs: false},
		{name: "glob star", workspace: workspace, raw: `docs/*.md`, want: ``, wantAbs: false},
		{name: "glob question", workspace: workspace, raw: `docs/?.md`, want: ``, wantAbs: false},
		{name: "glob bracket", workspace: workspace, raw: `docs/[abc].md`, want: ``, wantAbs: false},
		{name: "absolute with empty workspace", workspace: ``, raw: `C:/CodeProjects/codenerd/docs/a.md`, want: ``, wantAbs: true},
		{name: "absolute with blank workspace", workspace: `   `, raw: `C:/CodeProjects/codenerd/docs/a.md`, want: ``, wantAbs: true},
		{name: "absolute on other volume stays out", workspace: workspace, raw: `D:/other/file.md`, want: ``, wantAbs: true},
		{name: "dot", workspace: workspace, raw: `.`, want: ``, wantAbs: false},
		{name: "dotdot", workspace: workspace, raw: `..`, want: ``, wantAbs: false},
		{name: "outside workspace", workspace: workspace, raw: `../outside.md`, want: ``, wantAbs: false},
		{name: "absolute outside workspace", workspace: workspace, raw: `C:/other/file.md`, want: ``, wantAbs: true},
		{name: "nerd bare", workspace: workspace, raw: `.nerd`, want: ``, wantAbs: false},
		{name: "nerd nested", workspace: workspace, raw: `.nerd/campaigns/x/artifacts/t.md`, want: ``, wantAbs: false},
		{name: "relative file", workspace: workspace, raw: `Docs/architecture/features/00-INDEX.md`, want: `Docs/architecture/features/00-INDEX.md`, wantAbs: false},
		{name: "absolute file relativizes", workspace: workspace, raw: `C:/CodeProjects/codenerd/Docs/architecture/features/00-INDEX.md`, want: `Docs/architecture/features/00-INDEX.md`, wantAbs: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, fromAbs := canonicalRemediationPath(tt.workspace, tt.raw)
			if got != tt.want || fromAbs != tt.wantAbs {
				t.Fatalf("canonicalRemediationPath(%q, %q) = (%q, %v), want (%q, %v)", tt.workspace, tt.raw, got, fromAbs, tt.want, tt.wantAbs)
			}
		})
	}
}

func TestIsWindowsAbs(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{``, false},
		{`ab`, false},
		{`a`, false},
		{`1:/a`, false},
		{`/a`, false},
		{`c:a`, false},
		{`c:/a`, true},
		{`C:/a`, true},
		{`C:\a`, true},
		{`cc:/a`, false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := isWindowsAbs(tt.path); got != tt.want {
				t.Fatalf("isWindowsAbs(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
