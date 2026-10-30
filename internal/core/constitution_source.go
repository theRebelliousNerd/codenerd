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
		pb.WriteString("\n\n# Policy Module: ")
		pb.WriteString(mod)
		pb.WriteString("\n")
		pb.Write(data)
	}
	return sb.String(), pb.String(), coreModules, nil
}

// ErrConstitutionReplaced reports a kernel whose schemas or policy no longer
// begin with the constitution it booted on (a shard specialization replaced
// them): a reload has nothing of its to swap, and leaves it as it is.
var ErrConstitutionReplaced = errors.New("the kernel's program no longer starts with the constitution it booted on")
