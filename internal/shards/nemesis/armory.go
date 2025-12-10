package nemesis

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"codenerd/internal/logging"
)

// Armory persists successful attack tools for regression testing.
// Just as codeNERD learns preferences, Nemesis learns weaknesses.
// Every future build must survive all attacks in the Armory.
type Armory struct {
	attacks   []ArmoryAttack
	stats     ArmoryStats
	storePath string
	mu        sync.RWMutex
}

// ArmoryAttack represents a persisted attack tool.
type ArmoryAttack struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Category      string    `json:"category"`       // concurrency, resource, logic, integration
	Vulnerability string    `json:"vulnerability"`  // what invariant it violates
	Specification string    `json:"specification"`  // tool generation prompt
	BinaryPath    string    `json:"binary_path"`    // path to compiled attack tool
	TargetPatch   string    `json:"target_patch"`   // original patch that it broke
	CreatedAt     time.Time `json:"created_at"`
	LastSuccess   time.Time `json:"last_success"`   // last time it found a bug
	SuccessCount  int       `json:"success_count"`  // how many bugs it's found
	RunCount      int       `json:"run_count"`      // how many times it's been run
}

// ArmoryStats tracks overall Armory effectiveness.
type ArmoryStats struct {
	TotalAttacks     int       `json:"total_attacks"`
	ActiveAttacks    int       `json:"active_attacks"`    // attacks that found bugs recently
	StaleAttacks     int       `json:"stale_attacks"`     // attacks that haven't found bugs in 30 days
	TotalBugsFound   int       `json:"total_bugs_found"`
	LastUpdated      time.Time `json:"last_updated"`
}

// NewArmory creates a new Armory instance.
func NewArmory(nerdDir string) *Armory {
	storePath := filepath.Join(nerdDir, "nemesis", "armory.json")
	logging.Shards("Initializing Armory at: %s", storePath)

	armory := &Armory{
		attacks:   make([]ArmoryAttack, 0),
		storePath: storePath,
	}

	// Load existing attacks
	if err := armory.load(); err != nil {
		logging.ShardsDebug("No existing Armory found: %v", err)
	}

	return armory
}

// AddAttack adds a successful attack to the Armory.
func (a *Armory) AddAttack(attack ArmoryAttack) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Generate ID if not set
	if attack.ID == "" {
		attack.ID = generateAttackID(attack.Name)
	}

	// Check if attack already exists
	for i, existing := range a.attacks {
		if existing.Name == attack.Name {
			// Update existing attack
			a.attacks[i].SuccessCount++
			a.attacks[i].LastSuccess = time.Now()
			logging.Shards("Armory: updated existing attack %s (success_count=%d)", attack.Name, a.attacks[i].SuccessCount)
			_ = a.save()
			return
		}
	}

	// Add new attack
	attack.SuccessCount = 1
	attack.LastSuccess = time.Now()
	attack.RunCount = 1
	a.attacks = append(a.attacks, attack)
	a.stats.TotalAttacks++
	a.stats.TotalBugsFound++
	a.stats.LastUpdated = time.Now()

	logging.Shards("Armory: added new attack %s (%s)", attack.Name, attack.Category)
	_ = a.save()
}

// GetRegressionAttacks returns attacks that should be run as regression tests.
func (a *Armory) GetRegressionAttacks() []ArmoryAttack {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Return all non-stale attacks
	active := make([]ArmoryAttack, 0)
	staleThreshold := time.Now().Add(-30 * 24 * time.Hour) // 30 days

	for _, attack := range a.attacks {
		if attack.LastSuccess.After(staleThreshold) || attack.SuccessCount > 0 {
			active = append(active, attack)
		}
	}

	logging.ShardsDebug("Armory: returning %d regression attacks", len(active))
	return active
}

// GetAttackByName retrieves an attack by name.
func (a *Armory) GetAttackByName(name string) (ArmoryAttack, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	for _, attack := range a.attacks {
		if attack.Name == name {
			return attack, true
		}
	}
	return ArmoryAttack{}, false
}

