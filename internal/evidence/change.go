// Package evidence binds explicit acceptance obligations to workspace contents.
// Its first supported contract is a Go bug fix with pre-existing regression tests.
package evidence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"codenerd/internal/processutil"
	"codenerd/internal/tools"
)

type Obligation struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Package     string `json:"package"`
	Test        string `json:"test"`
	TestFile    string `json:"test_file"`
	Reproducer  bool   `json:"reproducer"`
}

// Contract is supplied by the caller, never extracted from the model's final
// response. The executor copies it before the first tool call.
type Contract struct {
	Task        string       `json:"task"`
	Authority   string       `json:"authority"`
	Obligations []Obligation `json:"obligations"`
}

type Witness struct {
	Obligation  string        `json:"obligation"`
	Snapshot    string        `json:"snapshot"`
	Command     []string      `json:"command"`
	Toolchain   string        `json:"toolchain"`
	Environment string        `json:"environment_sha256"`
	Status      string        `json:"status"`
	OutputHash  string        `json:"output_sha256"`
	Detail      string        `json:"detail,omitempty"`
	Duration    time.Duration `json:"duration_ns"`
}

type Report struct {
	Contract   Contract  `json:"contract"`
	ContractID string    `json:"contract_sha256"`
	Before     string    `json:"before"`
	After      string    `json:"after"`
	Status     string    `json:"status"`
	Baseline   []Witness `json:"baseline"`
	Witnesses  []Witness `json:"witnesses"`
	Unknown    []string  `json:"unknown,omitempty"`
}

type Transaction struct {
	root      string
	env       []string
	goPath    string
	contract  Contract
	protected map[string]string
	report    Report
}

func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

func LoadContract(path string) (Contract, error) {
	f, err := os.Open(path)
	if err != nil {
		return Contract{}, err
	}
	defer f.Close()
	if info, err := f.Stat(); err != nil || info.Size() > 1024*1024 {
		return Contract{}, errors.New("acceptance contract exceeds 1 MiB or cannot be inspected")
	}
	dec := json.NewDecoder(io.LimitReader(f, 1024*1024))
	dec.DisallowUnknownFields()
	var c Contract
	if err := dec.Decode(&c); err != nil {
		return c, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return c, errors.New("acceptance contract must contain exactly one JSON object")
	}
	return c, c.Validate()
}

func (c Contract) Validate() error {
	if strings.TrimSpace(c.Task) == "" || strings.TrimSpace(c.Authority) == "" || len(c.Obligations) == 0 || len(c.Obligations) > 32 {
		return errors.New("acceptance requires task, authority and 1..32 obligations")
	}
	seen := map[string]bool{}
	reproducer := false
	for _, o := range c.Obligations {
		if o.ID == "" || seen[o.ID] || strings.TrimSpace(o.Description) == "" {
			return errors.New("acceptance obligations require unique IDs and descriptions")
		}
		seen[o.ID] = true
		if !regexp.MustCompile(`^Test[A-Za-z0-9_]+$`).MatchString(o.Test) {
			return fmt.Errorf("%s: require one exact Go test name", o.ID)
		}
		if o.Package != "." && (!strings.HasPrefix(o.Package, "./") || strings.Contains(o.Package, "..") || strings.ContainsAny(o.Package, "\\\n\r\t ")) {
			return fmt.Errorf("%s: package must be one workspace-relative package", o.ID)
		}
		if !strings.HasSuffix(o.TestFile, "_test.go") {
			return fmt.Errorf("%s: regression test file is required", o.ID)
		}
		reproducer = reproducer || o.Reproducer
	}
	if !reproducer {
		return errors.New("Go bug-fix acceptance requires a reproducer obligation")
	}
	return nil
}

