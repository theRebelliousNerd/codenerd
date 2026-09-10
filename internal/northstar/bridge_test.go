package northstar

import (
	"codenerd/internal/atomicfile"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newBridgeStore(t *testing.T) (*Store, string) {
	t.Helper()
	nerdDir := t.TempDir()
	store, err := NewStore(nerdDir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store, nerdDir
}

func writeWizardJSON(t *testing.T, nerdDir string, doc WizardDocument) {
	t.Helper()
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal wizard doc: %v", err)
	}
	path := filepath.Join(nerdDir, VisionJSONFileName)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func sampleWizardVision() *Vision {
	doc := sampleWizardDoc()
	return doc.ToVision()
}

func sampleWizardDoc() WizardDocument {
	return WizardDocument{
		Mission: "Make logic the executive",
		Problem: "LLMs improvise control flow",
		Vision:  "A kernel that decides and an LLM that creates",
		Personas: []WizardPersona{
			{Name: "Operator", PainPoints: []string{"cannot audit decisions"}, Needs: []string{"traceable reasoning"}},
		},
		Capabilities: []WizardCapability{
			{Description: "Project vision into the kernel", Timeline: "now", Priority: "critical", Serves: []string{"Operator"}},
		},
		Risks: []WizardRisk{
			{Description: "Vision drifts from code", Likelihood: "high", Impact: "high", Mitigation: "Reconcile on every boot"},
		},
		Requirements: []WizardRequirement{
			{ID: "REQ-A", Type: "functional", Description: "Single authority", Priority: "must-have",
				Supports: []string{"cap_1"}, Addresses: []string{"risk_1"}},
		},
		Constraints: []string{"No network at boot"},
	}
}

func TestSyncVisionAuthority_WhenOnlyJSONExists_ShouldImportIntoStore(t *testing.T) {
	store, nerdDir := newBridgeStore(t)
	writeWizardJSON(t, nerdDir, sampleWizardDoc())

	res, err := SyncVisionAuthority(store, nerdDir)
	if err != nil {
		t.Fatalf("SyncVisionAuthority: %v", err)
	}
	if res.Direction != SyncImported {
		t.Fatalf("direction = %q, want %q", res.Direction, SyncImported)
	}

	stored, err := store.LoadVision()
	if err != nil {
		t.Fatalf("LoadVision: %v", err)
	}
	if stored == nil {
		t.Fatal("store still has no vision after import; the dual-store bug is back")
	}
	if stored.Mission != "Make logic the executive" {
		t.Errorf("mission = %q", stored.Mission)
	}
	if len(stored.Capabilities) != 1 || stored.Capabilities[0].ID != "cap_1" {
		t.Errorf("capabilities = %+v, want one with synthesised id cap_1", stored.Capabilities)
	}
	if _, err := os.Stat(filepath.Join(nerdDir, VisionMangleFileName)); err != nil {
		t.Errorf("import did not refresh %s: %v", VisionMangleFileName, err)
	}
}

func TestSyncVisionAuthority_WhenOnlyStoreHasVision_ShouldExportBothSurfaces(t *testing.T) {
	store, nerdDir := newBridgeStore(t)
	vision := sampleWizardVision()
	if err := store.SaveVision(vision); err != nil {
		t.Fatalf("SaveVision: %v", err)
	}

	res, err := SyncVisionAuthority(store, nerdDir)
	if err != nil {
		t.Fatalf("SyncVisionAuthority: %v", err)
	}
	if res.Direction != SyncExported {
		t.Fatalf("direction = %q, want %q", res.Direction, SyncExported)
	}

	roundTripped, err := LoadVisionJSON(nerdDir)
	if err != nil {
		t.Fatalf("LoadVisionJSON: %v", err)
	}
	if roundTripped == nil {
		t.Fatal("export produced no readable JSON surface")
	}
	if !VisionsEquivalent(roundTripped, vision) {
		t.Errorf("exported JSON does not round-trip:\n got %+v\nwant %+v", roundTripped, vision)
	}

	mangle, err := os.ReadFile(filepath.Join(nerdDir, VisionMangleFileName))
	if err != nil {
		t.Fatalf("read exported mangle: %v", err)
	}
	if !strings.Contains(string(mangle), "northstar_defined().") {
		t.Errorf("exported mangle lacks northstar_defined():\n%s", mangle)
	}
}

func TestSyncVisionAuthority_WhenSurfacesAgree_ShouldBeNoop(t *testing.T) {
	store, nerdDir := newBridgeStore(t)
	writeWizardJSON(t, nerdDir, sampleWizardDoc())

	if _, err := SyncVisionAuthority(store, nerdDir); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	jsonPath := filepath.Join(nerdDir, VisionJSONFileName)
	before, err := os.Stat(jsonPath)
	if err != nil {
		t.Fatalf("stat json: %v", err)
	}

	res, err := SyncVisionAuthority(store, nerdDir)
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if res.Direction != SyncNoop {
		t.Fatalf("second sync direction = %q, want %q (a converged pair must not churn)", res.Direction, SyncNoop)
	}
	after, err := os.Stat(jsonPath)
	if err != nil {
		t.Fatalf("stat json after: %v", err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Error("second sync rewrote northstar.json; repeated boots would ping-pong import/export")
	}
}

func TestSyncVisionAuthority_WhenJSONIsNewerAndDifferent_ShouldImport(t *testing.T) {
	store, nerdDir := newBridgeStore(t)
	old := sampleWizardVision()
	old.Mission = "Stale mission"
	if err := store.SaveVision(old); err != nil {
		t.Fatalf("SaveVision: %v", err)
	}

	doc := sampleWizardDoc()
	doc.Mission = "Fresh mission from the wizard"
	writeWizardJSON(t, nerdDir, doc)
	future := time.Now().Add(2 * time.Hour)
	jsonPath := filepath.Join(nerdDir, VisionJSONFileName)
	if err := os.Chtimes(jsonPath, future, future); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	res, err := SyncVisionAuthority(store, nerdDir)
	if err != nil {
		t.Fatalf("SyncVisionAuthority: %v", err)
	}
	if res.Direction != SyncImported {
		t.Fatalf("direction = %q, want %q", res.Direction, SyncImported)
	}
	stored, _ := store.LoadVision()
	if stored.Mission != "Fresh mission from the wizard" {
		t.Errorf("mission = %q, want the newer JSON mission", stored.Mission)
	}
	if stored.CreatedAt.IsZero() {
		t.Error("import dropped created_at; the vision's age must survive reconciliation")
	}
}

func TestSyncVisionAuthority_WhenStoreIsNewerAndDifferent_ShouldExport(t *testing.T) {
	store, nerdDir := newBridgeStore(t)
	doc := sampleWizardDoc()
	doc.Mission = "Older mission on disk"
	writeWizardJSON(t, nerdDir, doc)
	past := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(filepath.Join(nerdDir, VisionJSONFileName), past, past); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	fresh := sampleWizardVision()
	fresh.Mission = "Newer mission in the store"
	if err := store.SaveVision(fresh); err != nil {
		t.Fatalf("SaveVision: %v", err)
	}

	res, err := SyncVisionAuthority(store, nerdDir)
	if err != nil {
		t.Fatalf("SyncVisionAuthority: %v", err)
	}
	if res.Direction != SyncExported {
		t.Fatalf("direction = %q, want %q", res.Direction, SyncExported)
	}
	onDisk, err := LoadVisionJSON(nerdDir)
	if err != nil || onDisk == nil {
		t.Fatalf("LoadVisionJSON: %v", err)
	}
	if onDisk.Mission != "Newer mission in the store" {
		t.Errorf("json mission = %q, want the store's newer mission", onDisk.Mission)
	}
}

func TestLoadVisionJSON_WhenMissionEmpty_ShouldNotDefineVision(t *testing.T) {
	nerdDir := t.TempDir()
	writeWizardJSON(t, nerdDir, WizardDocument{Problem: "half-finished wizard run"})

	v, err := LoadVisionJSON(nerdDir)
	if err != nil {
		t.Fatalf("LoadVisionJSON: %v", err)
	}
	if v != nil {
		t.Errorf("a mission-less document defined a vision (%+v); it would flip northstar_defined() on for nothing", v)
	}
}

func TestGuardianInitialize_WhenOnlyWizardJSONExists_ShouldSeeVision(t *testing.T) {
	// The headline P0: the wizard writes JSON, /alignment reads the store.
	// Before the bridge this guardian reported HasVision()==false.
	store, nerdDir := newBridgeStore(t)
	writeWizardJSON(t, nerdDir, sampleWizardDoc())

	g := NewGuardian(store, DefaultGuardianConfig())
	if err := g.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if !g.HasVision() {
		t.Fatal("guardian has no vision after boot even though .nerd/northstar.json defines one")
	}
	if got := g.GetVision().Mission; got != "Make logic the executive" {
		t.Errorf("mission = %q", got)
	}
}

func TestUpdateVision_WhenCalled_ShouldRefreshExportSurfaces(t *testing.T) {
	store, nerdDir := newBridgeStore(t)
	g := NewGuardian(store, DefaultGuardianConfig())
	if err := g.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	vision := sampleWizardVision()
	vision.Mission = "Updated through the guardian"
	if err := g.UpdateVision(vision); err != nil {
		t.Fatalf("UpdateVision: %v", err)
	}

	onDisk, err := LoadVisionJSON(nerdDir)
	if err != nil || onDisk == nil {
		t.Fatalf("LoadVisionJSON: %v", err)
	}
	if onDisk.Mission != "Updated through the guardian" {
		t.Errorf("json mission = %q; UpdateVision must keep the operator surface in step", onDisk.Mission)
	}
	mangle, err := os.ReadFile(filepath.Join(nerdDir, VisionMangleFileName))
	if err != nil {
		t.Fatalf("read mangle: %v", err)
	}
	if !strings.Contains(string(mangle), "Updated through the guardian") {
		t.Error("northstar.mg was not refreshed by UpdateVision")
	}
}

func TestWizardDocument_WhenRoundTripped_ShouldPreserveLinks(t *testing.T) {
	vision := sampleWizardVision()
	roundTrip := WizardDocumentFromVision(vision)
	back := roundTrip.ToVision()
	if !VisionsEquivalent(vision, back) {
		t.Errorf("round trip lost data:\n got %+v\nwant %+v", back, vision)
	}
	if len(back.Requirements) != 1 || len(back.Requirements[0].Supports) != 1 {
		t.Errorf("requirement links lost in round trip: %+v", back.Requirements)
	}
}

// TestVisionSurfacesAreWrittenAtomically is the guard for a failure that is
// permanent rather than costly.
//
// LoadVisionJSON returns a hard error on a parse failure -- it does not fall
// back to an empty vision -- so a truncating write that is interrupted does not
// lose one boot's export. It makes every later load fail, which is fail-closed
// becoming fail-forever, the case internal/atomicfile's doc describes for
// `nerd init`.
//
// The assertion is identity, not content. A truncating write leaves the same
// correct bytes whenever nothing goes wrong, so "the file parses afterwards"
// passes either way, and that is how two durability suites in this repo came to
// pass with their atomic write reverted.
func TestVisionSurfacesAreWrittenAtomically(t *testing.T) {
	for _, tc := range []struct {
		name  string
		file  string
		write func(dir string, v *Vision) error
	}{
		{
			name: "json",
			file: VisionJSONFileName,
			write: func(dir string, v *Vision) error {
				_, err := WriteVisionJSON(dir, v)
				return err
			},
		},
		{
			name:  "mangle",
			file:  VisionMangleFileName,
			write: WriteVisionMangle,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, tc.file)

			first := &Vision{Mission: "the original mission"}
			if err := tc.write(dir, first); err != nil {
				t.Fatalf("first write: %v", err)
			}
			before, err := os.Stat(path)
			if err != nil {
				t.Fatalf("stat: %v", err)
			}
			// atomicfile.Open, not os.Open: this stands in for a reader holding the
			// file across the write, and on Windows a handle without FILE_SHARE_DELETE
			// blocks the replace outright. That is the contract this package exists to
			// provide, so the test states it rather than working around it.
			held, err := atomicfile.Open(path)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			defer func() { _ = held.Close() }()
			originalBytes, err := io.ReadAll(held)
			if err != nil {
				t.Fatalf("read original: %v", err)
			}
			if _, err := held.Seek(0, io.SeekStart); err != nil {
				t.Fatalf("seek: %v", err)
			}

			// Deliberately larger, so a truncate-then-write that failed midway
			// could not fit back what it had just destroyed.
			second := &Vision{Mission: "a substantially longer replacement mission " + strings.Repeat("x", 4096)}
			if err := tc.write(dir, second); err != nil {
				t.Fatalf("second write: %v", err)
			}

			after, err := os.Stat(path)
			if err != nil {
				t.Fatalf("stat after: %v", err)
			}
			if os.SameFile(before, after) {
				t.Error("the write went through the existing file; an interrupted write would " +
					"leave the only copy half-formed, and LoadVisionJSON fails hard on that")
			}

			stillThere, err := io.ReadAll(held)
			if err != nil {
				t.Fatalf("read through the old handle: %v", err)
			}
			if string(stillThere) != string(originalBytes) {
				t.Error("a reader holding the file did not still see the contents it opened")
			}
		})
	}
}

func TestLoadVisionJSONFailsHardOnCorruption(t *testing.T) {
	// This is the property that makes the atomicity above matter. If it ever
	// becomes a soft failure, the argument changes -- and so should the comment
	// on the writers.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, VisionJSONFileName), []byte(`{"mission": "trunc`), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := LoadVisionJSON(dir); err == nil {
		t.Fatal("LoadVisionJSON accepted a truncated file; the writers' atomicity comment is now wrong")
	}
}
