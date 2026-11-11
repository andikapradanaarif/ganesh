# Go developer tools

Active developer tooling uses Go. `cmd/ganesh-dev` is separate from the general CLI and the experimental capture-only binary; it is never installed as a hook.

Run the native Crush fixtures from this repository:

```sh
go run ./cmd/ganesh-dev crush-test --source /path/to/crush/source --build-profile stripped
```

The runner accepts only the two inspected immutable revisions documented in [the adapter contract](s0-crush-hooks.md). It requires clean tracked source, no untracked Go files and absent overlay targets. It uses Go overlays without changing the checkout, disables module downloads and automatic toolchain selection, and bounds each build/test stage to 60 seconds with a 20-second Go test runtime cap. Ambient `GOFLAGS` or `GODEBUG` are rejected; `GOWORK=off` prevents an ambient workspace changing the build. Profiles are exactly `development` (default), `stripped` and experimental `collector`.

The fixture output is synthetic. Streamed build/test stages forward stdout and stderr separately; runner progress goes to stdout and wrapper errors go to stderr. Metadata-only git commands privately capture bounded stdout and discard stderr. The runner preserves failures and performs no automatic retry or fallback. Cold-launch rejection and explicit duplicate recovery must still be reported separately. Passing does not qualify an installed client, complete capture profile or narrative eligibility.

Read the completed studies without launching any collectors:

```sh
go run ./cmd/ganesh-dev study-summary --study 34 --dir .local/capture-startup-34-20261102-a
go run ./cmd/ganesh-dev study-summary --study 36 --dir .local/standalone-capture-36-20261102
```

These directories are private local evidence, excluded from git. Another checkout needs its own permitted copy of the original records; the commands do not create data. Only studies 34 and 36 are supported. The report wraps `metrics` and `decision` with a schema version and study ID. Successful JSON output can describe an incomplete or invalidly bound study; check `decision.status`: `complete_local_comparison`, `incomplete` or `invalid_study_binding`. The latter two emit `metrics: null`, and all profile/signal fields remain false. A parsing, permission or I/O failure returns a static error and exit 1. JSON object key order is unspecified; consumers and parity checks compare decoded objects.

Reads require a 0700 directory and regular 0600 `plan.json`/`attempts.jsonl` files, rejecting final-component symlinks and symlinked files. Ancestor paths and the same-user host remain trusted. Bounds are 1 MiB per file, 64 KiB per attempt line, 1,000 parsed rows and 64 JSON nesting levels. Duplicate keys, unknown fields, absent required scoring values and malformed types fail. No input prose, source paths or child errors are copied into summary errors. Counts and numbers remain recorded evidence, not independently verified measurements. Do not modify a dataset while summarizing it; this is a reader for closed studies, without a transaction across files or cryptographic attestation.

The manifest must match the completed study's source revision, seed, cap and deadline; study 36 also requires its explicit study ID. Matching declared constants is a consistency check, not authentication. The sample must contain every expected stage/arm/category/block exactly once and valid positive parent timings. These input checks supplement the original summaries without changing their numeric rules. Percentiles use nearest rank and include failed/censored attempts. Missing OS clocks stay visible. Study 34 preserves its candidate-only control veto and first-execution tail gate; study 36 preserves its symmetric controls, first/warm tail gates and separate headroom probe. Neither rule enables a capture profile. The reader does not revalidate source artifacts, producer digests or the collector's reported outcomes.

