# S0 shared run accounting under #124

The [#138 guard](s0-request-controls.md) now has a shared run ledger. New client processes replay the same recorded usage; they cannot silently renew allowances or reuse an already started slot. Work stays in [#124](https://github.com/andikapradanaarif/ganesh/issues/124). No real model calls, installed-client changes or S0 answers ran.

## Accounting contract

[RunLimits](../internal/devtools/requestguard/run.go) freezes the run ID, allowed slots and exact profiles, aggregate provider-request/output ceilings, an input-usage stop threshold and an absolute deadline. Exclusive creation returns the SHA-256 of the header; every process supplies that frozen path/digest and its allowed slot. The client never initializes a missing run or falls back to process-only counters. A changed profile, unknown/reused slot, altered header, unsafe file or malformed/truncated record is refused.

A nonblocking OS file lock spans each request's synced start through its finish. Admission reserves one provider request and enough remaining output allowance for its full requested cap. A completed reply charges its actual native input/output usage; unused output reservation becomes available to later requests. Auxiliary calls and failed attempts use the same ledger. One request in flight is enforced across bound processes; a competing process is refused before forwarding and does not interrupt the admitted request.

Contention during client admission is reported as `ErrRunBusy` / `shared_run_busy`, separately from a journal/storage failure. Slots identify processes, not provider requests: any admitted start permanently consumes its slot, including one that fails or is interrupted. Adoption must enumerate answer/grading processes and any explicitly authorized repeats within the 1,000-slot bound. Spare capacity does not authorize retries or create failure headroom automatically.

An unfinished start remains potentially dispatched with unknown usage, even after a killed process releases its lock. Unknown usage or a failed request stops continuation. There is no automatic repair, refund or restart that guesses consumption. Per-process journals remain separate; a local accounting failure before a shared finish conservatively leaves the shared request unfinished. A completed slot is not an accepted answer: slots, provider requests, token usage and S0 scores are separate quantities.

Responses and bounded non-streaming, single-choice Chat Completions can share the ledger. Chat grading requires `max_tokens`, a final `stop` choice and native `prompt_tokens`/`completion_tokens`; streaming, conflicting output-cap fields, multiple choices, truncation and missing usage are rejected. These are protocol/accounting checks, not acceptance of a grader's judgments. The exact future DeepSeek request/profile still needs adoption with the complete run inputs.

The frozen deadline governs admission and bounds forwarded HTTP contexts across processes. Input usage is a **post-response stop**, not a pre-request tokenizer/context bound: the response that reaches or exceeds the threshold is charged and stops continuation. Requested-output limits cannot prove provider billing. Only callers bound to this ledger are covered; SDK-managed transports and unrelated clients remain outside qualification.

## Offline evidence

The [Go fixture command](../integration/crush/requestprofile/README.md) runs 13 actual-client cases against the pinned private Crush build: the prior five cases, refusal of a configuration without a shared run, and seven cases below. Each case has its own synthetic frozen run; shared cases keep one ledger across their fresh child processes. Every child uses isolated state, dummy credentials and the existing OS loopback-only sandbox.

| Shared case | Provider requests | Known input/output | Outcome |
| --- | ---: | ---: | --- |
| Four-request allowance across three processes | 4 | 44 / 12 | Two sessions succeed; third exits 78 before forwarding |
| Output reservation across two processes | 3 | 33 / 9 | Second session's main request cannot fit the remaining allowance |
| Aggregate input stop | 3 | 33 / 9 | Second session reaches the frozen threshold; continuation stops |
| Unknown usage | 1 | Unknown | First 503 stops both the first and next session |
| Expired run deadline | 0 | 0 / 0 | Both fresh sessions exit 78 before forwarding |
| Reused slot | 2 | 22 / 6 | Second process cannot restart the first process's slot |
| Concurrent contender, killed holder, recovery attempt | 1 | Unknown | Contender is refused; kill leaves one unfinished reservation; recovery is refused |

The usage values are stub declarations, not real inference measurements. Mixed-protocol Go process tests additionally send one Responses answer and one Chat grade through the same two-request run: known totals are **18 input / 5 output**, with the next grading process denied. Tests cover header/profile/slot binding, corruption, private file permissions, symlinks/hardlinks, exclusive initialization, accounting faults and the shared deadline on the forwarded context.

Private post-review report SHA-256: `e225346696c4c1dd7a71bdeb7ab6f0ab33a53e75b4584e789e440b6155ea23d8`.

Private candidate binary SHA-256: `feca4b941545cd83f4e727109352ffa087f62524effd6cabee8bed9947e050fb`. Source pin remains `86897c56a304bac133650a6055ab1d091e6b8f83`, built with Go 1.27.4. The report binds all four guard source files, adapter/client-source changes, observed requests, local journals and shared ledgers. The contention-reason fix is included; regression tests distinguish it from storage faults and verify that the admitted request remains intact. Earlier private fixtures and #138's evidence are unchanged. Real inference requests: **zero**.

## Remaining limits

This is local accounting on a trusted private filesystem with working `flock`/`fsync`, supported on macOS/Linux. Other platforms refuse shared accounting. It does not protect against an owner rewriting files or restoring an older valid ledger, and does not claim distributed-filesystem or power-loss guarantees beyond those primitives. Missing/torn records are never silently repaired. Timings, optional blocked-event counts and report digests can vary between retained fixture runs.

The real 108 base + conditional 24 schedule and all scoring controls remain unchanged. This is not a dispatcher or live-run permission: final task/oracle adoption, natural grading acceptance, exact answer/grader/client/tool profiles and source/processor permissions, complete context fit and one approved whole-run allowance remain pending. No fixture ceiling becomes the owner's real budget. Existing frozen packets, labels, failures and baseline documents stay unchanged; overall S0 effort and conversation usage remain unmeasured. S0 is still **0/108**.
