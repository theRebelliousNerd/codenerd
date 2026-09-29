//go:build windows

package browser

import "testing"

var _ ProcessQuerier = (*windowsProcessQuerier)(nil)

func TestDecodeChromeProcesses(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		count       int
		bad         bool
	}{
		{"none", "", 0, false}, {"null", "null", 0, false},
		{"single", `{"ProcessId":10,"ParentProcessId":9,"CommandLine":"chrome"}`, 1, false},
		{"array", `[{"ProcessId":10,"ParentProcessId":9,"CommandLine":"chrome"}]`, 1, false},
		{"malformed", `{`, 0, true}, {"missing PID", `{}`, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeChromeProcesses([]byte(tc.input))
			if (err != nil) != tc.bad || len(got) != tc.count {
				t.Fatalf("decode = %v, %v", got, err)
			}
		})
	}
}