// RecordRun records that an attack was run.
func (a *Armory) RecordRun(name string, foundBug bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for i, attack := range a.attacks {
		if attack.Name == name {
			a.attacks[i].RunCount++
			if foundBug {
				a.attacks[i].SuccessCount++
				a.attacks[i].LastSuccess = time.Now()
				a.stats.TotalBugsFound++
			}
			break
		}
	}

	a.stats.LastUpdated = time.Now()
	_ = a.save()
}

// GetStats returns Armory statistics.
func (a *Armory) GetStats() ArmoryStats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Recalculate active/stale counts
	staleThreshold := time.Now().Add(-30 * 24 * time.Hour)
	active := 0
	stale := 0

	for _, attack := range a.attacks {
		if attack.LastSuccess.After(staleThreshold) {
			active++
		} else {
			stale++
		}
	}

	stats := a.stats
	stats.ActiveAttacks = active
	stats.StaleAttacks = stale
	stats.TotalAttacks = len(a.attacks)

	return stats
}

// PruneStaleAttacks removes attacks that haven't found bugs in a long time.
func (a *Armory) PruneStaleAttacks(daysStale int) int {
	a.mu.Lock()
	defer a.mu.Unlock()

	threshold := time.Now().Add(-time.Duration(daysStale) * 24 * time.Hour)
	pruned := 0

	active := make([]ArmoryAttack, 0)
	for _, attack := range a.attacks {
		if attack.LastSuccess.After(threshold) || attack.SuccessCount >= 3 {
			// Keep if recent success or historically effective
			active = append(active, attack)
		} else {
			logging.Shards("Armory: pruning stale attack %s (last_success=%v)", attack.Name, attack.LastSuccess)
			pruned++
		}
	}

	a.attacks = active
	a.stats.StaleAttacks = 0
	a.stats.LastUpdated = time.Now()

	if pruned > 0 {
		_ = a.save()
	}

	return pruned
}

// ExportForRegression exports attacks in a format suitable for CI/CD integration.
func (a *Armory) ExportForRegression() []map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	exports := make([]map[string]interface{}, 0, len(a.attacks))

	for _, attack := range a.attacks {
		exports = append(exports, map[string]interface{}{
			"name":          attack.Name,
			"category":      attack.Category,
			"vulnerability": attack.Vulnerability,
			"binary_path":   attack.BinaryPath,
			"priority":      attack.SuccessCount, // Higher success = higher priority
		})
	}

	return exports
}

// load reads the Armory from disk.
func (a *Armory) load() error {
	data, err := os.ReadFile(a.storePath)
	if err != nil {
		return err
	}

	var stored struct {
		Attacks []ArmoryAttack `json:"attacks"`
		Stats   ArmoryStats    `json:"stats"`
	}

	if err := json.Unmarshal(data, &stored); err != nil {
		return err
	}

	a.attacks = stored.Attacks
	a.stats = stored.Stats

	logging.ShardsDebug("Armory loaded: %d attacks, %d total bugs found", len(a.attacks), a.stats.TotalBugsFound)
	return nil
}

// save writes the Armory to disk.
func (a *Armory) save() error {
	// Ensure directory exists
	dir := filepath.Dir(a.storePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	stored := struct {
		Attacks []ArmoryAttack `json:"attacks"`
		Stats   ArmoryStats    `json:"stats"`
	}{
		Attacks: a.attacks,
		Stats:   a.stats,
	}

	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(a.storePath, data, 0644)
}

// generateAttackID creates a unique ID for an attack.
func generateAttackID(name string) string {
	return name + "_" + time.Now().Format("20060102150405")
}

// ToFacts converts Armory state to kernel facts.
func (a *Armory) ToFacts() []struct {
	Predicate string
	Args      []interface{}
} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	facts := make([]struct {
		Predicate string
		Args      []interface{}
	}, 0)

	for _, attack := range a.attacks {
		facts = append(facts, struct {
			Predicate string
			Args      []interface{}
		}{
			Predicate: "armory_tool",
			Args: []interface{}{
				attack.ID,
				attack.Name,
				attack.Category,
				attack.Vulnerability,
				attack.CreatedAt.Unix(),
			},
		})
	}

	return facts
}
