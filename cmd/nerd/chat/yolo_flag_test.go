package chat

import (
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
)

// `nerd --yolo` is session autonomy: the chat it opens resolves choices
// itself and says so to the kernel (yolo_mode), without writing the switch to
// config.json. Until 2026-09-25 the chat never read the flag.
func TestInitChat_YoloFlagIsSessionAutonomy(t *testing.T) {
	m := InitChat(Config{Yolo: true})
	m.Config = config.DefaultUserConfig() // whatever the repo's own config says is not under test
	m.workspace = t.TempDir()

	if !m.yoloEnabled() {
		t.Fatal("the --yolo flag did not turn on autonomy in the chat it launched")
	}
	if m.Config.Yolo {
		t.Fatal("the --yolo flag was written into the persisted config")
	}

	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	m.kernel = k
	m.syncYoloFact()
	if rows, _ := k.Query("yolo_mode"); len(rows) != 1 {
		t.Fatalf("the kernel holds %d yolo_mode facts under --yolo, want 1: the clarification rules still ask", len(rows))
	}

	// /yolo off ends flag-granted autonomy as well as the persisted switch.
	m = m.setYolo(false)
	if m.yoloEnabled() {
		t.Fatal("/yolo off left the --yolo flag's autonomy on")
	}
	if rows, _ := k.Query("yolo_mode"); len(rows) != 0 {
		t.Fatalf("yolo_mode survived /yolo off: %d rows", len(rows))
	}
	if _, err := os.Stat(filepath.Join(m.workspace, ".nerd", "config.json")); err != nil {
		t.Fatalf("/yolo off did not persist its choice: %v", err)
	}

	control := InitChat(Config{})
	control.Config = config.DefaultUserConfig()
	if control.yoloEnabled() {
		t.Fatal("a chat launched without --yolo is autonomous")
	}
}

// --api-key reaches the Cortex boot the chat asks for, as it does for the
// one-shot verbs; the factory uses it only as the legacy fallback.
func TestSharedBootConfig_CarriesLaunchFlags(t *testing.T) {
	m := InitChat(Config{APIKey: "flag-key", DisableSystemShards: []string{"legislator"}})
	got := sharedBootConfig(m.Config, m.DisableSystemShards, m.apiKeyFlag, "/ws")
	if got.APIKey != "flag-key" {
		t.Fatalf("BootConfig.APIKey = %q, want the --api-key value", got.APIKey)
	}
	if len(got.DisableSystemShards) != 1 || got.DisableSystemShards[0] != "legislator" {
		t.Fatalf("BootConfig.DisableSystemShards = %v", got.DisableSystemShards)
	}
	if got.Workspace != "/ws" || got.UserConfigOverride != m.Config {
		t.Fatalf("BootConfig lost the workspace or the loaded config: %+v", got)
	}
}