// Snapshot includes file names, modes and contents, including dirty and
// untracked files. Runtime .nerd state and Git administration are excluded;
// project configuration and agent definitions are included explicitly.
// Symlinks are rejected: an external target cannot supply bounded evidence.
func Snapshot(ctx context.Context, root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", errors.New("explicit workspace required for evidence")
	}
	root, err := tools.CanonicalWorkspaceRoot(root)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			if filepath.ToSlash(rel) == ".nerd" {
				return nil
			}
			if strings.HasPrefix(filepath.ToSlash(rel), ".nerd/") && rel != filepath.Join(".nerd", "agents") && !strings.HasPrefix(filepath.ToSlash(rel), ".nerd/agents/") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(filepath.ToSlash(rel), ".nerd/") && rel != filepath.Join(".nerd", "config.json") && !strings.HasPrefix(filepath.ToSlash(rel), ".nerd/agents/") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("snapshot cannot certify non-regular file %s", rel)
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		fh := sha256.New()
		_, copyErr := io.Copy(fh, f)
		closeErr := f.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		fmt.Fprintf(h, "%q:%o:%x\n", filepath.ToSlash(rel), info.Mode(), fh.Sum(nil))
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func Begin(ctx context.Context, root string, c Contract) (*Transaction, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("explicit workspace required for acceptance")
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	root, err := tools.CanonicalWorkspaceRoot(root)
	if err != nil {
		return nil, err
	}
	c.Obligations = append([]Obligation(nil), c.Obligations...)
	encoded, _ := json.Marshal(c)
	tx := &Transaction{root: root, contract: c, protected: map[string]string{}, report: Report{Contract: c, ContractID: digest(encoded), Status: "unverified"}}
	tx.env = os.Environ()
	sort.Strings(tx.env)
	tx.goPath, err = exec.LookPath("go")
	if err != nil {
		return nil, err
	}
	tx.report.Before, err = Snapshot(ctx, root)
	if err != nil {
		return nil, err
	}
	tx.protected, err = verificationInputs(ctx, root)
	if err != nil {
		return nil, err
	}
	for _, o := range c.Obligations {
		path, err := tools.ResolveWorkspacePath(ctx, root, o.TestFile)
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		tx.protected[path] = digest(data)
		// Pin the file that actually defines the selected test, not an
		// unrelated file supplied alongside a matching name elsewhere.
		file, parseErr := parser.ParseFile(token.NewFileSet(), path, data, 0)
		if parseErr != nil {
			return nil, parseErr
		}
		found := false
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == o.Test {
				found = true
			}
		}
		pkgPath, resolveErr := tools.ResolveWorkspaceDir(ctx, root, o.Package)
		if !found || resolveErr != nil || filepath.Dir(path) != pkgPath {
			return nil, fmt.Errorf("%s: test_file must define the selected test in its package", o.ID)
		}
		w := tx.run(ctx, o, tx.report.Before)
		tx.report.Baseline = append(tx.report.Baseline, w)
		if o.Reproducer && w.Status != "failed" {
			return nil, fmt.Errorf("%s: reproducer must demonstrably fail before editing (got %s: %s)", o.ID, w.Status, w.Detail)
		}
		if !o.Reproducer && w.Status != "passed" {
			return nil, fmt.Errorf("%s: regression baseline must pass (got %s)", o.ID, w.Status)
		}
	}
	after, err := Snapshot(ctx, root)
	if err != nil {
		return nil, err
	}
	if after != tx.report.Before {
		return nil, errors.New("workspace changed while recording acceptance baseline")
	}
	return tx, nil
}

func (tx *Transaction) Verify(ctx context.Context) Report {
	r := tx.report
	r.Witnesses = nil
	r.Unknown = nil
	r.Status = "unverified"
	inputs, inputErr := verificationInputs(ctx, tx.root)
	if inputErr != nil || len(inputs) != len(tx.protected) {
		r.Unknown = append(r.Unknown, "verification inputs added, removed, or unreadable")
	}
	for path, want := range tx.protected {
		data, err := os.ReadFile(path)
		if err != nil || digest(data) != want {
			r.Unknown = append(r.Unknown, "acceptance test changed or disappeared: "+path)
		}
	}
	after, err := Snapshot(ctx, tx.root)
	if err != nil {
		r.Unknown = append(r.Unknown, err.Error())
		return r
	}
	r.After = after
	if after == r.Before {
		r.Unknown = append(r.Unknown, "no artifact change")
	}
	for _, o := range tx.contract.Obligations {
		r.Witnesses = append(r.Witnesses, tx.run(ctx, o, after))
	}
	current, err := Snapshot(ctx, tx.root)
	if err != nil || current != after {
		r.Unknown = append(r.Unknown, "workspace changed during verification")
	}
	for _, w := range r.Witnesses {
		if w.Status != "passed" {
			r.Unknown = append(r.Unknown, w.Obligation+": "+w.Status)
		}
		for _, baseline := range r.Baseline {
			if baseline.Obligation == w.Obligation && (baseline.Toolchain != w.Toolchain || baseline.Environment != w.Environment) {
				r.Unknown = append(r.Unknown, w.Obligation+": verification environment changed")
			}
		}
	}
	if len(r.Unknown) == 0 {
		r.Status = "verified"
	}
	return r
}

// This first contract supports source bug fixes with caller-owned tests and
// build configuration. Changing tests, fixtures or module selection requires
// a newly reviewed contract, rather than silently weakening its witnesses.
func verificationInputs(ctx context.Context, root string) (map[string]string, error) {
	inputs := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == ".nerd" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		name := d.Name()
		if !strings.HasSuffix(name, "_test.go") && name != "go.mod" && name != "go.sum" && name != "go.work" && name != "go.work.sum" && !strings.Contains("/"+filepath.ToSlash(rel), "/testdata/") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		inputs[path] = digest(data)
		return nil
	})
	return inputs, err
}

