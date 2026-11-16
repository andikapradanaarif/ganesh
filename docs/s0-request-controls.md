# S0 answering request controls under #124

The [#137 failure](s0-execution-profile.md#installed-answering-client-check) now has an executable fix in a **private experimental Crush build**. The [Go guard](../internal/devtools/requestguard/guard.go) covers native Responses requests through the shared instrumented transport, including auxiliary calls. The [pinned-build fixture and command](../integration/crush/requestprofile/README.md) reproduce the offline checks without changing the installed client or external source checkout. No real provider calls or original S0 answers ran.

The guard serializes requests, caps dispatch count and requested/total output, forbids identical payload replay, and stops on failed, interrupted or unaccounted replies. It syncs a private metadata-only start before forwarding and a finish before returning success. Unknown usage remains null; known totals do not imply that unknown requests cost zero. Plain HTTP/1 transports use fresh connections without environment proxies or replay hints. Custom OAuth/debug transports are rejected. Bounded responses are buffered until native JSON/SSE completion and usage have been checked; incremental SDK delivery is therefore changed.

The mandatory build adapter terminates the isolated session with exit **78** after a guard failure. This prevents the SDK's otherwise continuing local retries/backoff and fallback. Missing/invalid guard configuration also denies dispatch. This behavior is for one experimental session per process; partial stdout after a failure is not an accepted answer. The installed ordinary client still has its original behavior.

## Measured offline result, 2026-11-16

All five actual-client fixtures pass using the same source pin as #137, `86897c56a304bac133650a6055ab1d091e6b8f83`, built with Go 1.27.4. Synthetic prompts, dummy credentials, isolated state and an OS loopback-only network sandbox were mandatory.

| Fixture | Requests reaching the stub | Usage accounting | Process result |
| --- | ---: | --- | --- |
| HTTP 503 | 1 | 1 unknown; no zero estimate | Exit 78, 1.630 s |
| Successful SSE | 2 | Both known: stub-declared totals 22 input / 6 output | Exit 0, answer returned, 0.413 s |
| Missing usage | 1 | 1 unknown; continuation blocked | Exit 78, 0.324 s |
| One-request cap | 1 | First known; next attempt recorded as blocked | Exit 78, 0.351 s |
| Missing guard configuration | 0 | No request ledger created | Exit 78, 0.296 s |

Peak observed in-flight requests was one. No fixture reached its 35-second launcher deadline. The successful case accounts for both observed requests (requested output 40 and 256) through the shared guard; roles are not inferred from payload sizes alone. The declared 11/3 per-reply usage is synthetic accounting input, not measured tokenizer output or provider billing.

Private report SHA-256: `8e672f7e4d1b806f2fff5b7fd05ea5ba70492805fa9eba8f5d725157ebc12da8`.

Private candidate binary SHA-256: `5097047286d45baa2e3a9c3d41ba95c2363f9d5559726e2a50aeea152543d46a`.

The report joins guard/adapter/original-and-patched-client-source hashes, binary identity, all observed request hashes and private journals. Earlier development fixtures exposed a cancellation-test cleanup defect, Git archive PAX handling and SDK backoff after transport rejection. Those were corrected; earlier private outputs remain separate. The final fixture includes the fail-fast adapter, not only the earlier traffic-blocking version.

## Remaining scope

This establishes bounded synthetic **single-process transport** behavior, not complete S0 readiness. Input tokens have a post-response stop threshold; no pre-request tokenizer/context-fit guarantee is claimed. A response can already have consumed unknown usage when it fails, and local output limits cannot establish provider billing behavior. The guard does not implement cross-process whole-run accounting: a new process gets new counters. No 108/132-answer batch can treat those as a global budget. Natural-task grading acceptance for the already selected DeepSeek candidate, exact model/profile/context fit, actual client/tool/source/processor permissions and an approved whole-run allowance remain in #124. No new issue, smaller pilot, packet rewrite, installation or inference grant follows.

Go tests cover failure/replay blocking, auxiliary usage, request/model/route/output limits, invalid/truncated usage, concurrency, cancellation and journal failures. The race-enabled guard tests, serial repository suite and `go vet ./...` pass. An earlier concurrent suite hit the existing capture startup timing test while the private client build was running; the serial rerun passed without capture changes. Baseline documents and frozen packets stay unchanged. Overall focused S0 effort and Codex conversation usage remain unmeasured; this fixture is not a stage pass.
