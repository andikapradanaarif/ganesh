# Offline calibration comparison

[Issue #63](https://github.com/andikapradanaarif/ganesh/issues/63) adds an offline comparison step after the [synthetic grader fixtures](s0-grader-calibration-fixtures.md) prepared under [#60](https://github.com/andikapradanaarif/ganesh/issues/60). This is subscription-only development with **zero additional paid API spending authorized**. The historical [run-input proposal](s0-run-input-proposal.md) remains a proposal. No API evaluation runner or provider calls are added.

The command reads three fixed filenames in one private directory:

- `owner-labels.blank.json`: the immutable original neutral-ID frame, containing six tasks with three cases each. Supply its SHA-256 from the preserved fixture manifest, after checking that manifest against its saved digest. The command does not authenticate the manifest itself.
- `owner-labels.json`: the owner's separately saved judgments, copied from the blank packet.
- `grader-labels.json`: separately saved grader judgments in the same packet format. Importing these declarations does not call or authenticate a model.

Keep each file regular and 0600, the directory 0700 and all content excluded from git. Final-component symlinks are rejected. Ancestor directories and the same-user host remain trusted. Reads are capped at 128 KiB per file, with the existing 64-level JSON nesting bound; notes are capped at 4 KiB per case. Use closed snapshots: there is no cross-file transaction or protection against deliberate same-user rewriting.

```sh
go run ./cmd/ganesh-dev calibration-summary --dir .local/s0/issue-60-20261108/grader-fixtures --frame-sha256 "$S0_CALIBRATION_FRAME_SHA256"
```

Preserve the original blank and fixture manifest. In each working copy, keep every case/task ID and all six case fields. Enter `true`, `false` or explicit `null` for `completion`, `introduced_critical_error` and `unsupported_claim`; null means unknown. Notes stay private. Set packet metadata as follows:

| Judgments supplied | `status` | `reviewer_identity` / `labelled_at_utc` |
| --- | --- | --- |
| None | `unlabelled` | Both null |
| Some, with at least one unknown | `partial` | Nonempty identity and RFC 3339 UTC timestamp ending in `Z` |
| All 54 values across 18 cases | `labelled` | Nonempty identity and RFC 3339 UTC timestamp ending in `Z` |

Case order is irrelevant. Added, dropped, duplicate or task-mismatched rows fail; duplicate, case-aliased, unknown or missing JSON fields also fail. A completed answer with an introduced critical error is invalid. The reader checks structural consistency, not whether the judgments follow the rubric. Do not substitute assistant-proposed labels for owner decisions.

Absent owner/grader files are an unstarted comparison. Existing malformed or unsafe files fail with a static error, exit 1 and no JSON report. A valid partial report exits successfully with `status: incomplete`; it is not an accepted calibration. Each field reports owner-known, grader-known, compared, agreed, disagreed, false-negative and false-positive counts. A field enters the comparison only when both sides supplied a boolean. False negative means owner true / grader false; false positive means owner false / grader true. For the critical-error field these correspond to missed owner-flagged errors and false alarms. These are relative to owner judgments, not independently established truth.

Fully compared cases require all three judgments on both sides. Only all 18 fully compared cases produce `status: compared_declarations`. No percentage is computed from a partial denominator, and `calibration_accepted` is always false, including for perfect agreement. Output contains aggregate counts and exact input digests, without case IDs, notes, reviewer identities, timestamps or paths. Keep even this metadata private unless intentionally publishing aggregates.

The frame pins case membership, not answer content, rubric fidelity or reviewer provenance. Saved label files contain no per-answer hashes. The operator must preserve their relationship to the original answers and record the corresponding fixture/run metadata separately; the reader does not verify model identity, different-family grading, independence, provider disclosure or budget compliance. It never reads answers, source excerpts or proposed labels. Actual independent labels, rubric/source checks and the execution-sheet review remain necessary for acceptance.

## Local evidence

The preserved #62 fixture manifest and blank-packet hashes match their saved records. Running the new command on that directory reports six tasks / 18 cases, absent owner and grader files, zero compared judgments, `incomplete` and `calibration_accepted: false`. The blank packet was not changed and no judgments were manufactured.

Synthetic tests check explicit confusion counts, shuffled joins, partial coverage, malformed/unsafe inputs and content-free CLI output, including failed output writes. These tests establish the implemented comparison behavior only. The fixtures remain synthetic variants across six tasks; no grader agreement, held-out outcome, capture qualification or S0 stage pass is established.
