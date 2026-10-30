package core

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"codenerd/internal/logging"
)

// The constitution's source. A kernel's schemas and policy come from the .mg
// corpus compiled into the binary (coreLogic) -- except when codeNERD runs on
// its own source tree in dev mode (internal/system, dev_policy.go): then the
// corpus is read from that tree's internal/core, so an edited policy file is
// what the next kernel boots on and what a hot reload swaps in
// (ReloadConstitution) without a rebuild. The choice is process-wide, like
// the embedded corpus it replaces, and is made at boot before any kernel is
// built.

// devConstitution holds the source tree's internal/core directory as an
// fs.FS, or nil for the embedded corpus.
var devConstitution atomic.Pointer[devConstitutionSource]

type devConstitutionSource struct {
	dir  string
	fsys fs.FS
}

// constitutionFS is where the constitution is read from now: the dev source
// tree when one is in use, the binary otherwise. Paths are "defaults/...".
func constitutionFS() fs.FS {
	if src := devConstitution.Load(); src != nil {
		return src.fsys
	}
	return coreLogic
}

// UseDevConstitution makes every kernel built from now on read the
// constitution from dir, a source tree's internal/core directory, instead of
// the binary. dir must hold the corpus (defaults/schemas.mg and
// defaults/policy); anything else is refused and the binary's corpus stays.
func UseDevConstitution(dir string) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("dev constitution: %w", err)
	}
	for _, required := range []string{"defaults/schemas.mg", "defaults/policy"} {
		if _, err := os.Stat(filepath.Join(abs, filepath.FromSlash(required))); err != nil {
			return fmt.Errorf("dev constitution: %s has no %s: %w", abs, required, err)
		}
	}
	devConstitution.Store(&devConstitutionSource{dir: abs, fsys: os.DirFS(abs)})
	logging.Kernel("Constitution read from source tree %s (dev mode)", abs)
	return nil
}

// UseEmbeddedConstitution returns to the binary's corpus.
func UseEmbeddedConstitution() {
	devConstitution.Store(nil)
}

// DevConstitutionDir is the source tree's internal/core directory the
// constitution is read from, or "" when it is the binary's.
func DevConstitutionDir() string {
	if src := devConstitution.Load(); src != nil {
		return src.dir
	}
	return ""
}

// assembleConstitution builds the constitution's schema and policy text from
// fsys exactly as a kernel boots it: the schema index and every module of
// defaultSchemaFiles (marked), every defaults/policy/*.mg (marked), then the
// root policy modules. A file that cannot be read is an error: a dropped
// module would remove Decls or rules silently.
func assembleConstitution(fsys fs.FS) (schemas, policy string, coreModules []string, err error) {
	var sb, pb strings.Builder
	data, err := readRequiredEmbeddedFile(fsys, "defaults/schemas.mg")
	if err != nil {
		return "", "", nil, err
	}
	sb.Write(data)
	for _, schemaFile := range defaultSchemaFiles {
		data, err := readRequiredEmbeddedFile(fsys, "defaults/"+schemaFile)
		if err != nil {
			return "", "", nil, err
		}
		sb.WriteString("\n\n# Schema Module: ")
		sb.WriteString(schemaFile)
		sb.WriteString("\n")
		sb.Write(data)
	}
	entries, err := fs.ReadDir(fsys, "defaults/policy")
	if err != nil {
		return "", "", nil, fmt.Errorf("critical: failed to read policy directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".mg") {
			continue
		}
		data, err := readRequiredEmbeddedFile(fsys, "defaults/policy/"+entry.Name())
		if err != nil {
			return "", "", nil, err
		}
		pb.WriteString("\n\n# Policy Module: ")
		pb.WriteString(entry.Name())
		pb.WriteString("\n")
		pb.Write(data)
	}
	coreModules = DefaultCorePolicyModules()
	for _, mod := range coreModules {
		data, err := readRequiredEmbeddedFile(fsys, "defaults/"+mod)
		if err != nil {
			return "", "", nil, err
		}
		pb.WriteString("\n\n")
		pb.Write(data)
	}
	return sb.String(), pb.String(), coreModules, nil
}

// ErrConstitutionReplaced reports a kernel whose schemas or policy no longer
// begin with the constitution it booted on (a shard specialization replaced
// them): a reload has nothing of its to swap, and leaves it as it is.
var ErrConstitutionReplaced = errors.New("the kernel's program no longer starts with the constitution it booted on")

// constitutionSwap is a validated replacement of one kernel's constitution,
// ready to install.
type constitutionSwap struct {
	k                     *RealKernel
	oldSchemas, oldPolicy string
	schemas, policy       string
	newConstSchemas       string
	newConstPolicy        string
	coreModules           []string
}

