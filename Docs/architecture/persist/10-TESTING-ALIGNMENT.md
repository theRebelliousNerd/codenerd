# 10 — Testing Alignment: persist

> Last verified against codebase: **2026-07-13**

## 1. Commands

```powershell
go test ./internal/persist/...
go test ./internal/persist/factsnap/ -v
go test ./internal/persist/factsnap/ -v -run TestCodecParity
go test ./internal/persist/factsnap/ -v -run TestSizeComparison
```

No CGO required for this package (unlike full `nerd.exe` sqlite-vec builds).

## 2. Test inventory

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

Helpers shared across tests: `sampleFacts`, `sortFacts`, `equalishFacts`, `normalizeArg`.

## 3. Coverage shape

| Area | Coverage quality |
|------|------------------|
| Happy-path codecs | **Strong** |
| Path rewriting | **Strong** (table-driven) |
| Legacy JSON | **Strong** |
| Failure cleanup | **Good** (unknown codec) |
| Partial disk full / rename fail | **Weak** (hard to unit-test; no mocks) |
| Concurrent writers | **None** |
| Integration with kernel Assert | **None** (no production caller) |
| Exotic `ToAtom` types (time, duration, bool multi-hop) | **Weak** (sampleFacts uses common shapes) |

## 4. Alignment with code

| Production branch | Tested? |
|-------------------|---------|
| Gzip write/read | Yes |
| Zstd write/read | Yes |
| Auto codec | Yes |
| Unknown codec | Yes |
| JSON via `Read` | Yes |
| `LegacyJSON` | Yes |
| `CanonicalPath` | Yes |
| `collectFacts` nil store | Defensive only (not unit-targeted) |
| `MkdirAll` failure | Not forced |

## 5. Gaps and recommended tests (docs backlog)

1. **Integration**: first real caller package should own export→import→assert.  
2. **Bool / float multi-hop** explicit cases.  
3. **Empty slice** write/read.  
4. **Corrupt gzip** body returns error (may already via store loaders; assert message).  
5. **Cross-process rename** stress — optional, OS-specific.

## 6. Test:source ratio

Approximately **458** test lines vs **287** source lines (~1.6:1). For a pure library this is healthy; the missing layer is **system** tests, not more unit cases for the same API.

## 7. CI expectation

`go test ./internal/persist/...` should stay green on every PR that touches:

- `internal/persist/**`
- `internal/types` Fact/ToAtom semantics
- `mangle-go` factstore SimpleColumn APIs
