package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// removedFeatureKeys names keys that were once valid inside the `features`
// block of .nerd/config.json and are now gone, with the reason an operator
// needs in order to act.
//
// decodeStrictJSON would already refuse the file — it disallows unknown
// fields — but its message ("unknown field \"diff_eval\"") reads like a typo
// and tells the operator nothing about why the key vanished or whether their
// configuration still does what they meant. A removed toggle is a behaviour
// change, so the failure names the key and says what happened to it.
var removedFeatureKeys = map[string]string{
	"diff_eval": "the differential evaluation path was deleted: it could not " +
		"un-derive a fact whose negated premise became true, and it accumulated " +
		"aggregate results instead of replacing them. The kernel now always " +
		"rebuilds the store from the EDB, which is what diff_eval=false already " +
		"did. Delete this key",
}

// rejectRemovedKeys fails a config load that still carries a key this
// codebase no longer honours. It runs BEFORE decodeStrictJSON so the specific
// explanation wins over the strict decoder's generic unknown-field error.
//
// Malformed JSON is not this function's problem: it returns nil and lets
// decodeStrictJSON produce the parse error.
func rejectRemovedKeys(data []byte) error {
	var envelope struct {
		Features map[string]json.RawMessage `json:"features"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil
	}
	var found []string
	for key := range envelope.Features {
		if _, removed := removedFeatureKeys[key]; removed {
			found = append(found, key)
		}
	}
	if len(found) == 0 {
		return nil
	}
	sort.Strings(found)
	reasons := make([]string, 0, len(found))
	for _, key := range found {
		reasons = append(reasons, fmt.Sprintf("features.%s is no longer a supported key: %s", key, removedFeatureKeys[key]))
	}
	return errors.New(strings.Join(reasons, "; "))
}
