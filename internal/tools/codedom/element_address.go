package codedom

import "context"

// ElementAt resolves ref -- with path, or a ref that names its file, or one the
// structure index resolves to exactly one element -- to the element's file and
// its key within that file (codemodel.Element.Key). The working set files an
// observation of an element under that address and dates it by the element's
// own revision, so an edit elsewhere in the file leaves it current.
func ElementAt(ctx context.Context, ref, path string) (file, key string, err error) {
	re, err := resolveElement(ctx, ref, path)
	if err != nil {
		return "", "", err
	}
	return re.file.rel, re.elem.Key, nil
}
