package system

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/logging"

	"github.com/fsnotify/fsnotify"
)

// Dev-mode policy hot reload. When the user opts in (dev.policy_hot_reload)
// and the workspace is codeNERD's own source tree, the kernels boot on the
// .mg corpus in that tree (core.UseDevConstitution) and a watcher swaps an
// edited file into the running kernels: the whole program is compiled first
// (analysis and stratification), and only a program that passes is
// installed; a failing edit is reported and the running program stays.
// Facts, shards and session state survive a swap; derived state is
// recomputed. Every outcome is published as a PolicyReloadEvent for the
// session to show.

// PolicyReloadEvent is one hot reload's outcome.
type PolicyReloadEvent struct {
	At    time.Time
	Files []string // the changed corpus files, relative to the tree
	Err   error    // nil: the edit is live; else why the old program stays
}

// devPolicyTree reports the corpus directory (internal/core) of workspace
// when it is codeNERD's source tree: its go.mod declares module codenerd and
// it holds internal/core/defaults with the corpus.
func devPolicyTree(workspace string) (string, error) {
	f, err := os.Open(filepath.Join(workspace, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("no go.mod: %w", err)
	}
	defer f.Close()
	module := ""
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(scanner.Text()), "module "); ok {
			module = strings.TrimSpace(rest)
			break
		}
	}
	if module != "codenerd" {
		return "", fmt.Errorf("go.mod declares module %q, not codenerd", module)
	}
	dir := filepath.Join(workspace, "internal", "core")
	if _, err := os.Stat(filepath.Join(dir, "defaults", "schemas.mg")); err != nil {
		return "", fmt.Errorf("no policy corpus under %s: %w", dir, err)
	}
	return dir, nil
}

// configureDevPolicy chooses the constitution's source for this boot, before
// any kernel is built: the source tree when dev.policy_hot_reload is on and
// the workspace is codeNERD's tree, the binary otherwise. It returns the tree
// it chose, or "".
func configureDevPolicy(bctx *bootContext) string {
	if !bctx.appCfg.GetDev().PolicyHotReload {
		core.UseEmbeddedConstitution()
		return ""
	}
	dir, err := devPolicyTree(bctx.workspace)
	if err == nil {
		err = core.UseDevConstitution(dir)
	}
	if err != nil {
		core.UseEmbeddedConstitution()
		logging.Get(logging.CategoryBoot).Warn("dev.policy_hot_reload is on, but %s is not codeNERD's source tree (%v); the policy compiled into the binary is used", bctx.workspace, err)
		return ""
	}
	logging.Boot("Dev mode: policy loaded from %s, hot reload on", dir)
	return dir
}

// reloadableKernel is a kernel whose constitution can be swapped.
type reloadableKernel interface {
	ReloadConstitution() error
}

// policyReloader watches the tree's corpus and reloads the kernel on edits.
type policyReloader struct {
	dir       string
	workspace string
	kernel    core.Kernel
	watcher   *fsnotify.Watcher
	events    chan PolicyReloadEvent
	debounce  time.Duration
	cancel    context.CancelFunc
	done      chan struct{}
}

