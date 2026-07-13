// Package prompt - Embedded corpus loader for baked-in prompt atoms.
// This file uses go:embed to bake prompt atoms into the binary at compile time,
// eliminating filesystem dependencies for built-in prompts.
package prompt

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"strings"

	"codenerd/internal/logging"
)

// embeddedAtoms contains all YAML files from atoms/ baked into the binary.
// The atoms directory is a subdirectory of this package.
//
//go:embed atoms
var embeddedAtoms embed.FS

// LoadEmbeddedCorpus loads the baked-in prompt atoms from the embedded filesystem.
// This is called at startup to initialize the JIT compiler with built-in atoms.
// Returns an EmbeddedCorpus containing all atoms from internal/prompt/atoms/.
func LoadEmbeddedCorpus() (*EmbeddedCorpus, error) {
	timer := logging.StartTimer(logging.CategoryStore, "LoadEmbeddedCorpus")
	defer timer.Stop()

	logging.Get(logging.CategoryStore).Info("Loading embedded prompt corpus")

	var allAtoms []*PromptAtom
	seen := make(map[string]string)

	// Walk the embedded filesystem
	err := fs.WalkDir(embeddedAtoms, "atoms", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Only process YAML files
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		// Read and parse the file
		atoms, parseErr := parseEmbeddedYAML(path)
		if parseErr != nil {
			return parseErr
		}
		for _, atom := range atoms {
			if previous, ok := seen[atom.ID]; ok {
				return fmt.Errorf("duplicate embedded atom id %q in %s and %s", atom.ID, previous, path)
			}
			seen[atom.ID] = path
		}

		allAtoms = append(allAtoms, atoms...)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk embedded atoms: %w", err)
	}
	if err := ValidatePromptAtomSet(allAtoms); err != nil {
		return nil, fmt.Errorf("invalid embedded atom set: %w", err)
	}

	logging.Get(logging.CategoryStore).Info("Loaded %d atoms from embedded corpus", len(allAtoms))

	return NewEmbeddedCorpus(allAtoms), nil
}

// parseEmbeddedYAML parses a YAML file from the embedded filesystem.
func parseEmbeddedYAML(path string) ([]*PromptAtom, error) {
	data, err := embeddedAtoms.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded file: %w", err)
	}

	parsed, migrations, err := ParsePromptAtomYAML(data, path, func(sourcePath, contentFile string) ([]byte, error) {
		return embeddedAtoms.ReadFile(pathpkgJoin(sourcePath, contentFile))
	})
	if err != nil {
		return nil, err
	}
	if len(migrations) > 0 {
		return nil, fmt.Errorf("embedded atom %s requires compatibility migration %s; built-in atoms must be canonical", migrations[0].AtomID, migrations[0].Code)
	}
	atoms := make([]*PromptAtom, 0, len(parsed))
	for _, record := range parsed {
		atoms = append(atoms, record.Atom)
	}
	return atoms, nil
}

// pathpkgJoin keeps the slash-separated namespace required by embed.FS.
func pathpkgJoin(sourcePath, relative string) string {
	return path.Join(path.Dir(sourcePath), filepath.ToSlash(relative))
}

// MustLoadEmbeddedCorpus loads the embedded corpus and panics on error.
// Use this for initialization where failure is unrecoverable.
func MustLoadEmbeddedCorpus() *EmbeddedCorpus {
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		panic(fmt.Sprintf("failed to load embedded corpus: %v", err))
	}
	return corpus
}

// GetEmbeddedAtomCount returns the number of atoms in the embedded corpus.
// Useful for diagnostics and testing.
func GetEmbeddedAtomCount() (int, error) {
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		return 0, err
	}
	return corpus.Count(), nil
}
