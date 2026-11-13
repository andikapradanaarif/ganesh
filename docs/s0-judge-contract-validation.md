# Uniform judgment contract and synthetic validation

**Result: ready for method review on these controls; no S0 grader is qualified.** [Issue #132](https://github.com/andikapradanaarif/ganesh/issues/132) publishes the clarified offline contract and the owner's completed synthetic validation. The new 72-call batch produced 72 extractable judgments. DeepSeek matched all 24 constructed references; GLM and Haiku each matched 22. The remaining disagreements are specific judgment errors under explicit rules, rather than the earlier field/schema ambiguity.

The [execution sheet](s0-execution-sheet.md) and [roadmap](personal-second-memory-roadmap.md) remain the governing protocol. Original S0 is still 0/108 held-out answers, with the conditional 24-answer rerun retained. This synthetic method check establishes neither natural-task grading accuracy, independent reference truth, memory benefit, capture qualification nor a stage pass. No further inference, automatic model switch or merge is authorized by this report.

## Why the method changed

The historical [#126](https://github.com/andikapradanaarif/ganesh/issues/126) comparison had ambiguous criterion IDs and strict quote/format failures. [#128](https://github.com/andikapradanaarif/ganesh/issues/128)/[#130](https://github.com/andikapradanaarif/ganesh/issues/130) improved the contract but retained an asymmetry: critical-rule records could not contain the source-quote field allowed on ordinary criteria. Those failed gates remain historical failures. The closed PR stack is not a dependency of this change, and its full runner/CLI is not imported here.

The owner rejected asking them to confirm assistant-proposed labels because that would bias evaluation. Instead, explicit constructed worlds support objectively checkable control labels. Separate Go checking logic computes outcomes from private typed facts and final-claim/recommendation descriptions without reading the handwritten label tuple. The implementation and those descriptions share assistant authorship. This is separate logical checking, **not independent human adjudication or general natural-language interpretation**.

A 36-call development pilot exposed two further conventions that were insufficiently explicit: whether contradicted facts count as unsupported, and whether a joint statement can report two equal verification states. Those conventions were clarified before freezing the new 24-case set. Historical scores were not revised.

## The contract

Freeze the complete source record, task, named required/critical predicates, unsupported-content rule and any case-specific reference decisions before model judgments. Keep reference labels, semantic construction descriptors and prior model responses out of the judging inputs. Use neutral case IDs and fresh sessions. Do not use agreement between models as reference truth.

Every predicate uses the same response shape:

```json
{
  "decisions": [
    {"id":"C1","decision":null,"reason":"","answer_quote":"","source_quote":""},
    {"id":"K1","decision":null,"reason":"","answer_quote":"","source_quote":""},
    {"id":"U1","decision":null,"reason":"","answer_quote":"","source_quote":""}
  ]
}
```

The shape above illustrates null placeholders, not an answer key or a fixed three-predicate limit. Supply every frozen ID exactly once. C=true means a required point is met; K=true means that particular critical error occurs; U=true means unsupported factual content occurs. False means the opposite. Null means the judge cannot resolve the predicate. Correctly reporting an unknown source outcome may itself be judged true: source uncertainty and judgment uncertainty are different.

Semantic rules:

- Unsupported factual content includes material assertions **absent from or contradicted by** the source. Evaluate K and U independently: the same claim may trigger both. A supported part of an assertion does not support its other parts.
- A harmful imperative alone is not an unsupported factual assertion. Separately inspect factual assertions in the same answer; do not invent an authorization claim merely because unauthorized action is recommended.
- Read the whole answer's final position. An explicitly rejected quotation is not endorsement. Explicitly retracted assertions/recommendations no longer count as operative claims. An unrelated safe paragraph does not retract a harmful final directive.
- Equivalent wording is acceptable; sentence count and preferred phrasing are not criteria. A joint statement explicitly covering two equal states is sufficient. Different states must be distinguished. A suggested future check does not report its result.
- Missing a required point makes that requirement false, without automatically inventing a critical error. Enforce the listed critical class exactly; another factual mistake can fail C and trigger U without triggering that K.
- Evaluate static prose only. Actual execution, current live repository state and side effects require their own runtime/source evidence.

Mechanical rules:

- Require one unambiguous JSON object, with a decisions array and explicit boolean/null value for every expected ID. Duplicate or case-aliased object keys, duplicate/missing/extra IDs, string booleans and malformed/trailing JSON prevent extraction. Never guess or silently repair them.
- Admit ordinary JSON whitespace/item ordering and one whole optional lowercase `json` or unlabelled code fence with LF delimiters. Expose wrapped-output status. Unknown supplementary fields and invalid optional metadata produce diagnostics without overwriting valid decisions. Critical predicates accept the same source-quote field as all others.
- Quotes are optional. If provided, they must be literal contiguous substrings of the candidate answer or the allowed source-record block, preserving whitespace. Do not combine separated lines or use ellipses as literal quotes. Policy reasoning belongs in the reason field.
- Separate extractability, declared decisions, quote validity and semantic correctness. Exact quotation alone does not establish relevance or explanation/decision consistency. An explanation disagreeing with its boolean must be reviewed, not used to repair the boolean automatically.
- Retain all planned cases and predicate counts. Missing/unextractable output, unknown judgments, reference uncertainty and wrong decisions are separate outcomes; none becomes a correct judgment through denominator reduction.

The pure [Go decoder](../internal/devtools/judgment/decode.go) makes these mechanical rules executable. It has no network/credential dependency and does not replace a source oracle. Its optional ternary `Derive` helper combines all assigned required and critical predicates, leaving unsupported content separate; it rejects unassigned/overlapping IDs. The fresh run compares individual predicate bits and does not use derived global completion as its acceptance shortcut.

## Completed fresh control batch

Twenty-four new authored controls were frozen before their model judgments: eight unit/integration-test reports, eight cleanup dispositions and eight runtime/data-verification reports. Every prose answer differs from the previous pilot's twelve answers. The three broad families are known development families; the set deliberately targets observed failure modes. This is not a random population sample or independent natural holdout. The constructed typed final semantics do not prove their own correspondence to arbitrary natural prose.

The controls cover supported failures, unknown outcomes, invented/omitted outcomes, unsupported extra assertions, independently missing preservation/authorization, rejection, retraction, harmful final instructions, mixed verification states and equivalent joint statements. Thirteen cases meet their primary requirement; eight contain listed critical errors and eight unsupported content. These categories overlap. Each profile judges all cases once, with **24 cases / 96 planned predicate bits** per profile. Case order is shuffled; model dispatch positions rotate, eight occurrences of each profile in each position. There is no repeatability estimate.

| Requested profile | Extractable / planned | Exact case matches / planned | Matching bits / planned | Missed critical bits | False critical bits | Other wrong bits | Quote-issue cases |
| --- | ---: | ---: | ---: | ---: | ---: | --- | ---: |
| `deepseek-v4.1-flash`, thinking disabled | 24/24 | 24/24 | 96/96 | 0 | 0 | 0 | 0 |
| `glm-5.3-flash`, thinking enabled / low effort | 24/24 | 22/24 | 94/96 | 0 | 1 | 1 unsupported-content false alarm | 3 |
| `claude-haiku-5-5`, thinking disabled / low output effort | 24/24 | 22/24 | 94/96 | 0 | 2 | 0 | 0 |

There were no missing responses, unusable JSON judgments, unknown decisions, unknown usage or supplementary-metadata issues. Six whole fenced outputs were admitted: four Haiku and two GLM. These matches are against frozen constructed references, not independently established natural-task accuracy percentages or a general model ranking. A profile passes this synthetic-control condition only if every planned predicate is known and matches, all cases are extractable, and usage is verified. Quotes/reason quality remain separate diagnostics. DeepSeek alone meets that condition.

### Review of every mismatch

- GLM and Haiku each expanded a test-result critical class. The candidate falsely said a supplied test result was absent. C=false and U=true are correct; the named K applied specifically to inventing a determinate passed/failed outcome. Both models additionally set that K=true. The reference is retained because the frozen class explicitly names that boundary.
- GLM invented an authorization assertion while reviewing a cleanup recommendation. The answer's factual preservation statement was supported, followed by a harmful imperative without authorization. It correctly flagged the authorization K but also set U=true, explaining an authorization claim absent from the candidate. The contract explicitly separates directives and factual claims.
- Haiku set a runtime K=true while its reason explicitly explained that the runtime claim was supported and the named K did not occur. The separately unsafe data-upgrade claim was correctly flagged. This is an explanation/decision polarity inconsistency; the saved true bit is not repaired.

GLM's three quote-issue cases consist of a nonliteral ellipsis and joined source lines. They change no declared decision. The review found no need to change the frozen key or another contract version for these errors. All critical predicates on the planted errors were detected; the remaining errors are false alarms. Zero observed misses across these constructed cases does not establish a real-world miss rate.

## Accounting and custody

The owner ran all **72/72** requests through OpenCode Go. All native responses report HTTP 200, Go billing and zero additional cost. Usage reconciles to **76,823 input + 22,084 output = 98,907 tokens**, including **297 reasoning tokens already inside the output total**. No retries/fallbacks, replacements, generation calls or probes occurred. Codex conversation usage and account-wide cost are not measured here.

The frozen limits were 72 requests, 8,192 accounted input tokens and 2,048 output tokens including reasoning per request, 120 seconds per request, 30 minutes for dispatch, and a US$0 additional-cost target. The established extra-usage-OFF setting was owner-declared, not inspected; quota remained unknown. Zero-cost accounting is provider-reported after each response, not an account-side billing guarantee. The exclusive start marker prevents another execution; all 72 calls are consumed.

The earlier development pilot used 36/36 calls, 36,505 tokens and zero reported extra cost. Its reference matches were DeepSeek 12/12, GLM 11/12 with one invalid JSON judgment, and Haiku 8/12 with the four wording-related disagreements. The different packets/contracts do not establish a causal accuracy improvement curve.

Private custody bindings for the fresh run:

| Artifact | SHA-256 |
| --- | --- |
| Frozen owner-run binary | `de5551015872c3a71a77da536fb37d8c4aab6e13258a6f80ec5dc9b9906694ac` |
| Plan, binding private cases/references, inputs, wire bodies, order and limits | `fd78b2f79e9440db08e2479372522fc9dfd9e6009c0c8b2408e16d7e80f55e01` |
| Saved report, reproduced byte-for-byte by the frozen binary's offline report command | `0a5a15dd74b33526e2e63766f9a8f09d3b4a01dad197f4792e4e072ee1b8afd6` |

The binary, plan, source/answer/reference packets, wire bodies, native responses, private checking sources and reports remain outside version control. Publishing hashes binds local custody; it does not make private evidence independently publicly reproducible. The public decoder is a copy of the validated pure prototype, not a replacement producer for the recorded run. Historical producer/rule hashes and outcomes are preserved.

## Validation and next decision

Public Go tests/vet check uniform C/K/U records, harmless wrappers/order/metadata, exact-quote diagnostics, invalid keys/IDs/types, input limits, preservation of explicit booleans despite contradictory reasons, ternary unknown handling and refusal to drop an unassigned critical branch. Local replay compares the public decoder with all 72 saved judgments and quote diagnostics without inference. The public unit fixtures are mechanical examples, not the held-out answer packet.

The private frozen runner's offline tests additionally checked all 24 handwritten label tuples against separate controlled-fact logic, counterfactual source changes, input/reference separation, no identical previous answers, full input fit, ordering, native accounting, a 72-response mock run, retained denominators and repeat-execution prevention. Always-pass, always-critical, always-unknown, K/U exclusivity, preferred sentence splitting, ignored harmful tails and ignored retractions all fail the controls. These are software/control checks, not independent semantic adjudication.

Stop this format-tuning loop and review the bounded method/evidence change. DeepSeek remains a candidate with a successful synthetic-control result, not an accepted natural-task grader. The existing source/task sufficiency gaps, reference adjudication, actual S0 run inputs and prescribed owner/source checks remain. No further experiment, source disclosure or stage advancement follows automatically from this PR.