func (tx *Transaction) Unverified(reason string) Report {
	r := tx.report
	r.Status = "unverified"
	r.Unknown = []string{reason}
	return r
}

// Persist writes the machine-readable witness beside runtime state, outside
// the snapshot input set. Rename publishes a complete report atomically.
func Persist(root string, r Report) (string, error) {
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(r.ContractID) || (r.After != "" && !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(r.After)) {
		return "", errors.New("invalid evidence identity")
	}
	dir, err := tools.ResolveWorkspacePath(context.Background(), root, filepath.Join(".nerd", "evidence"))
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, "report-*.tmp")
	if err != nil {
		return "", err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return "", err
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	path := filepath.Join(dir, r.ContractID+"-"+short(r.After)+".json")
	if err = os.Rename(temp, path); err != nil {
		return "", err
	}
	return path, nil
}

func (r Report) Summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Acceptance: %s (snapshot %s).", r.Status, short(r.After))
	for _, w := range r.Witnesses {
		fmt.Fprintf(&b, "\n- %s: %s", w.Obligation, w.Status)
	}
	for _, u := range r.Unknown {
		fmt.Fprintf(&b, "\n- Unverified: %s", u)
	}
	return b.String()
}

func short(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

func (tx *Transaction) run(ctx context.Context, o Obligation, snapshot string) Witness {
	start := time.Now()
	w := Witness{Obligation: o.ID, Snapshot: snapshot, Status: "unverified", Command: []string{"go", "test", "-json", "-count=1", "-run", "^" + regexp.QuoteMeta(o.Test) + "$", o.Package}}
	ctx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	versionCmd := processutil.Cancellable(exec.CommandContext(ctx, tx.goPath, "version"))
	versionCmd.Dir, versionCmd.Env = tx.root, tx.env
	version, err := versionCmd.CombinedOutput()
	if err != nil {
		w.Detail = err.Error()
		return w
	}
	w.Toolchain = strings.TrimSpace(string(version))
	// Pin process environment at acceptance, including PATH and workspace
	// selection. Boot cannot change the verifier's inputs mid-transaction.
	goEnv := processutil.Cancellable(exec.CommandContext(ctx, tx.goPath, "env", "-json"))
	goEnv.Dir, goEnv.Env = tx.root, tx.env
	effective, envErr := goEnv.CombinedOutput()
	if envErr != nil {
		w.Detail = "cannot inspect verification environment: " + envErr.Error()
		return w
	}
	// Go emits a fresh temporary directory in its diagnostic prefix-map on
	// every `go env` call. Normalize only that generated source path; retain
	// every effective compiler, target, tag and module setting in the identity.
	var effectiveConfig map[string]any
	if err := json.Unmarshal(effective, &effectiveConfig); err != nil {
		w.Detail = "invalid go env response"
		return w
	}
	if flags, ok := effectiveConfig["GOGCCFLAGS"].(string); ok {
		effectiveConfig["GOGCCFLAGS"] = regexp.MustCompile(`-ffile-prefix-map=.+?go-build[0-9]+=/tmp/go-build`).ReplaceAllString(flags, "-ffile-prefix-map=<go-work>=/tmp/go-build")
	}
	effective, _ = json.Marshal(effectiveConfig)
	w.Environment = digest(append([]byte(strings.Join(tx.env, "\n")), effective...))
	cmd := processutil.Cancellable(exec.CommandContext(ctx, tx.goPath, w.Command[1:]...))
	cmd.Dir = tx.root
	cmd.Env = tx.env
	out, err := cmd.CombinedOutput()
	w.OutputHash = digest(out)
	w.Duration = time.Since(start)
	if ctx.Err() != nil {
		w.Detail = ctx.Err().Error()
		return w
	}
	var testPass, testFail, packagePass bool
	for _, line := range strings.Split(string(out), "\n") {
		var event struct {
			Action string
			Test   string
		}
		if json.Unmarshal([]byte(line), &event) != nil {
			continue
		}
		if event.Test == o.Test {
			testPass = testPass || event.Action == "pass"
			testFail = testFail || event.Action == "fail"
		}
		packagePass = packagePass || (event.Test == "" && event.Action == "pass")
	}
	if err == nil && testPass && packagePass {
		w.Status = "passed"
	} else if err != nil && testFail {
		w.Status = "failed"
	} else {
		w.Detail = "test did not supply an executed pass/fail witness"
	}
	return w
}