// startPolicyReloader watches dir's defaults, defaults/schema and
// defaults/policy for .mg changes and reloads kernel when one settles.
func startPolicyReloader(dir, workspace string, kernel core.Kernel) (*policyReloader, error) {
	if _, ok := kernel.(reloadableKernel); !ok {
		return nil, fmt.Errorf("the kernel cannot reload its constitution")
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	for _, sub := range []string{"defaults", filepath.Join("defaults", "schema"), filepath.Join("defaults", "policy")} {
		if err := watcher.Add(filepath.Join(dir, sub)); err != nil && sub != filepath.Join("defaults", "schema") {
			_ = watcher.Close()
			return nil, fmt.Errorf("watch %s: %w", sub, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &policyReloader{
		dir: dir, workspace: workspace, kernel: kernel, watcher: watcher,
		events: make(chan PolicyReloadEvent, 16), debounce: 300 * time.Millisecond,
		cancel: cancel, done: make(chan struct{}),
	}
	go r.run(ctx)
	return r, nil
}

func (r *policyReloader) run(ctx context.Context) {
	defer close(r.done)
	var (
		mu      sync.Mutex
		pending = map[string]bool{}
		timer   *time.Timer
		fire    = make(chan struct{}, 1)
	)
	for {
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case ev, ok := <-r.watcher.Events:
			if !ok {
				return
			}
			if !strings.HasSuffix(ev.Name, ".mg") || ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}
			rel, err := filepath.Rel(r.dir, ev.Name)
			if err != nil {
				rel = ev.Name
			}
			mu.Lock()
			pending[filepath.ToSlash(rel)] = true
			mu.Unlock()
			if timer == nil {
				timer = time.AfterFunc(r.debounce, func() {
					select {
					case fire <- struct{}{}:
					default:
					}
				})
			} else {
				timer.Reset(r.debounce)
			}
		case err, ok := <-r.watcher.Errors:
			if !ok {
				return
			}
			logging.Get(logging.CategoryKernel).Warn("policy hot reload: watcher error: %v", err)
		case <-fire:
			mu.Lock()
			files := make([]string, 0, len(pending))
			for f := range pending {
				files = append(files, f)
			}
			pending = map[string]bool{}
			mu.Unlock()
			slices.Sort(files)
			r.publish(PolicyReloadEvent{At: time.Now(), Files: files, Err: r.reload()})
		}
	}
}

// reload swaps the kernel's constitution and, for the domain cortex, rebuilds
// the derivation map the shards' routing reads from the new corpus.
func (r *policyReloader) reload() error {
	if err := r.kernel.(reloadableKernel).ReloadConstitution(); err != nil {
		return err
	}
	if cortex, ok := r.kernel.(*core.CortexKernel); ok {
		dm, err := buildKernelDerivationMap(defaultKernelShardConfigs(r.workspace))
		if err != nil {
			return fmt.Errorf("the corpus reloaded, but its derivation map could not be rebuilt (shared facts go everywhere until the next reload): %w", err)
		}
		cortex.SetDerivationMap(dm)
	}
	return nil
}

func (r *policyReloader) publish(ev PolicyReloadEvent) {
	if ev.Err != nil {
		logging.Get(logging.CategoryKernel).Warn("policy hot reload of %v failed; the running program stays: %v", ev.Files, ev.Err)
	} else {
		logging.Kernel("policy hot reload: %v is live", ev.Files)
	}
	select {
	case r.events <- ev:
	default:
		logging.Get(logging.CategoryKernel).Warn("policy hot reload: no reader for the outcome of %v; it is logged only", ev.Files)
	}
}

// stop ends the watcher and joins it.
func (r *policyReloader) stop() {
	r.cancel()
	_ = r.watcher.Close()
	<-r.done
}

// PolicyReloads is the stream of dev-mode hot reload outcomes, or nil when
// this session reads the binary's policy.
func (c *Cortex) PolicyReloads() <-chan PolicyReloadEvent {
	if c == nil || c.policyReloader == nil {
		return nil
	}
	return c.policyReloader.events
}

// DevPolicyDir is the source tree the session's policy is read from, or ""
// when it is the binary's.
func (c *Cortex) DevPolicyDir() string {
	if c == nil || c.policyReloader == nil {
		return ""
	}
	return c.policyReloader.dir
}

// initDevPolicyReload starts the watcher for a dev-mode boot.
func initDevPolicyReload(bctx *bootContext) error {
	if bctx.devPolicyDir == "" {
		return nil
	}
	r, err := startPolicyReloader(bctx.devPolicyDir, bctx.workspace, bctx.kernel)
	if err != nil {
		logging.Get(logging.CategoryBoot).Warn("Dev mode: policy is read from %s, but hot reload could not start: %v", bctx.devPolicyDir, err)
		return nil
	}
	bctx.policyReloader = r
	return nil
}
