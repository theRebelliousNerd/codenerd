package init

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"codenerd/internal/orient"
	"codenerd/internal/types"
)

// OrientationDocuments refreshes the persisted orientation and returns the
// documents its policy derived as worth reading (orient_read_candidate), with
// the bodies the census measured. It is the one document set knowledge
// extraction reads: no caller keeps a filename list of its own. The refresh
// result is returned so a caller with a running kernel can publish the
// changed projection (RefreshResult.Apply). A workspace that was never
// oriented is an error naming nerd init. A read candidate whose body was not
// measured is named in the error, and the documents that were measured are
// still returned beside it.
func (i *Initializer) OrientationDocuments(ctx context.Context) ([]DocumentInfo, *orient.RefreshResult, error) {
	root := i.config.Workspace
	refresh, err := orient.Refresh(ctx, root, i.embedEngine)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil, fmt.Errorf("workspace has no orientation snapshot; run nerd init: %w", err)
		}
		return nil, nil, fmt.Errorf("refresh orientation: %w", err)
	}
	snapshot, err := orient.LoadSnapshot(root)
	if err != nil {
		return nil, refresh, fmt.Errorf("load orientation snapshot: %w", err)
	}
	eng, err := snapshot.Engine(ctx)
	if err != nil {
		return nil, refresh, err
	}
	defer eng.Close()
	rows, err := eng.Query("orient_read_candidate")
	if err != nil {
		return nil, refresh, err
	}
	var docs []DocumentInfo
	seen := map[string]bool{}
	var missing []string
	for _, row := range rows {
		path := types.ExtractString(row.Args[0])
		if seen[path] {
			continue
		}
		seen[path] = true
		body, ok := snapshot.Bodies[path]
		if !ok {
			missing = append(missing, path)
			continue
		}
		docs = append(docs, DocumentInfo{
			Path:        path,
			AbsPath:     filepath.Join(root, filepath.FromSlash(path)),
			Content:     body,
			Title:       documentTitle(path, body),
			Size:        len(body),
			IsRelevant:  true,
			Reasoning:   "orientation read candidate",
			ContentHash: computeDocHash(body),
		})
	}
	if len(missing) > 0 {
		return docs, refresh, fmt.Errorf("orientation read candidates without a measured body: %s", strings.Join(missing, ", "))
	}
	return docs, refresh, nil
}

// documentTitle is the first Markdown heading, or the file name.
func documentTitle(path, body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			if title := strings.TrimSpace(strings.TrimLeft(line, "#")); title != "" {
				return title
			}
		}
	}
	return filepath.Base(path)
}
