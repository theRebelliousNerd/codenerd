package campaign

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/tactile"
)

type assaultTargetsFile struct {
	CampaignID string       `json:"campaign_id"`
	CreatedAt  time.Time    `json:"created_at"`
	Scope      AssaultScope `json:"scope"`
	Targets    []string     `json:"targets"`
	Include    []string     `json:"include,omitempty"`
	Exclude    []string     `json:"exclude,omitempty"`
}

type assaultBatchFile struct {
	CampaignID string    `json:"campaign_id"`
	BatchID    string    `json:"batch_id"`
	CreatedAt  time.Time `json:"created_at"`
	Targets    []string  `json:"targets"`
}

type assaultResult struct {
	CampaignID string           `json:"campaign_id"`
	BatchID    string           `json:"batch_id"`
	Target     string           `json:"target"`
	Cycle      int              `json:"cycle"`
	Stage      AssaultStageKind `json:"stage"`
	Attempt    int              `json:"attempt"`
	StartedAt  time.Time        `json:"started_at"`
	DurationMs int64            `json:"duration_ms"`
	ExitCode   int              `json:"exit_code"`
	Killed     bool             `json:"killed,omitempty"`
	KillReason string           `json:"kill_reason,omitempty"`
	Truncated  bool             `json:"truncated,omitempty"`
	LogPath    string           `json:"log_path"`
	Error      string           `json:"error,omitempty"`
}

type stageOutcome struct {
	ExitCode   int
	Killed     bool
	KillReason string
	Truncated  bool
	Error      string
}

type assaultTriageOutput struct {
	Summary            string                  `json:"summary"`
	RecommendedTasks   []assaultRemediationTask `json:"recommended_tasks"`
	AdditionalMetadata map[string]interface{}  `json:"metadata,omitempty"`
}

type assaultRemediationTask struct {
	Type        string   `json:"type"` // "/shard_task" | "/tool_create"
	Priority    string   `json:"priority,omitempty"`
	Description string   `json:"description"`
	Shard       string   `json:"shard,omitempty"`
	ShardInput  string   `json:"shard_input,omitempty"`
	Artifacts   []string `json:"artifacts,omitempty"`
}

func (o *Orchestrator) getAssaultConfig() AssaultConfig {
	if o == nil || o.campaign == nil || o.campaign.Assault == nil {
		return DefaultAssaultConfig()
	}
	cfg := o.campaign.Assault.Normalize()
	// Persist normalization for long-horizon durability.
	o.campaign.Assault = &cfg
	return cfg
}

func (o *Orchestrator) assaultDir() (dir string, slug string) {
	slug = sanitizeCampaignID(o.campaign.ID)
	return filepath.Join(o.workspace, ".nerd", "campaigns", slug, "assault"), slug
}
