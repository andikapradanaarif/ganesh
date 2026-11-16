# S0 owner preflight and execution-input proposal

The next owner action in [#124](https://github.com/andikapradanaarif/ganesh/issues/124) is a **local metadata preflight**, using [the Go command](../cmd/ganesh-s0-preflight/main.go). It reads Git HEAD and hashes 28 already selected files in the recorded Kiosk/Valo-backend roots: six native instruction files and 22 static-source/Makefile paths. Reports contain file hashes, sizes, source-match results and executable presence; no source bodies, account settings or credentials are returned. This assistant has tested synthetic repositories and has not run this inspection on the real projects.

The private handoff supplies an already built binary, package path, manifest hash and exact command. Review its `inspection-plan.json` for the two roots and selected paths, then run the `inspect` command. The report directory must not exist; an earlier report is never overwritten. A match is evidence for the selected files, not task/oracle adoption, complete existing-notes discovery, tool grants or runtime isolation. This is a read window, not an atomic or locked whole-repository snapshot. Changed/unavailable files remain visible and require assessment.

There is **no inference, install or approval command**. Neither `prepare` nor `inspect` creates an experiment ledger or accepts a grader. No model call or current project-source inspection is needed from this agent to prepare the handoff. The unchanged [baseline](s0-execution-sheet.md), original [bound packet](s0-source-backed-task-revision.md) and [shared-accounting limits](s0-run-accounting.md) remain separate authorities/evidence.

The owner reported the original generic error after copying an `inspect` command without its required `-out` argument. That failure was reproduced before any project access. The v2 binary reports fixed stage codes such as `arguments_output`, `manifest_hash` and `output_already_exists`, without leaking raw error strings or source text. It reserves the output directory before source reads, so an existing/unwritable destination cannot trigger a wasted rescan. `check-package -package PATH -sha256 HASH` verifies the same package without opening project roots or creating a report.

## What is prepared

The preparer verifies the original packet's manifest and every pinned file, the #139 post-review report, and the reserved six calibration task/rubric/oracle hashes. It derives 276 prospective process slots: 132 possible S0 answers and 132 matching grades, plus six proposed calibration answers and six grades. The base/conditional task IDs, arm positions, answer-file allowlists and grading references are preserved. Calibration is ordered first; each phase keeps its frozen sequence and answer/grade pairs. Conditional work still requires the original D/F trigger.

The calibration proposal uses fresh model-written, source-only answers to the existing six reserved tasks. Earlier constructed answer variants, owner labels/exposure and failures remain unchanged. Reused task/source contexts and shared-author references do not become an independent holdout. Source-frame assembly, reference adjudication, acceptance/stop rules and required source-based owner checks remain unresolved before calls. Agreement with assistant references alone cannot accept the grader. No leading label-confirmation flow is resumed.

`authority.json` and `readiness.json` retain those unresolved fields. `process-slots.json` is a proposal, not a dispatcher or a grant of 276 sessions. Current call authority remains **zero**. The answering ID `gpt-6-luna` is still proposed; DeepSeek/Go remains the already selected grading candidate under #83/#84. Actual profile/context/account/source/processor grants still need binding. A/B remain honestly empty; no typed memory is invented and normal own-project tools are not replaced by cached excerpts.

## Proposed limits requiring adoption

| Item | Prospective value |
| --- | --- |
| Main answer request / total session output | 4,096 / 8,192 tokens, including reasoning and auxiliary output |
| Main answer requests / elapsed cap | At most six provider requests / 180 s per session |
| Calibration answer | One direct source-only request, 4,096 output / 180 s; no agent auxiliary calls |
| Grading | One non-streaming direct request, 2,048 output / 60 s; no agent auxiliary calls |
| Maximum requests | `132 × 6 + 132 + 6 + 6 = 936` |
| Maximum combined output | `132 × 8,192 + 138 × 2,048 + 6 × 4,096 = 1,388,544` tokens |
| Combined input | 2,000,000-token **post-response stop**, not a hard pre-dispatch cap |
| Pacing and proposed driver concurrency | At least 600 s between answer starts; one process at a time |
| Aggregate active model caps | 33,120 s = 9h12, including the proposed calibration phase |
| Nominal paired schedule at full session caps | 82,080 s = 22h48, before additional review/quota/other pauses |
| Proposed absolute horizon | 48 hours from the eventual approved start |
| Additional paid spend / retries / fallback | $0 / 0 / 0 |

These amend unapproved proposals, not existing grants. Separating requested output from total session output leaves room for auxiliary requests; a 40-token auxiliary reply and a full 4,096-token main reply fit the proposed 8,192 total in the executable regression. Grading/calibration use direct calls so an agent's title request cannot consume their single-request allowance. The old 31,680-second figure excluded calibration and represented active model caps, not wall time with pacing. The new input-stop semantics and horizon require explicit adoption; quota, current Extra Usage state and whole-request fit remain unknown. Missing context-fit evidence is not repaired by calling bytes tokens. Historical focused-effort overruns/unknowns are not reset.

## Reproduce the local-only operations

From a checkout containing this change, with cached Go dependencies:

```sh
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go run ./cmd/ganesh-s0-preflight prepare \
  -packet /absolute/path/to/bound/packet \
  -calibration /absolute/path/to/calibration-manifest.json \
  -roots-map /absolute/path/to/project-roots.json \
  -guard-report /absolute/path/to/run-accounting-139-review/report.json \
  -out /absolute/path/to/new-owner-package
```

That operation uses cached artifacts and does not open the project roots. The owner then runs:

```sh
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go run ./cmd/ganesh-s0-preflight inspect \
  -package /absolute/path/to/owner-package \
  -sha256 PACKAGE_MANIFEST_SHA256 \
  -out /absolute/path/to/new-inspection-report
```

The retained private package manifest is `51e6dde7802e3557f086872a824bb8e9cd1225584aa885bbb5c9d3e2d7d65161`; the v2 owner binary is `6b2fd31aeb45597282a5c42be915a61f045b7f0f457eae3035d1e9dda2aa7a3c`. The original binary and failed-attempt evidence remain unchanged. Project-source/data artifacts stay private. Tests cover synthetic Git repositories, changed files, tampered hashes, traversal/symlink/empty-scope refusal, private metadata-only output, unsupported live commands, output headroom and budget arithmetic. The follow-up also tests missing-output diagnostics, source-free package verification, and zero source reads on output failures. Two offline preparations with identical inputs reproduce the same manifest bytes. Race tests, the serial repository suite and vet pass. A local match never makes the proposed S0 run ready by itself. S0 remains **0/108**, with conditional 24 retained.
