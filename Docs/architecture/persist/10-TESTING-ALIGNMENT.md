# 10 — Testing Alignment: persist

> Last verified against codebase: **2026-08-15**

## 1. Commands

```bash
go test ./internal/persist/...
go test ./internal/persist/factsnap/ -v
go test ./internal/persist/factsnap/ -v -run TestCodecParity
go test ./internal/persist/factsnap/ -v -run TestSizeComparison
go test ./internal/persist/snapshot/ -v
go test ./cmd/nerd/ -run TestSnapshot
```

No CGO required for the library packages. The snapshot round-trip test and the
`cmd/nerd` tests boot a real `core.RealKernel` (embedded constitution only — no
LLM, no network), which costs a few hundred milliseconds each.

## 2. Test inventory

### `internal/persist/factsnap`

| Test | File | Focus |
|------|------|-------|
| `TestRoundTripGzip` | `factsnap_test.go` | 1000-fact gzip fidelity + canonical path exists |
| `TestRoundTripZstd` | `factsnap_test.go` | 1000-fact zstd fidelity |
| `TestLegacyJSONFallback` | `factsnap_test.go` | `Read` accepts `.json` |
| `TestSizeComparison` | `factsnap_test.go` | gzip < JSON; logs sizes |
| `TestWriteCodec_Auto` | `factsnap_codec_test.go` | Auto → `.sc.gz` |
| `TestWriteCodec_UnknownCodec` | `factsnap_codec_test.go` | error + no residual files |
| `TestCodecParity` | `codec_parity_test.go` | 100 / 1k / 10k; magic headers; semantic equality |
| `TestCodecParity_PathRewriting` | `codec_parity_test.go` | `CanonicalPath` table |
| `TestLegacyJSONDirect` | `legacy_test.go` | helper success + missing + malformed |
| `TestRead_WhenSuffixStripped_ShouldSniffGzipMagic` | `factsnap_robustness_test.go` | magic-byte recovery |
| `TestRead_WhenSuffixStripped_ShouldSniffZstdMagic` | same | magic-byte recovery |
| `TestRead_WhenSuffixContradictsContent_ShouldTrustContent` | same | content beats filename |
| `TestRead_WhenJSONHasNoMagic_ShouldStillUseLegacyPath` | same | sniff does not break JSON |
| `TestWrite_WhenDefaultOptions_ShouldEmitSha256Sidecar` | same | sha256sum(1) shape + `Verify` |
| `TestRead_WhenSnapshotTruncated_ShouldFailIntegrityCheck` | same | `ErrIntegrity`, fail-closed |
| `TestWrite_WhenNoSidecarRequested_ShouldRemoveStaleSidecar` | same | no poisoned leftovers |
| `TestWrite_WhenWritersRaceOnOnePath_ShouldLeaveOneReadableSnapshot` | same | 8 concurrent writers, no temp residue |
| `TestWrite_WhenFactsEmpty_ShouldRoundTripToEmpty` | same | empty slice and nil |
| `TestBool_WhenRoundTripped_ShouldBecomeNameConstantAndThenStayStable` | same | bool multi-hop |
| `TestFloat_WhenRoundTrippedTwice_ShouldBeStableAfterFirstHop` | same | float multi-hop, incl. whole-float degradation |
| `TestNameConstant_WhenRoundTripped_ShouldStayMangleAtom` | same | pins the core divergence |

### `internal/persist/snapshot`

| Test | File | Focus |
|------|------|-------|
| `TestExport_WhenNamedSnapshot_ShouldLandUnderNerdSnapshots` | `snapshot_test.go` | canonical path + sidecar |
| `TestImport_WhenReferencedByBareName_ShouldResolveExtension` | same | bare-name resolution across codecs |
| `TestImport_WhenReferenceUnknown_ShouldFailWithDirectoryHint` | same | error names the directory |
| `TestSanitizeName_WhenNameEscapesDirectory_ShouldReject` | same | containment |
| `TestExport_WhenNameEscapesDirectory_ShouldNotWriteOutsideSnapshots` | same | containment, on disk |
| `TestList_WhenDirectoryMissing_ShouldReturnEmptyNotError` | same | fresh workspace |
| `TestList_WhenSnapshotsExist_ShouldSkipSidecarsAndReportCodec` | same | listing hygiene |
| `TestSummarize_WhenMixedPredicates_ShouldOrderByCountThenName` | same | histogram order |
| `TestCodecFor_WhenAliasGiven_ShouldMapOrReject` | same | CLI codec aliases |
| `TestSnapshotRoundTrip_WhenReloadedIntoFreshKernel_ShouldRestoreSameFacts` | `kernel_roundtrip_test.go` | **the integration**: domain → facts → kernel → snapshot → kernel |
| `TestSnapshotRoundTrip_WhenSnapshotCorrupted_ShouldRefuseToImport` | same | corruption stops at the boundary |

### `cmd/nerd`

| Test | Focus |
|------|-------|
| `TestSnapshotExport_WhenWorkspaceBooted_ShouldWriteReadableSnapshot` | export writes a file its own import can read |
| `TestSnapshotImport_WhenToMangleRequested_ShouldWriteSortedDatalog` | rendered Datalog is sorted and provenance-stamped |
| `TestSnapshotImport_WhenAssertRequested_ShouldLoadIntoLocalKernelOnly` | `--assert` reports a delta and writes nothing to the workspace |
| `TestSnapshotImport_WhenReferenceMissing_ShouldErrorNotPanic` | missing snapshot |
| `TestSnapshotList_WhenNoSnapshots_ShouldSuggestExport` | empty-state guidance |
| `TestSnapshotExport_WhenPredicateUnknown_ShouldFailLoudly` | never write an empty snapshot silently |
| `TestHumanBytes_WhenScaling_ShouldPickUnit` | size formatting |

Helpers shared across factsnap tests: `sampleFacts`, `sortFacts`,
`equalishFacts`, `normalizeArg`, `roundTrip`.

## 3. Coverage shape

| Area | Coverage quality |
|------|------------------|
| Happy-path codecs | **Strong** |
| Path rewriting / detection | **Strong** (suffix, missing suffix, lying suffix) |
| Legacy JSON | **Strong** |
| Integrity sidecar | **Strong** (write shape, verify, truncation, stale removal) |
| Failure cleanup | **Good** (unknown codec, contended writes) |
| Concurrent writers | **Good** (in-process); cross-process untested |
| Integration with a kernel | **Good** (`kernel_roundtrip_test.go`) |
| Operator surface | **Good** (`cmd/nerd/cmd_snapshot_test.go`) |
| Exotic `ToAtom` types | **Good** for bool/float/name; `time.Time`/`Duration` still untested |
| Disk-full / rename failure | **Weak** (no fault injection) |

## 4. Gaps and recommended tests (docs backlog)

1. `time.Time` / `time.Duration` multi-hop cases.
2. Fault injection for rename/sync failure (needs an FS seam that does not exist).
3. Cross-process contention — OS-specific, currently documented rather than tested.

## 5. Test:source ratio

Roughly **1,050** test lines against **810** source lines across `factsnap`,
`snapshot` and the CLI command (~1.3:1), and the missing layer is no longer
system tests — those exist now.

## 6. CI expectation

`go test ./internal/persist/... ./cmd/nerd/` should stay green on every PR that
touches:

- `internal/persist/**`
- `cmd/nerd/cmd_snapshot.go`
- `internal/types` Fact/ToAtom semantics
- `mangle-go` factstore SimpleColumn APIs