Audit the private local-only Crush digest reduction for [issue #50](https://github.com/andikapradanaarif/ganesh/issues/50):

```sh
go run ./cmd/ganesh-dev corpus-audit --dir .local/s0/issue-50-20261108 --raw crush-digests-valo.raw.json,crush-digests-kiosk.raw.json
```

The command reads the listed raw snapshot basenames plus `crush-digest-manifest.jsonl` and `crush-digest-candidates.jsonl` in the same private 0700 directory. Every file must be regular and 0600; final-component symlinks are rejected. It verifies exact raw-field hashes, a one-to-one raw/manifest/candidate join, the declared reduction, redaction counts and local-only flags. It emits aggregate JSON without source text and returns a static error on invalid input. Each file is capped at 2 MiB; rows have separate 64 KiB text and 128 KiB JSON-line bounds. The file cap is intentionally tighter than the theoretical sum of row limits for this small pilot and must be changed deliberately if a later corpus grows. This audit checks reproducibility of the declared patterns only. It does not establish safe model disclosure, first-party origin, semantic quality, full-context fit or permission to run models. See the [preflight](s0-corpus-calibration-preflight.md) for the observed result and remaining gaps.

Recheck the private Devin source links for [issue #53](https://github.com/andikapradanaarif/ganesh/issues/53):

```sh
go run ./cmd/ganesh-dev devin-audit --dir .local/s0/issue-50-20261108 --transcript-root /path/to/Devin/transcripts --grant-map-dir .local/s0/issue-53-20261108
```

The evidence directory must contain private 0600 `devin-sessions.json` and `devin-manifest.jsonl`; the grant-map directory must contain private 0600 `project-roots.json`, derived from the recorded owner grants and excluding Nexus. Both directories must be 0700. The explicit transcript root must be a real directory, working directories must be absolute clean paths, and the reader accepts only regular, stable files with names derived from validated session IDs. Input limits are 4 MiB per evidence file, 1 MiB for the grant map, 1,000 rows, 128 KiB per manifest line, 32 MiB per transcript and 128 MiB across all transcripts. The audit emits only aggregate counts, file bytes, drift categories and digests. Changed or newly available files are reported as drift; unsafe file types, path escapes and malformed inputs fail with a static error. It does not output transcript text or authorize model use. See the [source-link audit](s0-devin-source-audit.md) for the observed result and limitations.

Each transcript read stops at its checked size plus one sentinel byte. A file that grows or shrinks during the read fails the audit; the sentinel cannot start another accepted source read or appear in a partial report.

Prepare local-only Devin handoff excerpt candidates for [issue #58](https://github.com/andikapradanaarif/ganesh/issues/58):

```sh
go run ./cmd/ganesh-dev devin-candidates --dir .local/s0/issue-50-20261108 --transcript-root /path/to/Devin/transcripts --grant-map-dir .local/s0/issue-53-20261108 --manifest-sha256 "$S0_DEVIN_MANIFEST_SHA256" --output-dir .local/s0/issue-58-20261108
```

Set the manifest digest from the saved private source record. The command requires an unchanged source audit, rechecks the exact bytes selected for parsing, and extracts only the last nonempty recorded agent message without tool calls. It omits user/system messages, reasoning and tool fields. Existing source limits apply independently to the audit pass and extraction pass. Parsing accepts at most 20,000 steps per transcript; selected messages are limited to 16 KiB before pattern reduction and 64 KiB after it. Oversized or unsupported messages are excluded without fallback or truncation. See the [candidate report](s0-devin-excerpt-candidates.md) for the exact reduction policy and real counts.

The output directory must already be 0700 and excluded from git. `devin-excerpts.json` is created exclusively as 0600, capped at 2 MiB, and synchronized before success. Existing output is never overwritten. A failed write can leave an incomplete private file; no aggregate success is returned for it. Stdout contains only counts, byte totals, redaction totals and the artifact digest; errors are static. Candidates retain source links, unknown origin/closure and local-only status. This command grants no source-content disclosure or model-run permission.

Recheck the two selected Kiosk Codex rollouts for [issue #55](https://github.com/andikapradanaarif/ganesh/issues/55):

```sh
go run ./cmd/ganesh-dev codex-audit --manifest-dir .local/s0/issue-50-20261108 --sessions-root /path/to/Codex/sessions --grant-map-dir .local/s0/issue-53-20261108
```

The two-row `codex-kiosk-manifest.txt` and `project-roots.json` must be regular 0600 files in 0700 directories. The reader checks each manifest path under the explicit sessions root, rejects symlinked components, hashes at most 32 MiB per rollout and 64 MiB total, and requires a first-line `session_meta` record within 64 KiB whose `cwd` equals the granted Kiosk root. Changed or missing files are reported as drift; malformed metadata, unsafe paths and unsupported files fail with a static error. It emits aggregate counts, file bytes and digests without rollout text or private paths. It does not authorize model use; see the [Codex source-link audit](s0-codex-source-audit.md) for results and limits.

The first metadata line must end with a newline. Each read stops at the checked file size plus one growth-detection byte, including buffered prefetch; growth or shrinkage fails without a partial result.

Compare saved owner and grader calibration judgments without model calls:

```sh
go run ./cmd/ganesh-dev calibration-summary --dir .local/s0/issue-60-20261108/grader-fixtures --frame-sha256 "$S0_CALIBRATION_FRAME_SHA256"
```

The digest pins the preserved `owner-labels.blank.json` frame. Optional `owner-labels.json` and `grader-labels.json` use the same packet format; missing files and null judgments remain incomplete. The reader requires exact joins for six tasks / 18 cases, private 0700/0600 storage and at most 128 KiB per file. It reports aggregate comparison counts and input hashes, never notes, identities or answer text. It does not run a model or accept calibration. See [the comparison workflow](s0-calibration-comparison.md) for status/metadata fields, denominators, validation and provenance limits.

Check a proposed structured single-case grading response offline:

```sh
go run ./cmd/ganesh-dev grader-evidence-check --dir .local/s0/structured-case --frame-sha256 "$S0_ANSWER_FRAME_SHA256" --response-sha256 "$S0_GRADER_RESPONSE_SHA256"
```

The caller pins fixed `answer-frame.json` and `grader-evidence.json` files, each at most 64 KiB, in private 0700/0600 storage. The new versioned format requires four final criterion dispositions, exact UTF-8 byte spans for met criteria and consistent derived completion. It emits aggregate metadata only. A real but irrelevant quotation can pass the mechanical check; semantic relevance, calibration acceptance and model dispatch remain false. The [evidence contract](s0-grader-evidence.md) specifies strict fields, bounds, null/contradiction behavior and the remaining relevance audit. No earlier packet is converted or model called.

Reconcile saved native OpenCode Go usage without another model call:

```sh
go run ./cmd/ganesh-dev go-usage-check --dir .local/s0/saved-usage --usage-sha256 "$S0_GO_USAGE_SHA256"
```

The caller pins a private regular 0600 `go-usage.json` in a 0700 directory, capped at 32 KiB. The pure parser checks zero-cost Go billing, explicit zero Go reasoning, optional standard metadata and total/native-versus-cached input consistency with overflow-safe arithmetic. Exact numeric cost checking rejects tiny nonzero charges; allowance text and absent optional counters are preserved. Output contains accounting metadata only and keeps model identity, grading, calibration acceptance and dispatch unverified/false. The [usage contract](s0-go-usage.md) documents the fixed 2,048-output-token profile, ignored provider extensions, private trust boundary and 23-object saved-record audit. No stream/model/answer validation or live runner is added.

Run the disposable issue-state verifier against its fixed synthetic expectations:

```sh
go run ./cmd/ganesh-dev issue-state-probe
```

This command takes no external inputs and performs no connector or model calls. It compares 24 digest-pinned synthetic cases with expectations committed before candidate implementation, reporting aggregate matches/mismatches and keeping publication/stage flags false. A mismatch report exits 1. See [the probe record](s0-issue-state-probe.md) for the exact predicate, simulated clock, oracle separation and substantial untested coverage. This is preparation for one P03-D case, not completion of S0b-1 or a production verifier.

Probe exact protected-branch head support without deployment inference:

```sh
go run ./cmd/ganesh-dev repository-head-probe
```

The fixed embedded set contains 43 digest-pinned synthetic cases frozen before candidate implementation. The separate scorer reports aggregate matches/mismatches, with publication/stage flags false; a valid mismatch report exits 1. No external input, connector or model call is accepted. See [the head probe record](s0-repository-head-probe.md) for branch-protection, source/scope, commit/revision and freshness checks, ignored prose, oracle separation and incomplete P03-D coverage. A valid commit lookup or tag is insufficient for a protected-branch head claim, and neither establishes deployment.

Probe exact CI results with explicit current-head dependencies:

```sh
go run ./cmd/ganesh-dev ci-result-probe
```

The no-input command compares 75 digest-pinned synthetic cases with manually specified expectations frozen before implementation. It separates exact commit/run results from current-head qualification, using the existing protected-branch verifier only for the latter. The aggregate reports matches/mismatches with publication/stage flags false; a valid mismatch report exits 1. See [the CI probe record](s0-ci-result-probe.md) for workflow/run/attempt/revision binding, status/time constraints, old-run examples, ignored prose and incomplete integration/atomicity coverage. A workflow success does not mean all tests passed or deployment is safe.

Probe decision authority without promoting PR prose:

```sh
go run ./cmd/ganesh-dev decision-authority-probe
```

The no-input command compares 67 digest-pinned synthetic cases with manually specified expectations frozen before implementation. Positive controls use a fake designated structured registry; authenticated/owner-labeled PR prose cannot grant decision authority. The separate scorer includes adoption state in its comparison and reports only aggregate matches/mismatches with publication/stage flags false; a valid mismatch report exits 1. See [the authority probe record](s0-decision-authority-probe.md) for fake designation/authentication/writer controls, exact enum/state/revision checks, seven-day rechecks and missing real provenance/integration coverage. No actual registry is enrolled or source/model call made.

Trace agent echoes without counting reports as independent evidence:

```sh
go run ./cmd/ganesh-dev echo-origin-probe
```

The no-input command compares 50 digest-pinned synthetic graphs with expected outcomes frozen before implementation. It traces source/object identities and revisions through tool/agent/copy nodes, validates the entire bounded DAG and preserves root-class counts. A closed lineage remains reported-only; invalid snapshots return incomplete lineage without partial counts. The aggregate retains independence/publication/stage flags false, and a valid mismatch report exits 1. See [the echo-origin record](s0-echo-origin-probe.md) for graph bounds, alias/revision semantics, oracle separation and missing real provenance/consumer coverage. No model/service call or real source enrollment occurs.

Build and run the active Go checks:

```sh
go build ./...
go vet ./...
go test -p=1 ./...
go test -race -p=1 ./...
CGO_ENABLED=0 go test -p=1 ./...
```

The Python sources under `integration/capture` and `integration/crush/test_runner.py` stay at their original paths as historical records. Historical commands and test counts in the completed study documents describe those runs; use this page for current workflows. The closed performance producers remain expired. Do not edit them, extend their attempt caps or run a new study through them. Current Go commands and tests never invoke Python. The archive retains older scripts' environment/runtime requirements for historical reproduction; it adds no Python requirement to current build, test, fixture or summary commands.

## Migration evidence, 2026-11-02

On macOS amd64 with Go 1.27.3 and CGO enabled, the Go reports matched the retained historical evaluators exactly after JSON decoding: every metric and decision for study 34's 114 attempts and study 36's 84 attempts. A one-time read-only reference check imported the Python evaluators with bytecode generation disabled; it generated no new timing attempts. Study 34 still recommends the stripped build under its original rule, while study 36's local-signal and headroom gates remain false. All capture profiles remain disabled and [issue #34](https://github.com/andikapradanaarif/ganesh/issues/34) remains open.

The Go native runner with profile `stripped` passed against `aed2bae006adc2f17c9ea2c693e2ca9cb4eca84f` and `3378f47a8398c79a10e35cc61c994160cc0ae3ea`. Each main fixture delivered 41 synthetic notifications and retained 15 records, with 13 intended non-blocking rejections and `decision=none` throughout. No extra callback rejection or automatic retry occurred in these two runs. The baseline separately reproduced both cancelled/expired-context ending suppressions; the repaired pin passed both ending-delivery cases. The compile/test stages took 21.989 and 31.882 seconds respectively, within the 60-second cap; these are validation durations, not collector latency measurements.

The approved roadmap, execution sheet, historical producers, fixtures and recorded datasets remain unchanged. No client configuration, installed binary, capture eligibility or production policy changes. Work is tracked in [issue #38](https://github.com/andikapradanaarif/ganesh/issues/38).

Validation passed: Go build, vet, normal/race/CGO-disabled suites, document target/diff checks and both native runs above. Unit fixtures cover exact commands/environment/timeouts, source-gate failures, incomplete and contaminated samples, distinct historical vetoes, failed tails, missing clocks, private-file boundaries, duplicate/case-aliased keys and content-free errors. The capture-only dependency list still excludes `internal/devtools`, `internal/s0` and `os/exec`.

## PR #39 review follow-up

| Finding | Disposition |
| --- | --- |
| Streamed stdout/stderr were merged | Restore separate destinations through the CLI and subprocess runner. Real child-process tests cover success/failure routing and private metadata capture; runner-plan tests bind each stage to both destinations. |
| Partial cohorts were withheld | Retain `metrics: null` until binding and the full sample pass. Partial or contaminated labels cannot appear as validated cohort output; existing incomplete/contamination fixtures cover this choice. |
| New invalid-binding status | Keep the explicit `invalid_study_binding` result and document the three status values above. It distinguishes an incorrect manifest from a correctly bound but incomplete sample. |
| JSON object key ordering differs | Keep ordinary JSON objects and compare decoded values. Serialized key order is outside the report contract. |
| Reader trusts retained records | Keep the closed-study, same-user trust contract explicit. Matching manifest constants establishes consistency; artifact authentication and outcome attestation remain outside this reader's scope. |

The follow-up passed build, vet, normal/race/CGO-disabled suites and both native pins with stdout/stderr captured to separate files. Each native run again retained 15 records from 41 notifications with 13 intended non-blocking rejections and no additional rejection/retry; compile/test stages took 33.960 seconds (baseline) and 15.538 seconds (repaired). Both successful runs had empty stderr files. The real subprocess tests separately exercise nonempty stderr and failed-child output routing. Reports for all 198 retained attempts remained byte-identical to the approved outputs; no timing-study samples were added.
