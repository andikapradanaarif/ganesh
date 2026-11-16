# S0 answering request controls under #124

The [#137 failure](s0-execution-profile.md#installed-answering-client-check) now has an executable fix in a **private experimental Crush build**. The [Go guard](../internal/devtools/requestguard/guard.go) covers native Responses requests through the shared instrumented transport, including auxiliary calls. The [pinned-build fixture and command](../integration/crush/requestprofile/README.md) reproduce the offline checks without changing the installed client or external source checkout. No real provider calls or original S0 answers ran.

The guard serializes requests, caps dispatch count and requested/total output, forbids identical payload replay, and stops on failed, interrupted or unaccounted replies. It syncs a private metadata-only start before forwarding and a finish before returning success. Unknown usage remains null; known totals do not imply that unknown requests cost zero. Plain HTTP/1 transports use fresh connections without environment proxies or replay hints. Custom OAuth/debug transports are rejected. Bounded responses are buffered until native JSON/SSE completion and usage have been checked; incremental SDK delivery is therefore changed.

The mandatory build adapter terminates the isolated session with exit **78** after a guard failure. This prevents the SDK's otherwise continuing local retries/backoff and fallback. Missing/invalid guard configuration also denies dispatch. This behavior is for one experimental session per process; partial stdout after a failure is not an accepted answer. The installed ordinary client still has its original behavior.

## Measured offline result, 2026-11-16

All five actual-client fixtures pass using the same source pin as #137, `86897c56a304bac133650a6055ab1d091e6b8f83`, built with Go 1.27.4. Synthetic prompts, dummy credentials, isolated state and an OS loopback-only network sandbox were mandatory.

| Fixture | Requests reaching the stub | Usage accounting | Process result |
| --- | ---: | --- | --- |
| HTTP 503 | 1 | 1 unknown; no zero estimate | Exit 78, 2.765 s |
| Successful SSE | 2 | Both known: stub-declared totals 22 input / 6 output | Exit 0, answer returned, 0.558 s |
| Missing usage | 1 | 1 unknown; continuation blocked | Exit 78, 0.482 s |
| One-request cap | 1 | First known; next attempt recorded as blocked | Exit 78, 0.494 s |
| Missing guard configuration | 0 | No request ledger created | Exit 78, 0.382 s |

Peak observed in-flight requests was one. No fixture reached its 35-second launcher deadline. The successful case accounts for both observed requests (requested output 40 and 256) through the shared guard; roles are not inferred from payload sizes alone. The declared 11/3 per-reply usage is synthetic accounting input, not measured tokenizer output or provider billing. Timings and optional `blocked_events` counts can vary with scheduling during exit-78 teardown; reproducibility means the stated behavior/assertions pass, not that a new report has identical bytes or a matching digest. Published hashes identify one retained run.

Private report SHA-256: `8e3b07fc2b0e6fb33b732bb257c3077ae0a41bb72f3d3e305d9be65cdf96c986`.

Private candidate binary SHA-256: `f63143aba1374081d887ecd1a6d4a2306807cbd241eda63a68ca26c3bf167368`.

This rebuild includes the review fix for an empty blocked reason when the guard expires before its first request. A regression checks zero attempts, zero requests with unknown usage, and agreement between the journal reason and snapshot. Earlier private reports remain unchanged.

The report joins guard/adapter/original-and-patched-client-source hashes, binary identity, all observed request hashes and private journals. Earlier development fixtures exposed a cancellation-test cleanup defect, Git archive PAX handling and SDK backoff after transport rejection. Those were corrected; earlier private outputs remain separate. The final fixture includes the fail-fast adapter, not only the earlier traffic-blocking version.

## Remaining scope

Coverage applies to the shared instrumented provider transport. SDK-managed Bedrock/Vertex/Google paths bypass that instrumentation and are outside this qualification.

This establishes bounded synthetic **single-process transport** behavior, not complete S0 readiness. Input tokens have a post-response stop threshold; no pre-request tokenizer/context-fit guarantee is claimed. A response can already have consumed unknown usage when it fails, and local output limits cannot establish provider billing behavior. The guard does not implement cross-process whole-run accounting: a new process gets new counters. No 108/132-answer batch can treat those as a global budget. Natural-task grading acceptance for the already selected DeepSeek candidate, exact model/profile/context fit, actual client/tool/source/processor permissions and an approved whole-run allowance remain in #124. No new issue, smaller pilot, packet rewrite, installation or inference grant follows.

Go tests cover failure/replay blocking, auxiliary usage, request/model/route/output limits, invalid/truncated usage, concurrency, cancellation and journal failures. The race-enabled guard tests, serial repository suite and `go vet ./...` pass. An earlier concurrent suite hit the existing capture startup timing test while the private client build was running; the serial rerun passed without capture changes. Baseline documents and frozen packets stay unchanged. Overall focused S0 effort and Codex conversation usage remain unmeasured; this fixture is not a stage pass.
