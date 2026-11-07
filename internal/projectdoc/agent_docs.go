package projectdoc

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	wscope "codenerd/internal/workspace"
)

// Agent-doc twins. A directory's agents.md (or AGENTS.md) is the one source of
// its contributor guidance. Two harnesses read the same guidance under other
// names: Claude Code reads CLAUDE.md, and codeNERD reads the Markdown body of
// nerd.md. Both are generated from agents.md so the three never disagree:
//
//   - CLAUDE.md is a byte-identical copy. The repository gitignores CLAUDE.md
//     (local model prompt files), so these twins are local outputs.
//   - nerd.md keeps its own frontmatter, which is enforced policy and is never
//     generated, and takes agents.md as its body. A directory without a
//     nerd.md gets the minimal frontmatter the schema requires.
//
// Harness mirror trees (.claude, .codex, .grok, .agents) hold skill files for
// one harness each; their AGENTS.md files have no twins.

// AgentDocGenerator is the nerd.md generator id that writes the twins.
const AgentDocGenerator = "agent-docs"

// ClaudeFileName is the Claude Code twin of an agents.md.
const ClaudeFileName = "CLAUDE.md"

// harnessMirrors are workspace-root directories whose AGENTS.md files belong to
// one harness and get no twins.
var harnessMirrors = map[string]bool{".claude": true, ".codex": true, ".grok": true, ".agents": true}

// AgentDocTwin is one generated file and the bytes it must hold.
type AgentDocTwin struct {
	// Source is the agents.md it is generated from, workspace-relative POSIX.
	Source string
	// Path is the twin, workspace-relative POSIX.
	Path string
	// Local is true for a twin the repository does not track (CLAUDE.md).
	Local bool
	// Want is the twin's content.
	Want []byte
}

// AgentDocTwins lists every twin the workspace's agents.md files call for,
// sorted by path. A nerd.md whose frontmatter does not parse is an error: the
// generator must not write a body under rules the kernel would refuse.
func AgentDocTwins(workspace string) ([]AgentDocTwin, error) {
	sources, err := agentDocSources(workspace)
	if err != nil {
		return nil, err
	}
	var twins []AgentDocTwin
	for _, rel := range sources {
		src, err := os.ReadFile(filepath.Join(workspace, filepath.FromSlash(rel)))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", rel, err)
		}
		dir := filepath.ToSlash(filepath.Dir(rel))
		twins = append(twins, AgentDocTwin{
			Source: rel,
			Path:   joinRel(dir, ClaudeFileName),
			Local:  true,
			Want:   src,
		})
		nerdRel := joinRel(dir, FileName)
		existing, err := os.ReadFile(filepath.Join(workspace, filepath.FromSlash(nerdRel)))
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("read %s: %w", nerdRel, err)
		}
		want, err := nerdTwin(existing, src)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", nerdRel, err)
		}
		twins = append(twins, AgentDocTwin{Source: rel, Path: nerdRel, Want: want})
	}
	sort.Slice(twins, func(i, j int) bool { return twins[i].Path < twins[j].Path })
	return twins, nil
}

// StaleAgentDocTwins names the twins whose content differs from what their
// agents.md calls for. A local twin (CLAUDE.md) that does not exist is not
// stale: it is generated on demand and never tracked.
func StaleAgentDocTwins(workspace string) ([]string, error) {
	twins, err := AgentDocTwins(workspace)
	if err != nil {
		return nil, err
	}
	var stale []string
	for _, t := range twins {
		got, err := os.ReadFile(filepath.Join(workspace, filepath.FromSlash(t.Path)))
		if os.IsNotExist(err) && t.Local {
			continue
		}
		if err != nil || !bytes.Equal(got, t.Want) {
			stale = append(stale, t.Path)
		}
	}
	return stale, nil
}

// WriteAgentDocTwins writes every twin whose content differs and returns the
// paths it wrote. Unchanged twins are not touched.
func WriteAgentDocTwins(workspace string) ([]string, error) {
	twins, err := AgentDocTwins(workspace)
	if err != nil {
		return nil, err
	}
	var written []string
	for _, t := range twins {
		abs := filepath.Join(workspace, filepath.FromSlash(t.Path))
		if got, err := os.ReadFile(abs); err == nil && bytes.Equal(got, t.Want) {
			continue
		}
		if err := os.WriteFile(abs, t.Want, 0o644); err != nil {
			return written, fmt.Errorf("write %s: %w", t.Path, err)
		}
		written = append(written, t.Path)
	}
	return written, nil
}

// agentDocSources lists the workspace's agents.md and AGENTS.md files that are
// workspace members, outside the harness mirror trees.
func agentDocSources(workspace string) ([]string, error) {
	mem, err := wscope.For(workspace)
	if err != nil {
		return nil, err
	}
	if err := mem.Refresh(); err != nil {
		return nil, err
	}
	var out []string
	err = filepath.WalkDir(workspace, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		rel, relErr := filepath.Rel(workspace, p)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() && rel != "." && harnessMirrors[strings.SplitN(rel, "/", 2)[0]] {
			return filepath.SkipDir
		}
		member, admErr := mem.Admit(p, d.IsDir())
		if admErr != nil {
			return admErr
		}
		if !member {
			if d.IsDir() && rel != "." {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() && strings.EqualFold(d.Name(), "agents.md") {
			out = append(out, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// nerdTwin is the nerd.md a directory needs: existing's frontmatter block,
// byte for byte, or the minimal one, followed by a blank line and body. The
// result must parse, or the kernel would refuse every nerd.md at boot.
func nerdTwin(existing, body []byte) ([]byte, error) {
	front := []byte("---\nschema: " + SchemaVersion + "\n---\n")
	if len(existing) > 0 {
		block, err := frontmatterBlock(existing)
		if err != nil {
			return nil, err
		}
		front = block
	}
	out := append(append(append([]byte(nil), front...), '\n'), body...)
	if _, err := Parse(out); err != nil {
		return nil, fmt.Errorf("generated nerd.md does not parse: %w", err)
	}
	return out, nil
}

// frontmatterBlock returns data's leading `---` fenced block through the end
// of its closing fence line, terminator included.
func frontmatterBlock(data []byte) ([]byte, error) {
	off := 0
	fences := 0
	for off < len(data) {
		end := bytes.IndexByte(data[off:], '\n')
		next := len(data)
		if end >= 0 {
			next = off + end + 1
		}
		line := strings.TrimSpace(string(data[off:next]))
		if off == 0 && line != frontmatterFence {
			return nil, fmt.Errorf("first line must be %q to open the frontmatter block", frontmatterFence)
		}
		if line == frontmatterFence {
			fences++
			if fences == 2 {
				block := append([]byte(nil), data[:next]...)
				if end < 0 {
					block = append(block, '\n')
				}
				return block, nil
			}
		}
		off = next
	}
	return nil, fmt.Errorf("frontmatter block opened with %q was never closed", frontmatterFence)
}

func joinRel(dir, name string) string {
	if dir == "." || dir == "" {
		return name
	}
	return dir + "/" + name
}
