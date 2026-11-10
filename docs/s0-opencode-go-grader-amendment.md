# S0 OpenCode Go grader amendment

[Issue #83](https://github.com/andikapradanaarif/ganesh/issues/83), continuing [calibration #60](https://github.com/andikapradanaarif/ganesh/issues/60). **Owner-selected candidate; calibration and execution inputs remain unfrozen.** No model request or private provider disclosure has occurred under this amendment.

## Decision and scope

The owner reported “OpenAI only and Opencode Go”, asked “can i use opencode go deepseek flash 4.1”, and then said **“lets use go”** after the proposed DeepSeek grading role and need for a pre-run amendment/calibration were explained. In that immediate context, this records selection of **DeepSeek V4.1 Flash through OpenCode Go as the candidate grader for OpenAI-family answers**. It does not infer working model access, returned identity, disclosure permission or execution/spending authority.

This is an explicit run-specific exception to the [supplied execution sheet's named Anthropic grader](s0-execution-sheet.md#score-and-stop) and the [roadmap's named grading arrangement](personal-second-memory-roadmap.md#appendix-a-experiments-before-s1a). Their original text remains preserved. Only that family selection changes for this proposed run: different-family grading, source-keyed expected outcomes, separate calibration, owner checks of critical/unsupported flags and unknowns/disagreements, prescribed spot checks and honest incomplete outcomes remain required. A different family does not establish grader quality or independent truth.

The [earlier run-input proposal](s0-run-input-proposal.md) is historical; its Claude-answering/OpenAI-grading roles and illustrative prices are not this run's selected profile. No other roadmap scope, capture contract, task split, 108/132-answer rules or stage gate changes here.

## Candidate identity and access

Checked against [official OpenCode Go documentation](https://opencode.ai/docs/console/go/) on 2026-11-10:

| Input | Selected or pending |
| --- | --- |
| Answering family | OpenAI; exact model/client/build remains pending. |
| Grading family/model | DeepSeek / DeepSeek V4.1 Flash, through OpenCode Go. |
| OpenCode configuration ID | `opencode-go/deepseek-v4.1-flash`. |
| Documented provider request ID | `deepseek-v4.1-flash`. |
| Documented route | `https://opencode.ai/inference/go/openai/v1/chat/completions`; OpenAI-compatible transport does not make the model OpenAI-family. |
| Actual account access and returned ID/version | Unverified; no request has checked them. |
| Immutable snapshot/fingerprint | Unknown; no dated identifier is invented. |
| Complete-request context fit | Unverified; model listings and corpus file bytes are insufficient. |

The owner reports an existing Go subscription. The public catalog does not prove workspace model access, region permissions, remaining allowance or compatible client request metadata. Record requested versus returned identity and any unavailable version binding when an authorized test exists. No silent model/provider fallback or substitution is permitted in a frozen run.

## Data and subscription boundary

OpenCode documents Global-region routing for DeepSeek, no training use and a zero-day-retention agreement renewed monthly, currently valid through **2026-11-30**. These are provider declarations, not this repository's attestation. Check current terms and the actual route immediately before any authorized disclosure. [Provider settings and privacy](https://opencode.ai/docs/console/go/).

Current processing permission covers this assistant's bounded review of the six already scrubbed calibration excerpts. Selecting a model does not expand that permission to OpenCode/DeepSeek. Approval for the exact assembled private packet and processor/region remains pending; no raw transcripts, credentials or Nexus material are enrolled. Required final assembled-request scrubbing and model-output scrubbing remain in force.

**Additional paid spending remains $0.** OpenCode documents that Extra Usage can consume paid credits after allowance exhaustion. Keep it off and verify the account setting before execution; its current state is unknown. Use included allowance only, stop on exhaustion and do not buy credits or enable fallback. Remaining quota is unknown; an advertised allowance cannot guarantee that a session stays within a five-hour usage window. [Go usage policy](https://opencode.ai/docs/console/go/).

## Calibration sequence

First finish the outstanding source-only owner review of the selected Valo case. Owner labels remain declarations requiring rubric/source support; assistant-proposed labels cannot fill them. A private single-case grader handoff may be prepared from the existing frozen task/rubric/oracle, S2 excerpt and neutral answer, with owner/proposed labels withheld. Preparation grants no disclosure or model-run authority.

Before a first diagnostic request, freeze its exact payload/hash, account/client/model binding, permitted processors/region, input/output/reasoning limits, elapsed timeout, request/retry allowance, quota accounting and no-overage state. Those limits are still unapproved; one UI prompt must not be assumed to equal one provider request if the client makes auxiliary calls or retries. No API runner or credential inspection is added.

After an authorized single-case format/usage check, preserve the original result, failures, usage and model metadata. That check cannot accept the grader. Complete and compare the 18 calibration cases against owner judgments, predeclare the acceptance/disagreement rules, and resolve required owner checks before a grading acceptance claim. Unknowns stay unknown; the existing [calibration comparison](s0-calibration-comparison.md) always reports acceptance separately from declaration agreement.

The 30 held-out tasks, complete permitted corpus, baseline instructions/typed cards, context fit, model/limit/permission freeze and answer schedule remain pending. S0 may continue with capture honestly marked unknown under the roadmap; no further capture expansion is required by this amendment. No held-out answer, grader acceptance, narrative activation or service-stage pass follows from the model choice.

## Private preparation evidence

A local Go preparer assembled **one private Valo case, 6,697 UTF-8 bytes, with five source evidence spans** from the existing frozen inputs. It verified the fixture-to-task bindings, source/answer hashes and each exact evidence string against its nominated excerpt. The model input includes the task, rubric, source-keyed expectations, source excerpt and neutral answer; it excludes owner/proposed labels and original session/file provenance metadata. The private sidecar binds the assembled input and original artifacts by SHA-256. No original fixture or blank-label file was rewritten.

The draft, manifest and owner-choice record remain gitignored with 0700 directory / 0600 file modes. A declared literal-pattern precheck passed; this is not a universal secret-safety claim or a substitute for final assembled-request review. Owner judgments remain incomplete and no grader result exists. Preparing this handoff establishes neither model access nor provider permission, usage consumption or calibration acceptance. Private evidence is locally checkable; the public aggregate is not independent adjudication.

Validation rechecked the saved input digest/size, original fixture-file hashes and task/source bindings, private modes and git exclusion. The existing offline summary reports zero complete owner cases, no grader packet and acceptance false. All **37 relative file-target occurrences** in README, the historical run-input proposal and this amendment resolve; the two supplied-document anchors were checked against their headings. The supplied roadmap/execution sheet are unchanged and `git diff --check` is clean. No runtime implementation changed.
