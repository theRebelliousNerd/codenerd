---
doc-class: deep-dive
subsystem: knowledge-acquisition
implementation-status: proposed
last-verified: 2026-10-30
verified-against: 388205138ba6d47c1cff0c5e97c896dfa642a391
verification-method: static-source-inspection
---

# Preserve the documentation ref through Context7 acquisition

**Decision:** retain the discovered manifest's source locator and repository ref through linked-document acquisition, and give common-document fallback an explicit bounded ref strategy. Successfully identifying a repository does not establish that its documents live on `main`.

**Finding:** Context7 probes `master/llms.txt` but then resolves that manifest's relative links against `main`. Its common-document fallback also reads only `main`. The first seam can discard a successfully discovered manifest's branch identity; the second misses available documentation in a real supported master-only repository. Both reach normal expert knowledge acquisition.

**Evidence status:** static inspection of public CodeNERD main `388205138ba6d47c1cff0c5e97c896dfa642a391`, rechecked on 2026-10-30. Read-only GitHub metadata and source inspection verified the external Logrus input described below. No CodeNERD application, browser automation, test, knowledge acquisition, provider inference or benchmark was executed. The runtime consequences are source-predicted under the stated conditions, not reproduced outages.

## Existing fixes and ownership

This is not a report that CodeNERD's generic web-fetch links still lack a base. That defect is already repaired: the browser captures document URL and base URI; `web_fetch` uses `page.BaseURI`, and `resolveLink` resolves references and drops URL credentials. The existing research-observation audit records those changes as built. Preserve them. [Browser metadata](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/research/browser_read.go#L464-L489), [web-fetch base](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/research/web_fetch.go#L123-L131), [resolver](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/research/web_fetch.go#L339-L351), [as-built audit](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/research-observation-identity-and-admission.md#L218-L237).

