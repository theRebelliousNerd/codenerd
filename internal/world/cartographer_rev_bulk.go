//go:build !windows

package world

// bulkFileRevs is the Windows directory query. Other systems already carry
// the content generation on the Stat result fileContentGen reads.
func bulkFileRevs(string) ([]goFileRev, bool, error) {
	return nil, false, nil
}

// dirContentGens is the Windows directory query. fileContentGen reads the
// same clock off the Stat result here, so there is no bulk snapshot.
func dirContentGens(string) (map[string]fileGen, bool) {
	return nil, false
}
