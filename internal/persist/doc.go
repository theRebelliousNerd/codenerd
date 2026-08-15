// Package persist is an umbrella for codeNERD's file-oriented fact
// persistence. It holds no code of its own; the work is split so that the
// encoding and the workspace layout can evolve independently:
//
//   - persist/factsnap — the codec: []types.Fact to SimpleColumn + gzip/zstd,
//     atomic writes, integrity sidecars, suffix and magic-byte detection.
//   - persist/snapshot — the workspace store: `.nerd/snapshots/` layout,
//     naming rules, listing and reference resolution.
//
// This is not the sqlite cold store (internal/store), campaign artifacts
// (internal/campaign) or session state. It is durable projection of facts the
// kernel has already decided are true, in a form that survives being copied
// between machines.
//
// Neither package asserts anything into a kernel. A snapshot becomes untrusted
// input the moment it leaves the process that wrote it, so rehydration stays
// with the caller, behind whatever policy that caller answers to.
package persist