// prepareConstitution builds the kernel's program with the constitution read
// now from constitutionFS in place of the one it booted on -- its user
// extensions, overrides and appended policy stay -- and compiles it in full
// (analysis and stratification). Nothing is changed: a corpus that does not
// compile is the error returned and the kernel keeps its program.
func (k *RealKernel) prepareConstitution() (*constitutionSwap, error) {
	constSchemas, constPolicy, coreModules, err := assembleConstitution(constitutionFS())
	if err != nil {
		return nil, err
	}
	k.mu.RLock()
	oldSchemas, oldPolicy, learned := k.schemas, k.policy, k.learned
	bootSchemas, bootPolicy := k.constSchemas, k.constPolicy
	sandbox := k.sandbox
	k.mu.RUnlock()
	if bootSchemas == "" || !strings.HasPrefix(oldSchemas, bootSchemas) || !strings.HasPrefix(oldPolicy, bootPolicy) {
		return nil, ErrConstitutionReplaced
	}
	swap := &constitutionSwap{
		k: k, oldSchemas: oldSchemas, oldPolicy: oldPolicy,
		schemas:         constSchemas + strings.TrimPrefix(oldSchemas, bootSchemas),
		policy:          constPolicy + strings.TrimPrefix(oldPolicy, bootPolicy),
		newConstSchemas: constSchemas, newConstPolicy: constPolicy, coreModules: coreModules,
	}
	var sb strings.Builder
	sb.WriteString(swap.schemas)
	sb.WriteString("\n")
	sb.WriteString(swap.policy)
	sb.WriteString("\n")
	if learned != "" {
		sb.WriteString("# Learned Rules (Autopoiesis Layer - Stratified Trust)\n")
		sb.WriteString(learned)
	}
	text := sb.String()
	if sandbox {
		_, err = compileProgram(text, true)
	} else {
		_, _, err = compileShared(programCacheKey(text), text)
	}
	if err != nil {
		return nil, fmt.Errorf("the edited constitution does not compile: %w", err)
	}
	return swap, nil
}

// install puts the validated program in place, unless the kernel's program
// changed since it was prepared (then it reports that and installs nothing).
// Facts stay; the next evaluation rebuilds the program and recomputes every
// derived fact.
func (s *constitutionSwap) install() error {
	k := s.k
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.schemas != s.oldSchemas || k.policy != s.oldPolicy {
		return fmt.Errorf("the kernel's program changed while the reload was validated; not swapped")
	}
	k.schemas, k.policy = s.schemas, s.policy
	k.constSchemas, k.constPolicy = s.newConstSchemas, s.newConstPolicy
	for _, mod := range s.coreModules {
		k.loadedPolicyFiles[strings.ToLower(mod)] = struct{}{}
	}
	k.markPolicyDirtyLocked()
	k.refreshSchemaValidatorLocked()
	return nil
}

// ReloadConstitution swaps the kernel's constitution for the one constitutionFS
// holds now, after it compiles in full, and re-evaluates. On any failure the
// kernel's program is untouched and the error says why.
func (k *RealKernel) ReloadConstitution() error {
	swap, err := k.prepareConstitution()
	if err != nil {
		return err
	}
	if err := swap.install(); err != nil {
		return err
	}
	return k.Evaluate()
}

// ReloadConstitution swaps every shard's constitution: each is prepared and
// compiled first, and only when every one passes are they installed, so the
// shards never run on two different constitutions. A shard whose program was
// replaced (ErrConstitutionReplaced) is left as it is and named in the error.
func (c *CortexKernel) ReloadConstitution() error {
	shards := c.shardList()
	swaps := make([]*constitutionSwap, 0, len(shards))
	var skipped []string
	for _, s := range shards {
		swap, err := s.Kernel().prepareConstitution()
		if errors.Is(err, ErrConstitutionReplaced) {
			skipped = append(skipped, s.Domain())
			continue
		}
		if err != nil {
			return fmt.Errorf("shard %s: %w", s.Domain(), err)
		}
		swaps = append(swaps, swap)
	}
	for _, swap := range swaps {
		if err := swap.install(); err != nil {
			return err
		}
	}
	for _, s := range shards {
		if err := s.Kernel().Evaluate(); err != nil {
			return fmt.Errorf("shard %s: evaluate after reload: %w", s.Domain(), err)
		}
	}
	if len(skipped) > 0 {
		return fmt.Errorf("%w: shard(s) %s kept their program", ErrConstitutionReplaced, strings.Join(skipped, ", "))
	}
	return nil
}
