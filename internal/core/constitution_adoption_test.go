package core

import (
	"errors"
	"io/fs"
	"strings"
	"testing"

	"codenerd/internal/types"
)

func reloadIntent(target string) Fact {
	return Fact{Predicate: "user_intent", Args: []any{
		types.MangleAtom("/current_intent"), types.MangleAtom("/mutation"), types.MangleAtom("/fix"), target, ""}}
}

func useReloadTree(t *testing.T) string {
	t.Helper()
	root := devTree(t)
	if err := UseDevConstitution(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(UseEmbeddedConstitution)
	return root
}

func TestReloadConstitution_EvaluationRejectionPreservesAcceptedStore(t *testing.T) {
	root := useReloadTree(t)
	k, err := NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	if err := k.Assert(reloadIntent("kept")); err != nil {
		t.Fatal(err)
	}
	if err := k.Evaluate(); err != nil {
		t.Fatal(err)
	}
	oldPolicy, oldStore, oldInfo := k.policy, k.store, k.programInfo
	k.SetDerivedFactLimit(1)
	appendPolicy(t, root, reloadProbe)
	for range 2 {
		out, err := k.ReloadConstitution()
		if err == nil || out.Stage != "evaluate" || out.Disposition != "rejected" || out.Candidate == out.Active {
			t.Fatalf("evaluation witness did not reject before adoption: %+v %v", out, err)
		}
		if k.policy != oldPolicy || k.store != oldStore || k.programInfo != oldInfo || k.evalErr != nil {
			t.Fatal("evaluation failure changed the accepted generation or poisoned its store")
		}
		if rows, err := k.Query("user_intent"); err != nil || len(rows) != 1 {
			t.Fatalf("accepted store is no longer readable: %v %v", rows, err)
		}
	}
	k.SetDerivedFactLimit(0)
	out, err := k.ReloadConstitution()
	if err != nil || out.Stage != "ready" || out.Active != out.Candidate || out.Disposition != "adopted" {
		t.Fatalf("valid retry did not recover: %+v %v", out, err)
	}
	if got := probeTargets(t, k); len(got) != 1 || got[0] != "kept" {
		t.Fatalf("probe after recovery = %v", got)
	}
}

func reloadCortex(t *testing.T, configs ...KernelShardConfig) (*CortexKernel, []*RealKernel) {
	t.Helper()
	c := NewCortexKernel("cortex")
	if err := c.SetSharedPredicates([]string{"user_intent"}); err != nil {
		t.Fatal(err)
	}
	if len(configs) == 0 {
		configs = []KernelShardConfig{{Domain: "cortex"}, {Domain: "world"}, {Domain: "z-special"}}
	}
	var kernels []*RealKernel
	for _, config := range configs {
		s, err := NewKernelShard(config)
		if err != nil {
			t.Fatal(err)
		}
		if err := c.RegisterShard(s); err != nil {
			t.Fatal(err)
		}
		kernels = append(kernels, s.kernel)
	}
	c.SetFactRouter(nil)
	return c, kernels
}

func TestCortexReload_RejectsTheWholeGroupOnLateEvaluationOrSpecialization(t *testing.T) {
	for _, failure := range []string{"evaluation", "specialization"} {
		t.Run(failure, func(t *testing.T) {
			root := useReloadTree(t)
			c, kernels := reloadCortex(t)
			if err := c.Assert(reloadIntent("retained")); err != nil {
				t.Fatal(err)
			}
			if err := c.Evaluate(); err != nil {
				t.Fatal(err)
			}
			last := kernels[len(kernels)-1]
			if failure == "evaluation" {
				last.SetDerivedFactLimit(1)
			} else {
				last.ClearSchemas() // a deliberately specialized program is ineligible
			}
			policies := make([]string, len(kernels))
			for i, k := range kernels {
				policies[i] = k.policy
			}
			oldMap := &DerivationMap{}
			c.SetDerivationMap(oldMap)
			appendPolicy(t, root, reloadProbe)
			out, err := c.ReloadConstitution()
			if err == nil || out.Disposition != "rejected" {
				t.Fatalf("group accepted: %+v %v", out, err)
			}
			if failure == "specialization" && !errors.Is(err, ErrConstitutionReplaced) {
				t.Fatalf("lost specialization cause: %v", err)
			}
			if failure == "evaluation" && out.Stage != "evaluate" {
				t.Fatalf("fixture failed at %s: %v", out.Stage, err)
			}
			if c.derivationMap != oldMap {
				t.Fatal("rejection changed routing")
			}
			for i, k := range kernels {
				if k.policy != policies[i] {
					t.Fatalf("shard %d adopted despite group rejection", i)
				}
			}
		})
	}
}

func TestCortexReload_HydratesAlreadyPresentSharedFactsForNewConsumers(t *testing.T) {
	root := useReloadTree(t)
	c, kernels := reloadCortex(t,
		KernelShardConfig{Domain: "cortex"},
		KernelShardConfig{Domain: "world", OwnedPredicates: []string{"file_topology"}},
		KernelShardConfig{Domain: "z-special"})
	// Model old routing without a world consumer. The rebuilt map must use
	// world-owned file_topology: the shipped context_priority rule joins its
	// file_exists derivation with user_intent. The shared-only reloadProbe
	// alone needs only the catch-all, not a replica in every registered shard.
	c.SetDerivationMap(&DerivationMap{Consumes: map[string]map[string]struct{}{}})
	if err := c.Assert(Fact{Predicate: "file_topology", Args: []any{
		"before-edit", "hash", types.MangleAtom("/go"), int64(0), types.MangleAtom("/false")}}); err != nil {
		t.Fatal(err)
	}
	if err := c.Assert(reloadIntent("before-edit")); err != nil {
		t.Fatal(err)
	}
	for _, k := range kernels[1:] {
		if rows, err := k.Query("user_intent"); err != nil || len(rows) != 0 {
			t.Fatalf("fixture replica = %v %v", rows, err)
		}
	}
	appendPolicy(t, root, reloadProbe)
	out, err := c.ReloadConstitution()
	if err != nil || out.Disposition != "adopted" {
		t.Fatalf("reload: %+v %v", out, err)
	}
	if c.derivationMap == nil {
		t.Fatal("common staged program did not receive a derivation map")
	}
	if _, consumes := c.derivationMap.Consumes["world"]["user_intent"]; !consumes {
		t.Fatal("fixture did not establish a world consumer of user_intent")
	}
	if _, consumes := c.derivationMap.Consumes["z-special"]["user_intent"]; consumes {
		t.Fatal("fixture unexpectedly established an unrelated consumer")
	}
	for i, k := range kernels {
		if i == 2 {
			if rows, err := k.Query("user_intent"); err != nil || len(rows) != 0 {
				t.Fatalf("nonconsumer received the shared intent: %v %v", rows, err)
			}
		} else if got := probeTargets(t, k); len(got) != 1 || got[0] != "before-edit" {
			t.Fatalf("new consumer %d lacks the existing intent: %v", i, got)
		}
		if constitutionIdentity(k.constSchemas, k.constPolicy) != out.Active {
			t.Fatal("mixed constitution generations")
		}
	}
	if rows, err := c.Query("context_priority(\"before-edit\", 100)"); err != nil || len(rows) != 1 {
		t.Fatalf("routed world-local join lacks the existing intent: %v %v", rows, err)
	}
	// Leave a stale replica outside the current routing targets. Retraction
	// clears canonical state; the next reload must also retire that replica.
	if err := kernels[2].Assert(reloadIntent("stale-replica")); err != nil {
		t.Fatal(err)
	}
	if err := c.Retract("user_intent"); err != nil {
		t.Fatal(err)
	}
	if rows, err := kernels[2].Query("user_intent"); err != nil || len(rows) != 1 {
		t.Fatalf("fixture did not retain a stale nonconsumer replica: %v %v", rows, err)
	}
	if _, err := c.ReloadConstitution(); err != nil {
		t.Fatal(err)
	}
	for _, k := range kernels {
		if got := probeTargets(t, k); len(got) != 0 {
			t.Fatalf("stale replica resurrected: %v", got)
		}
	}
}

func TestCortexReload_KeepsUniformSharedDerivationsOnCatchAll(t *testing.T) {
	root := useReloadTree(t)
	c, kernels := reloadCortex(t)
	c.SetDerivationMap(&DerivationMap{Consumes: map[string]map[string]struct{}{}})
	if err := c.Assert(reloadIntent("uniform")); err != nil {
		t.Fatal(err)
	}
	appendPolicy(t, root, reloadProbe)
	if _, err := c.ReloadConstitution(); err != nil {
		t.Fatal(err)
	}
	if c.derivationMap == nil {
		t.Fatal("common staged program did not receive a derivation map")
	}
	if rows, err := c.Query("dev_reload_probe"); err != nil || len(rows) != 1 || types.ExtractString(rows[0].Args[0]) != "uniform" {
		t.Fatalf("routed uniform derivation = %v %v", rows, err)
	}
	for i, domain := range []string{"world", "z-special"} {
		k := kernels[i+1]
		if _, consumes := c.derivationMap.Consumes[domain]["user_intent"]; consumes {
			t.Fatal("shared-only rule unexpectedly requires a noncanonical replica")
		}
		if rows, err := k.Query("user_intent"); err != nil || len(rows) != 0 {
			t.Fatalf("nonconsumer %d received the shared intent: %v %v", i+1, rows, err)
		}
	}
}

// A read seam on the actual source FS controls edits without a timing race.
type reloadReadFS struct {
	fs.FS
	read func(string, []byte) ([]byte, error)
}

func (f reloadReadFS) ReadFile(name string) ([]byte, error) {
	b, err := fs.ReadFile(f.FS, name)
	if err != nil {
		return nil, err
	}
	return f.read(name, b)
}

func TestCortexReload_CapturesOnceForEveryShardAndNeverRereadsForRouting(t *testing.T) {
	root := useReloadTree(t)
	c, kernels := reloadCortex(t)
	appendPolicy(t, root, reloadProbe)
	source := devConstitution.Load()
	reads := 0
	devConstitution.Store(&devConstitutionSource{dir: root, fsys: reloadReadFS{FS: source.fsys, read: func(name string, b []byte) ([]byte, error) {
		if name == "defaults/schemas.mg" {
			reads++
			if reads > 2 {
				return nil, errors.New("source unavailable after capture")
			}
		}
		return b, nil
	}}})
	out, err := c.ReloadConstitution()
	if err != nil || reads != 2 {
		t.Fatalf("capture/routing reread source: reads=%d %+v %v", reads, out, err)
	}
	for _, k := range kernels {
		if constitutionIdentity(k.constSchemas, k.constPolicy) != out.Active {
			t.Fatal("shards adopted different captures")
		}
	}
}

func TestReloadConstitution_RejectsSourceThatChangesDuringCapture(t *testing.T) {
	root := useReloadTree(t)
	k, err := NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	source := devConstitution.Load()
	reads := 0
	devConstitution.Store(&devConstitutionSource{dir: root, fsys: reloadReadFS{FS: source.fsys, read: func(name string, b []byte) ([]byte, error) {
		if name == "defaults/schemas.mg" {
			reads++
			if reads == 2 {
				return append(b, []byte("\n# changed during capture\n")...), nil
			}
		}
		return b, nil
	}}})
	old := k.policy
	out, err := k.ReloadConstitution()
	if err == nil || out.Stage != "capture" || !strings.Contains(err.Error(), "changed during capture") || k.policy != old {
		t.Fatalf("unstable capture was adopted: %+v %v", out, err)
	}
}

func TestCortexReload_QuiescesRoutingAndDirectKernelWrites(t *testing.T) {
	root := useReloadTree(t)
	c, kernels := reloadCortex(t)
	appendPolicy(t, root, reloadProbe)
	source := devConstitution.Load()
	entered, release := make(chan struct{}), make(chan struct{})
	reads := 0
	devConstitution.Store(&devConstitutionSource{dir: root, fsys: reloadReadFS{FS: source.fsys, read: func(name string, b []byte) ([]byte, error) {
		if name == "defaults/schemas.mg" {
			reads++
			if reads == 1 {
				close(entered)
				<-release
			}
		}
		return b, nil
	}}})
	reloaded := make(chan error, 1)
	go func() { _, err := c.ReloadConstitution(); reloaded <- err }()
	<-entered // the production capture is paused with its adoption locks held
	routingLocked := !c.adoptionMu.TryRLock()
	if !routingLocked {
		c.adoptionMu.RUnlock()
	}
	allKernelsLocked := true
	for _, k := range kernels {
		if k.mu.TryLock() {
			allKernelsLocked = false
			k.mu.Unlock()
		}
	}
	written, queried, appended := make(chan error, 1), make(chan error, 1), make(chan struct{})
	go func() { written <- c.Assert(reloadIntent("during-adoption")) }()
	go func() { _, err := c.Query("dev_reload_probe"); queried <- err }()
	go func() { kernels[1].AppendPolicy("\n# direct writer after adoption\n"); close(appended) }()
	close(release)
	if err := <-reloaded; err != nil {
		t.Fatal(err)
	}
	if err := <-written; err != nil {
		t.Fatal(err)
	}
	if err := <-queried; err != nil {
		t.Fatal(err)
	}
	<-appended
	if !routingLocked || !allKernelsLocked {
		t.Fatal("capture did not quiesce routing and every live program")
	}
	if !strings.HasSuffix(kernels[1].policy, "# direct writer after adoption\n") {
		t.Fatal("concurrent direct program change was overwritten")
	}
	if rows, err := c.Query("user_intent"); err != nil || len(rows) != 1 || types.ExtractString(rows[0].Args[3]) != "during-adoption" {
		t.Fatalf("concurrent legitimate fact was lost: %v %v", rows, err)
	}
}

func TestCortexReload_PreservesExtensionsAndUsesBroadcastForDifferentPrograms(t *testing.T) {
	root := useReloadTree(t)
	c, kernels := reloadCortex(t)
	// Reuse the existing compile/evaluation fixture, without introducing a
	// second set of policy rules merely for reload.
	kernels[1].AppendPolicy(conePolicy)
	if err := c.Assert(reloadIntent("kept")); err != nil {
		t.Fatal(err)
	}
	appendPolicy(t, root, reloadProbe)
	out, err := c.ReloadConstitution()
	if err != nil || out.Disposition != "adopted" {
		t.Fatalf("extended program rejected: %+v %v", out, err)
	}
	if c.derivationMap != nil {
		t.Fatal("different programs used a map for an invented common program")
	}
	if !strings.Contains(kernels[1].policy, conePolicy) {
		t.Fatal("appended program was lost")
	}
	for i, k := range kernels {
		if got := probeTargets(t, k); len(got) != 1 || got[0] != "kept" {
			t.Fatalf("broadcast consumer %d lost existing facts: %v", i, got)
		}
	}
}
