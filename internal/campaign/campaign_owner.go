package campaign

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ErrCampaignOwned is returned when another orchestrator already publishes the
// campaign. Two owners would each write snapshots and journal records from
// their own memory, and the later save silently erases the other's progress.
var ErrCampaignOwned = errors.New("campaign is owned by another orchestrator")

// campaignOwnerLock is the cross-process single-publisher lock of one
// campaign: an advisory lock on campaigns/<id>.owner held for the life of the
// orchestrator that publishes the campaign. The OS drops it when the holder
// exits, however it exits. o.mu alone cannot do this: a CLI resume, the chat
// and the system campaign runner are separate orchestrators, often in
// separate processes, and the system runner picks up any campaign the
// snapshot calls active.
type campaignOwnerLock struct {
	campaignID string
	file       *os.File
}

func campaignOwnerPath(campaignsDir, campaignID string) string {
	return filepath.Join(campaignsDir, sanitizeCampaignID(campaignID)+".owner")
}

// acquireOwnershipLocked makes this orchestrator the single publisher of
// campaignID. Holding it already is a no-op; holding another campaign's lock
// releases that one first. The caller holds o.mu.
func (o *Orchestrator) acquireOwnershipLocked(campaignID string) error {
	if o.owner != nil && sanitizeCampaignID(o.owner.campaignID) == sanitizeCampaignID(campaignID) {
		return nil
	}
	o.releaseOwnershipLocked()

	dir := filepath.Join(o.nerdDir, "campaigns")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("campaign ownership: %w", err)
	}
	path := campaignOwnerPath(dir, campaignID)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("campaign ownership: %w", err)
	}
	ok, err := tryLockFile(f)
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("campaign ownership: lock %s: %w", path, err)
	}
	if !ok {
		_ = f.Close()
		pid, _ := os.ReadFile(path)
		return fmt.Errorf("%w: %s (pid %s holds %s)", ErrCampaignOwned, campaignID, strings.TrimSpace(string(pid)), path)
	}
	if err := f.Truncate(0); err == nil {
		_, _ = f.WriteAt([]byte(strconv.Itoa(os.Getpid())), 0)
	}
	o.owner = &campaignOwnerLock{campaignID: campaignID, file: f}
	return nil
}

// releaseOwnershipLocked drops the ownership lock, if held. The caller holds o.mu.
func (o *Orchestrator) releaseOwnershipLocked() {
	if o.owner == nil {
		return
	}
	_ = unlockFile(o.owner.file)
	_ = o.owner.file.Close()
	o.owner = nil
}
