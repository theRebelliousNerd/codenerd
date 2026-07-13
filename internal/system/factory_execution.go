package system

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/tactile"
)

func executionLayerConfigs(appCfg *config.UserConfig, workspace string) (tactile.ExecutorConfig, core.VirtualStoreConfig, error) {
	if appCfg == nil {
		return tactile.ExecutorConfig{}, core.VirtualStoreConfig{}, fmt.Errorf("user config is nil")
	}

	execution := appCfg.GetExecution()
	workingDir, err := containedExecutionWorkingDir(workspace, execution.WorkingDirectory)
	if err != nil {
		return tactile.ExecutorConfig{}, core.VirtualStoreConfig{}, err
	}
	timeout, err := time.ParseDuration(execution.DefaultTimeout)
	if err != nil || timeout <= 0 {
		return tactile.ExecutorConfig{}, core.VirtualStoreConfig{}, fmt.Errorf("invalid execution.default_timeout %q", execution.DefaultTimeout)
	}

	executorCfg := tactile.DefaultExecutorConfig()
	executorCfg.DefaultWorkingDir = workingDir
	executorCfg.DefaultTimeout = timeout
	if timeout > executorCfg.MaxTimeout {
		executorCfg.MaxTimeout = timeout
	}
	executorCfg.AllowedEnvironment = append([]string(nil), execution.AllowedEnvVars...)
	if executorCfg.DefaultLimits != nil {
		limits := *executorCfg.DefaultLimits
		limits.TimeoutMs = timeout.Milliseconds()
		executorCfg.DefaultLimits = &limits
	}

	virtualStoreCfg := core.VirtualStoreConfig{
		WorkingDir:      workingDir,
		AllowedEnvVars:  append([]string(nil), execution.AllowedEnvVars...),
		AllowedBinaries: append([]string(nil), execution.AllowedBinaries...),
	}
	return executorCfg, virtualStoreCfg, nil
}

func containedExecutionWorkingDir(workspace, configured string) (string, error) {
	workspaceAbs, err := filepath.Abs(workspace)
	if err != nil {
		return "", fmt.Errorf("resolve workspace: %w", err)
	}
	workspaceAbs = filepath.Clean(workspaceAbs)

	configured = strings.TrimSpace(configured)
	if configured == "" || configured == "." {
		return workspaceAbs, nil
	}
	candidate := configured
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(workspaceAbs, candidate)
	}
	candidate, err = filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve execution working directory: %w", err)
	}
	candidate = filepath.Clean(candidate)

	rel, err := filepath.Rel(workspaceAbs, candidate)
	if err != nil {
		return "", fmt.Errorf("compare execution working directory: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("execution.working_directory %q escapes workspace %q", configured, workspaceAbs)
	}
	return candidate, nil
}
