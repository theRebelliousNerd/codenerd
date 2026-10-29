package sqlpragmas

import (
	"os"
	"strings"
	"sync/atomic"
)

// EnvHostClass names the environment variable that selects a host class when
// no process has called SetHostClass.
//
// Values: "workstation" (default), "laptop", "micro". Unrecognized values fall
// back to workstation rather than failing an open — a typo in an env var must
// never be the reason a database will not open.
const EnvHostClass = "NERD_SQL_HOST_CLASS"

// HostClass scales the memory budgets in the pragma presets.
//
// The presets were tuned for a workstation (large RAM, NVMe) and that stays
// the default: the numbers a workstation gets are byte-for-byte the ones this
// package has always emitted. Smaller classes divide the cache and mmap
// budgets so a laptop or a memory-capped container does not hand SQLite a
// 4 GiB page-cache ceiling on every one of a dozen open agent databases.
//
// Only cache_size and mmap_size scale. journal_mode, synchronous, temp_store,
// busy_timeout and wal_autocheckpoint are correctness/latency choices, not
// capacity choices, and are identical on every host.
type HostClass int

const (
	// HostWorkstation is the default: full workstation-class budgets.
	HostWorkstation HostClass = iota

	// HostLaptop quarters the memory budgets (Hot: 512 MiB cache, 2 GiB mmap).
	HostLaptop

	// HostMicro is for memory-capped containers and CI runners: one sixteenth
	// of the workstation budgets (Hot: 128 MiB cache, 512 MiB mmap).
	HostMicro
)

// String names the class for logs and for the host-class env var.
func (h HostClass) String() string {
	switch h {
	case HostLaptop:
		return "laptop"
	case HostMicro:
		return "micro"
	default:
		return "workstation"
	}
}

// divisor is the factor the workstation budgets are divided by.
func (h HostClass) divisor() int64 {
	switch h {
	case HostLaptop:
		return 4
	case HostMicro:
		return 16
	default:
		return 1
	}
}

// scale reduces a workstation-class byte/KiB budget for this host class.
func (h HostClass) scale(v int64) int64 {
	d := h.divisor()
	if d <= 1 {
		return v
	}
	return v / d
}

// ParseHostClass maps a configuration string to a HostClass. It is
// case-insensitive and tolerates surrounding whitespace. ok is false for
// unrecognized input so a config loader can report the typo; ActiveHostClass
// deliberately ignores that and keeps the default.
func ParseHostClass(s string) (HostClass, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "workstation", "desktop", "server":
		return HostWorkstation, true
	case "laptop", "notebook":
		return HostLaptop, true
	case "micro", "container", "ci", "small":
		return HostMicro, true
	default:
		return HostWorkstation, false
	}
}

// hostClassOverride holds an explicit SetHostClass value, offset by one so the
// zero value means "unset" and HostWorkstation stays expressible as an
// explicit choice that outranks the environment.
var hostClassOverride atomic.Int64

// SetHostClass pins the host class for every subsequent pragma application,
// overriding EnvHostClass.
//
// This exists so configuration can reach this package without this package
// importing configuration: internal/config (or a cmd) resolves the user's
// setting and pushes it down here at boot. Importing a config package from
// here would break the leaf invariant that lets mcp/prompt/core import
// sqlpragmas at all.
func SetHostClass(h HostClass) {
	hostClassOverride.Store(int64(h) + 1)
}

// ClearHostClass drops an explicit SetHostClass, returning resolution to
// EnvHostClass. Mainly for tests and for a config reload that removed the key.
func ClearHostClass() {
	hostClassOverride.Store(0)
}

// ActiveHostClass resolves the host class in effect: an explicit
// SetHostClass wins, then EnvHostClass, then HostWorkstation.
//
// The environment is read on every call rather than cached at init so a
// long-running process (and a test) sees a changed value without a restart;
// opens are rare enough that a getenv per open is free.
func ActiveHostClass() HostClass {
	if v := hostClassOverride.Load(); v != 0 {
		return HostClass(v - 1)
	}
	if raw, ok := os.LookupEnv(EnvHostClass); ok {
		if h, parsed := ParseHostClass(raw); parsed {
			return h
		}
	}
	return HostWorkstation
}
