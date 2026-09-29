//go:build !windows

package world

// bulkFileRevs is the Windows directory query. Other systems already carry
// the content generation on the Stat result fileContentGen reads.
func bulkFileRevs(string) ([]goFileRev, bool, error) {
	return nil, false, nil
}
