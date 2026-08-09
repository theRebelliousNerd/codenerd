package security

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PathPolicy confines model-supplied browser artifacts to explicit roots.
type PathPolicy struct {
	baseDir string
	roots   []string
}

// NewPathPolicy resolves allowed roots against baseDir.
func NewPathPolicy(baseDir string, roots []string) (*PathPolicy, error) {
	if strings.TrimSpace(baseDir) == "" {
		var err error
		baseDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get path policy base directory: %w", err)
		}
	}
	baseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("resolve path policy base directory: %w", err)
	}
	if len(roots) == 0 {
		roots = []string{filepath.Join(".nerd", "browser", "screenshots"), filepath.Join(".nerd", "browser", "traces")}
	}
	resolved := make([]string, 0, len(roots))
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		if !filepath.IsAbs(root) {
			root = filepath.Join(baseDir, root)
		}
		absolute, err := filepath.Abs(root)
		if err != nil {
			return nil, fmt.Errorf("resolve writable root %q: %w", root, err)
		}
		resolved = append(resolved, filepath.Clean(absolute))
	}
	if len(resolved) == 0 {
		return nil, errors.New("browser writable_roots must contain at least one path")
	}
	return &PathPolicy{baseDir: filepath.Clean(baseDir), roots: resolved}, nil
}

// DefaultRoot returns the first configured writable root.
func (p *PathPolicy) DefaultRoot() string {
	if p == nil || len(p.roots) == 0 {
		return ""
	}
	return p.roots[0]
}

// ResolveForWrite validates the final path, including existing symlink parents.
func (p *PathPolicy) ResolveForWrite(requested, defaultRoot, defaultName string) (string, error) {
	if p == nil {
		return "", errors.New("browser write path policy is not configured")
	}
	if strings.TrimSpace(requested) == "" {
		requested = defaultName
	}
	if !filepath.IsAbs(requested) {
		base := p.baseDir
		if strings.TrimSpace(defaultRoot) != "" {
			base = defaultRoot
			if !filepath.IsAbs(base) {
				base = filepath.Join(p.baseDir, base)
			}
		}
		requested = filepath.Join(base, requested)
	}
	target, err := filepath.Abs(requested)
	if err != nil {
		return "", fmt.Errorf("resolve browser output path: %w", err)
	}
	target = filepath.Clean(target)
	resolvedTarget, err := resolveExistingPrefix(target)
	if err != nil {
		return "", err
	}
	for _, root := range p.roots {
		resolvedRoot, err := resolveExistingPrefix(root)
		if err != nil {
			return "", err
		}
		if pathWithin(resolvedRoot, resolvedTarget) {
			return target, nil
		}
	}
	return "", fmt.Errorf("browser output path %q is outside writable_roots", target)
}

// EnsurePrivateDir creates an owner-only directory where supported.
func EnsurePrivateDir(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	return protectPrivatePath(path, true)
}

// WritePrivateFile writes an owner-only browser artifact.
func WritePrivateFile(path string, data []byte) error {
	return writePrivateFile(path, data, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, false)
}

// WritePrivateFileExclusive creates an owner-only artifact without overwriting
// an existing path.
func WritePrivateFileExclusive(path string, data []byte) error {
	return writePrivateFile(path, data, os.O_CREATE|os.O_EXCL|os.O_WRONLY, true)
}

func writePrivateFile(path string, data []byte, flags int, removeOnFailure bool) error {
	file, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		if removeOnFailure {
			_ = os.Remove(path)
		}
		return err
	}
	if err := file.Close(); err != nil {
		if removeOnFailure {
			_ = os.Remove(path)
		}
		return err
	}
	if err := ProtectPrivateFile(path); err != nil {
		if removeOnFailure {
			_ = os.Remove(path)
		}
		return err
	}
	return nil
}

// ProtectPrivateFile applies the platform's current-user-only file policy.
func ProtectPrivateFile(path string) error {
	return protectPrivatePath(path, false)
}

// IsPrivatePath verifies the platform's current-user-only path policy.
func IsPrivatePath(path string, directory bool) (bool, error) {
	return isPrivatePath(path, directory)
}

func pathWithin(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func resolveExistingPrefix(path string) (string, error) {
	current := filepath.Clean(path)
	missing := make([]string, 0, 4)
	for {
		_, err := os.Lstat(current)
		if err == nil {
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", fmt.Errorf("resolve browser output symlink %q: %w", current, err)
			}
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return filepath.Clean(resolved), nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("inspect browser output path %q: %w", current, err)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return filepath.Clean(path), nil
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}