Context7 takes a separate route. Its `fetchURL` obtains a browser page but returns only its content string. Its own manifest parser constructs raw repository URLs rather than using `web_fetch`'s repaired resolution path. A sound shared browser reader therefore does not repair this caller's discarded source identity. [Context7 page adapter](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/research/context7.go#L345-L373).

The 0850 major-version audit concerns mapping an exact dependency path into acquisition subjects. The 0919 observed-version audit concerns a receipt disagreeing with the version GoDoc actually read. Here the repository is correctly identified; the downstream documentation ref is lost or hardcoded. Fixing this does not make Context7 documentation dependency-version-pinned. Research additions through 0948 and neighboring knowledge/research audits were screened; this is a bounded source-specific continuation of source-identity principles, not an exhaustive novelty claim.

## The actual producer and consumers

### Repository inference works for the witness

`Context7Tool` exposes a topic, optional repository and maximum document count. `executeContext7` infers a repository when none is supplied, tries an llms manifest, then common documentation, and returns an error if neither supplies documents. The repository descriptor preserves host, owner and name, but not a ref. [Tool and dispatch](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/research/context7.go#L15-L108), [repository descriptor](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/research/context7.go#L118-L146).

Logrus is explicitly mapped to `sirupsen/logrus`. GitHub module paths also resolve to their first owner/repository components. CodeNERD's technology vocabulary recognizes `github.com/sirupsen/logrus`, so a normal matching manifest/domain can produce the typed module subject. This is not an unsupported arbitrary topic. [Inference](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/research/context7.go#L150-L198), [vocabulary](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/init/technology_vocabulary.go#L59), [subject binding](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/init/knowledge_sources.go#L77-L105).

### Manifest discovery drops its ref

The manifest candidates are:

1. `main/llms.txt`
2. `master/llms.txt`
3. `main/.llms.txt`

When one succeeds and exceeds the content threshold, `fetchLlmsTxt` calls `parseLlmsTxt(ctx, r, content, maxDocs)`. Neither the successful URL nor its ref is passed. For every non-absolute document link, the parser builds `rawFileURL(r, "main", docURL)`. [Discovery and handoff](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/research/context7.go#L226-L242), [relative-link construction](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/research/context7.go#L252-L298).

This contradicts the meaning of an ordinary relative link in a discovered manifest. Relative-reference resolution requires an established base; it should not silently change the branch component of the retrieval locator. The relevant principle is narrow, not a claim that the whole llms.txt ecosystem is governed by one repository layout. [RFC 3986 reference resolution](https://www.rfc-editor.org/rfc/rfc3986#section-5.1).

### Common documentation has a separate main-only fallback

If the manifest stage returns no documents, `fetchCommonDocs` tries README and five conventional documentation paths. All six URLs use `main`. It does not try `master`, retain a discovered ref, or discover the repository's default branch. Thus supporting master during manifest lookup does not support master during fallback. [Fallback implementation](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/research/context7.go#L307-L331).

### The source reaches expert acquisition

The background refresher invokes the registered `context7_fetch` tool through its modular registry. `Context7Source` sends the subject's module path when present, propagates fetch errors and turns successful sections into knowledge atoms. The acquirer records an ordinary fetch error as `failed`. Successful material can later enter expert JIT knowledge retrieval. [Boot adapter](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/knowledge_refresh_boot.go#L26-L47), [source adapter](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/sources.go#L119-L143), [outcome classification](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/acquire.go#L170-L188), [JIT consumer](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/compiler_db.go#L349-L386).

## Two witnesses with different evidence status

### Conditional master manifest witness

This is a proposed deterministic fixture, not a claim about Logrus:

- The main manifest candidate is absent.
- `master/llms.txt` contains a valid relative link such as `docs/guide.md`.
- The master guide exists and satisfies the parser's content threshold.
- The corresponding main guide and main fallback documents are absent.
- Browser admission, transport and caller lifetime are healthy; the repository remains stable during acquisition.

Discovery finds the master manifest, but the next read targets the main guide. The tool can therefore report no documentation despite having located a valid manifest and a document available at its correct relative locator.

A second fixture can put distinct guide bodies on both refs while only master has the manifest. The current parser then selects the main guide. Its emitted Source URL truthfully names main, so this is **wrong-ref selection**, not proof of a forged master provenance label. Main-based manifests and absolute document links are important controls: they need not fail.

### Verified Logrus fallback input

Read-only GitHub inspection at approximately 10:09 and again 10:14 UTC on 2026-10-30 established:

- The master branch resolves to `6ef0748d1177dadaeee9aa79ffef604570f487d7`.
- Looking up a branch named main returns 404.
- That pinned root tree contains `README.md`, but neither `llms.txt` nor `.llms.txt`.
- The README is substantial and comfortably exceeds the fallback's 100-character threshold.

The immutable tree and README preserve the positive file evidence; absence of the main branch is a dated branch-namespace observation, not something the commit alone proves. [Pinned root tree](https://github.com/sirupsen/logrus/tree/6ef0748d1177dadaeee9aa79ffef604570f487d7), [pinned README](https://github.com/sirupsen/logrus/blob/6ef0748d1177dadaeee9aa79ffef604570f487d7/README.md), [master branch metadata](https://api.github.com/repos/sirupsen/logrus/branches/master), [main branch lookup](https://api.github.com/repos/sirupsen/logrus/branches/main).

For this verified input state, source predicts that the three manifest candidates produce no documents and all six main-only fallback candidates miss. The available master README is never requested. This is not a claim that Logrus has a master llms manifest; it exercises the separate fallback seam.

No raw-page or CodeNERD acquisition was run. The prediction assumes ordinary GitHub raw-file resolution, unchanged branch state and successful browser access to the requested URLs. It should be validated with a frozen local fixture before any live experiment. A future upstream main branch or manifest could change the live example without changing the source defect.

## Existing safeguards and limits

**HTTP and no-document errors are correctly handled.** The browser reader refuses non-200 responses. Context7 returns an error when no documents were found, explicitly avoiding ingestion of a no-documentation message as researched knowledge. This audit must not be read as recommending acceptance of 404 bodies or conversion of failures into success. [HTTP handling](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/research/browser_read.go#L194-L230), [no-documents outcome](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/research/context7.go#L92-L97), [regression test](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/research/context7_no_docs_test.go#L10-L50).

**Host ownership is already repaired.** GitHub and Codeberg raw URL construction are distinct, and the Mangle mapping points to the owning Codeberg repository. A ref repair must keep that distinction rather than routing every document through GitHub. [Host-aware URL construction](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/research/context7.go#L202-L224), [host tests](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/research/context7_tool_test.go#L31-L104).

**Reads are controlled.** They use the owned browser runtime, a per-read timeout and a marked single-document byte guard. Do not add a free-form HTTP fallback, new credentials, an alternate browser manager or unrestricted reference enumeration to evade those controls. [Scoped research guidance](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/research/agents.md), [read limits](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/research/context7.go#L334-L373).

**Other knowledge can compensate.** GoDoc, grounded web and repository-source material may still cover the subject. This finding does not establish an empty expert KB, inevitable incorrect code, or a demonstrated prompt-quality loss.

**Retry is bounded, but can repeat the mismatch.** An available failed source becomes retry-eligible after one day. The refresher is scheduled hourly after its initial delay, selects at most four subjects, and Refresh asks all sources for a selected subject. A persistent Context7 ref miss can therefore cause repeat attempts and collateral re-acquisition from otherwise successful sources, but not an unconditional every-hour loop. Availability, age, priority, budget and session lifetime all matter. [Schedule and budget](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/knowledge_refresh.mg#L8-L26), [retry admission](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/knowledge_refresh.mg#L89-L94), [all-source refresh](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/acquire.go#L219-L225).

For the no-manifest fixture with default limits and no early cancellation, the source loops attempt three manifest reads and six fallback reads. That is an operation count derived from the code, not measured latency, network traffic or cost.

## A coherent repair

Carry a discovered-document descriptor, not just its content: requested locator, observed document/base locator where available, host/repository identity, selected ref and content. Resolve relative manifest references from that descriptor through the already established URL-resolution machinery. Reuse the browser's observed base metadata for redirects rather than manufacturing a new main URL.

For common-document fallback, choose an explicit policy:

- A caller-selected ref, when supported and authorized, is authoritative for that request.
- Otherwise, resolve a repository's default branch through an authorized metadata path, or use a documented bounded candidate order such as main then master.
- Carry the chosen ref through every candidate path. Do not mix documents from different branches under an undifferentiated result.
- Preserve absolute document references, source headers, host ownership, cancellation and failure reporting.
- Distinguish ref/path absence from transport, admission and authentication errors when the adapter can establish that distinction. A transient failure should not silently switch document identity.

This is documentation-location correctness, not dependency-version correctness. Reading a repository's default branch does not prove compatibility with the workspace's required module version. Explicit tag/commit matching would be a separate contract requiring actual version-to-ref evidence, including repositories whose module and release layouts differ.

Do not fix only the first main-only string. A selected master ref must survive discovery, linked reads, fallback, provenance and any cache key. If an implementation caches discovery, key it by host, repository and requested/resolved ref policy, with a defined refresh/invalidation rule. Negative discovery must not permanently hide later documentation.

## Migration and costs

The narrow ref handoff can reuse existing browser reads and should not require a new provider or a replacement knowledge schema. Its changes include the helper signatures, returned metadata, tests and caller provenance. Default-branch discovery adds metadata acquisition, storage/cache state and invalidation work; bounded multi-ref probing adds unsuccessful browser reads. Measure these alternatives rather than assuming discovery is free.

Fixing a previously failed source also adds useful work: successful documents produce sections, embeddings, SQLite writes and prompt candidates. It may reduce repeated failed discovery while increasing current acquisition cost. A fallback strategy that probes all refs and all paths can exceed the present nine-read negative case; bound attempts and report partial coverage explicitly. Do not silently equate the number of retained documents with a complete operation budget.

Legacy failed receipts can be retried through the existing bounded policy, without clearing all agents' acquisition history. Previously successful main documents cannot be relabeled as master: a main source URL may have been legitimate. Where provenance cannot establish the manifest/ref relationship, retain honest historical identity and reacquire targeted coverage rather than guessing. Keep prior good material on transient failures and preserve source/subject replacement ownership.

Operational reporting should distinguish no repository mapping, ref absent, document absent, blocked acquisition, transport failure and successful partial coverage where evidence supports those states. This improves diagnosis without claiming that every failure is permanent or that unknown outcomes are safe successes.

## Fair evaluation

All evaluation below is proposed and unrun. Freeze fixtures and assert exact attempted locators, not merely that some documentation text appears.

1. **Main manifest control:** main manifest and relative main guide remain successful.
2. **Master manifest witness:** main manifest absent, master manifest present, relative guide only on master. Baseline requests the wrong main guide; repair must request the master guide.
3. **Different bodies on both refs:** prove that the selected manifest's guide wins and that provenance names the actual document. Do not let any nonempty response satisfy the assertion.
4. **Absolute link control:** an absolute manifest reference remains unchanged.
5. **Master fallback witness:** no manifests, no main branch, master README present. Use a frozen analogue of the verified Logrus input; do not make live upstream state the regression oracle.
6. **Custom default ref and caller-selected ref:** test the declared supported policy; do not claim all arbitrary branch names work under a main/master-only design.
7. **Host and redirect controls:** GitHub versus Codeberg, redirected manifests, effective bases and credential stripping. Preserve current browser ownership and cleanup behavior.
8. **Failure controls:** all documents absent, forbidden read, missing runtime, timeout, cancellation, malformed links, short bodies and guard-limited content. No error page or no-documentation text becomes knowledge.
9. **Limits and retries:** count attempts and retained documents separately; test the one-day retry boundary, competing subjects under budget, unchanged repeat runs and discovery-cache invalidation.
10. **Production integration:** use the registered tool and knowledge adapter, record source outcomes, then verify that correct sections can be retrieved through the real expert path. A passing helper test is not proof of final prompt inclusion.

Existing tests exercise main-based manifests, main README fallback, maximum returned documents, repository inference, owning-host routing and no-document errors. They are valuable controls, but do not cover the master-ref transition. [Main manifest test](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/research/context7_tool_test.go#L9-L29), [main limit fixture](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/research/research_coverage_test.go#L204-L225), [main README fixture](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/research/research_coverage_test.go#L312-L327).

Measure browser admissions/tabs, attempted URLs, bytes, elapsed time, cleanup outcomes, provider calls caused by all-source retries, rows/embeddings and actual included source claims. Compare fixed-budget discovery strategies on matched repositories. If evaluating model results later, hold models, tasks, tools and verification budgets matched; separate improved source availability from downstream task success. No speedup, token saving or answer-quality gain has been measured.

## Branch review

At QA head `767a3291d6287b238417f2466592a9002810d05a`, Context7 source matched main. Older lane/QA heads use the earlier owner/repository helper signatures, but still probe master manifests and construct downstream relative/fallback paths on main; the host-aware refactor did not introduce or fix that behavior. The audit inventory through research head `4d0c0c64ddc5da4b87587c1bec4368ca7c70edd5` was screened. No competing ref-continuity fix was found in this bounded inspection.

