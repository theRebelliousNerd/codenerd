package session

import (
	"errors"
	"io/fs"
	"os"

	"codenerd/internal/observation"
)

// withTurnWrites records on the return each path the turn wrote, with the
// preimage the executor took before the turn's first write to it and what the
// path holds now that the turn has ended. A caller that decides the turn
// failed -- a campaign attempt that did not end /done -- undoes exactly these
// writes, including the ones outside anything it declared (ladder C4: a
// rollback of the declared write set kept the coder's edit to a file outside
// it, which imported the file the rollback removed, and the tree stopped
// building).
func withTurnWrites(ret observation.Return, workspace string, res *ExecutionResult) observation.Return {
	if res == nil {
		return ret
	}
	for _, p := range res.WrittenPaths {
		path := turnFilePath(workspace, p)
		w := observation.FileWrite{Path: path, After: readFileState(path)}
		if pre, ok := res.PreWriteContents[p]; ok && pre.Known() {
			w.Before = observation.FileState{Known: true, Exists: pre.Existed, Content: pre.Content}
		}
		ret.Writes = append(ret.Writes, w)
	}
	return ret
}

func readFileState(path string) observation.FileState {
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		return observation.FileState{Known: true, Exists: true, Content: string(data)}
	case errors.Is(err, fs.ErrNotExist):
		return observation.FileState{Known: true}
	default:
		return observation.FileState{}
	}
}
