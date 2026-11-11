# Offline structured grader evidence

[Issue #85](https://github.com/andikapradanaarif/ganesh/issues/85) adds a mechanical evidence check before proposing another calibration run under [#60](https://github.com/andikapradanaarif/ganesh/issues/60). It reads a saved single-case answer/requirements frame and structured grading response. No model, source connector, credential lookup, retry or provider disclosure occurs.

The prior private repair run showed that a grader can quote real answer text while crediting required substance that is absent. This command validates quotation membership and consistent declarations. It cannot establish whether a quotation satisfies a criterion. The synthetic tests deliberately include a real but irrelevant quotation that passes mechanically while `semantic_relevance_verified`, `calibration_accepted` and `model_dispatch_authorized` remain false.

This format is proposed for a future calibration version. It does not convert earlier free-form notes into evidence, rewrite saved responses, adopt reference labels or authorize another run. The [original comparison](s0-calibration-comparison.md), required owner checks and [S0 execution sheet](s0-execution-sheet.md) remain unchanged. Grader quality and stage acceptance remain unresolved.

[Issue #89](https://github.com/andikapradanaarif/ganesh/issues/89) adds response version 2, which omits the model's redundant completion field and derives it locally. A model can still misjudge criterion coverage. Unresolved coverage remains unknown; this format change does not repair semantic grading or accept the stopped diagnostic. Response version 1 and its consistency checks remain supported unchanged.

[Issue #94](https://github.com/andikapradanaarif/ganesh/issues/94) adds offline response version 3 following the [whole-answer proposal](s0-whole-answer-adjudication.md). It separates declared positive coverage from declared defeating content and derives both criterion status and completion locally. It cannot establish that those declarations are correct. Versions 1/2 and their report format remain supported unchanged; no saved response is converted and no model run is authorized.

## Invocation and storage

```sh
go run ./cmd/ganesh-dev grader-evidence-check \
  --dir .local/s0/structured-case \
  --frame-sha256 "$S0_ANSWER_FRAME_SHA256" \
  --response-sha256 "$S0_GRADER_RESPONSE_SHA256"
```

The two fixed filenames are `answer-frame.json` and `grader-evidence.json`. Both must be regular 0600 files in a 0700 directory, outside version control. Final-component directory/file symlinks are rejected; ancestor directories and the same-user host remain trusted. The command does not run git to establish exclusion: the operator must preserve private storage and closed snapshots. There is no transaction across the two reads or protection against deliberate same-user replacement of the files and supplied digests.

Take the frame digest from its preserved pre-dispatch record and the response digest from the saved run record. Both must be 64 lowercase hexadecimal characters. They pin exact file bytes, including whitespace, and do not authenticate the author, model, rubric or records. Do not repin modified artifacts to disguise a changed run. A changed or malformed file returns the static error `private bounded grader evidence required`, exit 1 and no JSON report.

Each file is capped at 64 KiB. The decoded answer is nonempty UTF-8, at most 32 KiB; each requirement and criterion explanation is at most 1,024 UTF-8 bytes; notes at most 4,096 bytes. There are exactly four criteria. Versions 1/2 allow at most four spans per criterion; version 3 allows at most four in each of its two lanes. Each span quotes at most 2,048 bytes. The existing 64-level JSON nesting bound also applies. The command checks bytes and declared structure, not scrubbing adequacy or permission to disclose their contents.

## Frame schema, version 1

Every field is required and non-null. Unknown, case-aliased and duplicate keys (including escaped spellings) fail. Task and case identifiers are lowercase ASCII alphanumeric groups separated by hyphens, at most 128 bytes each. Criterion order is irrelevant; exactly one each of `C1`, `C2`, `C3`, `C4` is required. This fixed shape describes the current diagnostic profile, not a universal rubric.

| Field | Value |
|---|---|
| `schema_version` | Integer token `1` |
| `task_id`, `case_id` | Bound task/case identifiers |
| `answer` | Exact candidate answer after JSON decoding; no trimming or Unicode normalization |
| `criteria` | Four objects, each containing exactly `id` and nonblank `requirement` |

The response's `answer_frame_sha256` binds both the answer and criterion requirements to this exact frame. It does not establish the frame's relationship to an original task, reference oracle or approved run; the operator must retain that provenance separately.

## Response schema, version 1

All nine top-level fields are required. Only the three judgment fields accept explicit null. Strings, integers and booleans cannot substitute for each other; `1.0` is not the version integer token, and floating-point offsets fail.

| Field | Value |
|---|---|
| `schema_version` | Integer token `1` |
| `task_id`, `case_id` | Exactly the frame's identifiers |
| `answer_frame_sha256` | SHA-256 of the exact frame file bytes |
| `completion` | Boolean or null; must agree with the derivation below |
| `introduced_critical_error` | Boolean or null |
| `unsupported_claim` | Boolean or null, retained separately from critical error/completion |
| `criteria` | Exactly one disposition for each frame criterion ID |
| `notes` | Bounded non-null string; may be empty |

## Response schema, version 2

Version 2 has exactly the same fields as version 1 except `schema_version` is the integer `2` and `completion` is forbidden, including explicit null. All eight fields must exist; only `introduced_critical_error` and `unsupported_claim` are nullable. The answer/requirements frame remains at version 1. Criterion/span bounds, exact fields and private-file rules are identical for both response versions.

Completion is calculated by the application using the same table below. The model supplies final criterion dispositions and the critical/unsupported judgments; it never supplies completion in this version. Unknown versions, mixed layouts and attempts to supply `derived_completion` in a response are rejected. A live protocol that pins one response version must also check the reported `response_schema_version` against that protocol; the offline reader supports both versions and does not establish which a model was authorized to return.

Do not convert an old response by deleting completion, changing its version or repinning it to claim recovery. Old version-1 inconsistencies remain invalid. New version-2 examples and model results need their own frozen provenance and protocol.

## Response schema, version 3

The frame remains version 1. All eight top-level fields from version 2 remain required, but `schema_version` is the integer `3` and criterion objects use the fields below. The two top-level critical/unsupported judgments remain explicitly required and nullable. Completion, final criterion status and derived report fields are forbidden response fields, including null. Existing caller-pin, identity, file, UTF-8, requirement and notes bounds apply unchanged.

Each criterion has exactly seven non-null fields:

| Field | Contract |
|---|---|
| `id` | Exactly one each of `C1`, `C2`, `C3`, `C4`; order independent |
| `coverage` | `covered`, `missing` or `unknown`; describes positive coverage before considering defeat |
| `supporting_spans` | Explicit array, at most four spans; at least one for `covered`. Missing/unknown coverage may cite partial or ambiguous content |
| `coverage_explanation` | String, at most 1,024 UTF-8 bytes; nonblank for missing/unknown coverage, may be empty for covered |
| `defeat` | `present`, `absent` or `unknown`; the caller's whole-answer declaration, not a verified finding |
| `contrary_spans` | Explicit array, at most four spans; nonempty for present, empty for absent, optional for unknown |
| `defeat_explanation` | Nonblank string for every defeat value, at most 1,024 UTF-8 bytes |

Both lanes use the byte-exact span rules below, including required fields and rejection of null elements. Duplicate ranges within a lane reject; the same range may occur in both lanes or different criteria. All spans count toward the report's existing `verified_spans` total, so version 3 can count up to 32 lane entries within the unchanged 64 KiB response limit. The report does not publish their text or semantic interpretation.

The application combines the two declarations using the proposed table:

| Coverage | Defeat present | Defeat absent | Defeat unknown |
|---|---|---|---|
| `covered` | `unmet` | `met` | `unresolved` |
| `missing` | `unmet` | `unmet` | `unmet` |
| `unknown` | `unmet` | `unresolved` | `unresolved` |

Invalid or omitted enums reject before precedence is applied. Known missing coverage or declared present defeat gives unmet; otherwise uncertainty remains unresolved. Only declared covered content plus declared absence of defeat gives met. Those derived statuses feed the existing completion rule. Critical/unsupported flags are separate declarations: a critical flag does not reclassify every criterion, and an imperative is not mechanically classified as an unsupported factual claim.

For the synthetic answer `Keep edits. Discard edits.`, a single criterion entry could be:

```json
{
  "id": "C2",
  "coverage": "covered",
  "supporting_spans": [{"start": 0, "end": 11, "quote": "Keep edits."}],
  "coverage_explanation": "",
  "defeat": "present",
  "contrary_spans": [{"start": 12, "end": 26, "quote": "Discard edits."}],
  "defeat_explanation": "The contrary instruction defeats the preservation requirement."
}
```

This example is one criterion, not a complete response. Given the supplied declarations, it derives unmet. The reader does not determine whether the second sentence is endorsed, quoted, rejected, applicable or actually contrary to the requirement. Wrong absent or present declarations can therefore remain mechanically valid and produce wrong statuses. Explanations are validated for shape/bounds, never interpreted to correct a declaration. Do not convert older responses or notes into this format; new provenance and a frozen protocol are required for any later model result.

The reader supports all three versions. A protocol caller must require the expected `response_schema_version` in its report; accepting schema 3 offline does not authorize a provider to return it or establish a live input/usage contract. There is no live harness or new dispatch allowance here.

## Criterion and span fields, response versions 1/2

Each criterion object contains exactly `id`, `status`, `spans` and `explanation`. `status` is exactly `met`, `unmet` or `unresolved`; prose such as “met in part but defeated” is invalid. A met criterion needs at least one span. An unmet or unresolved criterion needs a nonblank explanation and may also cite spans, for example to identify a contradiction. Empty span lists are `[]`, not null. A met explanation may be empty, but its field must exist.

For future grading instructions, **unmet** includes clearly missing required substance, even without contradiction. **Unresolved** means genuine uncertainty about whether the answer satisfies a requirement. A source's underlying project state may remain unverified while the answer clearly meets a requirement to preserve that uncertainty. These are semantic definitions for the rubric/reviewer: the reader checks the declared status and explanation shape, not their correctness. It does not relabel an unresolved criterion as unmet based on words in its explanation.

Each span has exactly three required fields: integer `start`, integer `end` and string `quote`. Its half-open range `[start,end)` indexes UTF-8 bytes of the decoded frame answer: `0 ≤ start < end ≤ len(answer)`. Both boundaries must delimit complete code points. The quote must equal that exact substring. Duplicate ranges within one criterion fail; a range may support more than one criterion, and overlapping distinct ranges are permitted. These rules neither establish that the selected spans are sufficient nor require semantically equivalent answers to use identical wording.

For example, one criterion entry for the answer `Check state.` is:

```json
{
  "id": "C1",
  "status": "met",
  "spans": [{"start": 0, "end": 12, "quote": "Check state."}],
  "explanation": ""
}
```

This is one entry, not a complete four-criterion response. A multibyte character changes byte offsets; a quotation cutting a UTF-8 code point fails even if an offset would be a plausible character index.

## Completion and output

The command derives completion for all response versions. Version 1 rejects a contradictory supplied declaration; versions 2/3 have no supplied completion to compare. Version 3 first derives criterion statuses from its coverage/defeat declarations:

| Dispositions / critical judgment | Derived completion |
|---|---|
| Any unmet criterion, or critical error true | `false` |
| Otherwise any unresolved criterion, or critical judgment null | `null` |
| All four met and critical error false | `true` |

Unsupported-claim uncertainty remains separate; it does not silently become false or overwrite the completion rule. The command checks no semantic definition of unsupported claims or critical failures. A future run must freeze those definitions and their calibration evidence separately.

A mechanically valid report exits 0 with `status: mechanically_checked`, including when the derived completion is false or null. Output contains `response_schema_version`, criterion/status/span counts, derived completion, the two separately declared flags and input hashes. It omits IDs, requirements, answers, quotes, explanations, notes and paths. Even these aggregate declarations and digests may be private. An output-writer failure returns the same static error; bytes already written to that writer cannot be retracted, and contain only report metadata.

Every success report retains `semantic_relevance_verified: false`, `calibration_accepted: false` and `model_dispatch_authorized: false`. An actual quotation can be irrelevant, a met status can be mistaken and a pinned frame can contain a bad rubric. A separate source/rubric relevance review is required before any calibration progression decision. There is no keyword-based semantic oracle here, no acceptance threshold, live usage/cache parser, new model allowance or production eligibility.

Synthetic executable checks for response versions 1/2 cover exact decoding, caller pins, private-file boundaries, UTF-8 spans, omitted/null/unknown fields, completion precedence, criterion-order independence, invalid quoting and the real-but-irrelevant quotation boundary. A synthetic missing-coverage example with an unresolved status stays invalid when version 1 supplies false; version 2 derives null and still establishes no semantic truth. Tests also reject any completion field in version 2. These establish the implemented mechanical contract only. No model behavior or completed evaluation follows from passing these checks.

Version-3 checks cover all nine combination cells, completion/unknown precedence, separate-lane bounds and UTF-8 quoting, maximum lane counts, required non-null fields, unknown/mixed layouts, duplicate/aliased keys, forbidden supplied final statuses, privacy and CLI/writer behavior. Synthetic contradictory-answer and quoted/rejection controls explicitly demonstrate both directions of wrong semantic declarations remaining admissible with every relevance/acceptance/dispatch flag false. No model performance is inferred from these checks.

An additional private compatibility pass checks six new synthetic schema-3 responses built from the [frozen contradiction challenge](s0-whole-answer-adjudication.md): its twelve target observations produce the expected aggregate disposition counts, with C1/C4 supplied as context. All 173 prior private files remain unchanged. This is scripted input/reader compatibility, not generated grading or recovery of old results. The private verification report SHA-256 is `9375b6abdface22f0dc28c14b36501cabe94231e727c02849765d946c9b52af8`; the private bytes are not independently reproducible from this page.
