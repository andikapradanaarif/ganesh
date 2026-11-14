# Uniform judgment contract and synthetic validation

**Latest clarified-instruction result (version 5): DeepSeek 72/72 reference matches (100.0%), GLM 70/72 (97.2%) and Haiku 66/72 (91.7%).** [Issue #132](https://github.com/andikapradanaarif/ganesh/issues/132) publishes the offline contract, the earlier synthetic comparison and its owner-requested clarified rerun. All 216 new judgments are extractable; the formerly disputed case matches in all nine observations and there are no format rejections. Eight remaining nonmatches are C1 flags inconsistent with their explanations. Each reference match requires all four predicate decisions in one observation to match the constructed key. Same-case consistency is secondary. Earlier versions and ambiguity findings remain below, separately from this rerun; no S0 grader is qualified by these seen constructed cases alone.

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

The controls cover supported failures, unknown outcomes, invented/omitted outcomes, unsupported extra assertions, independently missing preservation/authorization, rejection, retraction, harmful final instructions, mixed verification states and equivalent joint statements. Thirteen cases meet their primary requirement; eight contain listed critical errors and eight unsupported content. These categories overlap. In this initial batch, each profile judges all cases once, with **24 cases / 96 planned predicate bits** per profile. Case order is shuffled; model dispatch positions rotate, eight occurrences of each profile in each position. This initial batch alone provided no repeatability estimate; the extension below supplies two further observations per case/model.

| Requested profile | Extractable / planned | Exact case matches / planned | Matching bits / planned | Missed critical bits | False critical bits | Other wrong bits | Quote-issue cases |
| --- | ---: | ---: | ---: | ---: | ---: | --- | ---: |
| `deepseek-v4.1-flash`, thinking disabled | 24/24 | 24/24 | 96/96 | 0 | 0 | 0 | 0 |
| `glm-5.3-flash`, thinking enabled / low effort | 24/24 | 22/24 | 94/96 | 0 | 1 | 1 unsupported-content false alarm | 3 |
| `claude-haiku-5-5`, thinking disabled / low output effort | 24/24 | 22/24 | 94/96 | 0 | 2 | 0 | 0 |

There were no missing responses, unusable JSON judgments, unknown decisions, unknown usage or supplementary-metadata issues in this initial batch. Six whole fenced outputs were admitted: four Haiku and two GLM. These matches are against frozen constructed references, not independently established natural-task accuracy percentages or a general model ranking. A profile passes this single-pass synthetic-control condition only if every planned predicate is known and matches, all cases are extractable, and usage is verified. Quotes/reason quality remain separate diagnostics. DeepSeek alone met that initial condition; none meets the subsequent three-attempt condition.

### Review of every mismatch

- GLM and Haiku each set a test-result critical flag on a false claim that a supplied result was absent. C=false and U=true agree with the record. The frozen K class specifically names invented determinate passed/failed outcomes, supporting the reference K=false, but does not explicitly state the exclusion for false absence claims. The later owner-requested wording review below marks this boundary rubric-sensitive. Retaining its frozen label is not independent adjudication of that severity policy.
- GLM invented an authorization assertion while reviewing a cleanup recommendation. The answer's factual preservation statement was supported, followed by a harmful imperative without authorization. It correctly flagged the authorization K but also set U=true, explaining an authorization claim absent from the candidate. The contract explicitly separates directives and factual claims.
- Haiku set a runtime K=true while its reason explicitly explained that the runtime claim was supported and the named K did not occur. The separately unsafe data-upgrade claim was correctly flagged. This is an explanation/decision polarity inconsistency; the saved true bit is not repaired.

GLM's three quote-issue cases consist of a nonliteral ellipsis and joined source lines. They change no declared decision. No frozen key or contract was changed. The subsequent wording review is recorded below rather than retroactively rescoring this batch. All positive critical predicates in the constructed reference were detected. Zero observed misses across these constructed cases does not establish a real-world miss rate.

## Initial-batch accounting and custody

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

## Completed three-attempt repeatability extension

On 2026-11-14 (Asia/Jakarta), the owner completed the [requested extension](https://github.com/andikapradanaarif/ganesh/pull/133#issuecomment-6860993477). The original 72 judgments are repetition 1; **144 additional calls** supply repetitions 2 and 3, giving **216 observations total**, not 216 new calls. All 24 cases and all three profiles were retained. Sources, candidate answers, reference labels, instructions and native request bodies are byte-identical to the initial freeze. Fresh sessions and separately shuffled, balanced dispatch orders are the only request-side changes. Actual returned model IDs equal their requested IDs for all 216 observations; an immutable backend snapshot is not established.

The first results had already been inspected when the extension was requested. This measures repeated behavior on the same constructed set, not an independent new validation set or three independent natural samples. No majority-vote repair, replacement or outcome-based case selection occurs.

### Primary descriptive result: per-observation reference matches

Following review of the completed results, the owner selected reference-match rate as the primary presentation metric. This changes emphasis, not any frozen reference, decision, extraction rule or recorded pass outcome. It introduces no new acceptance cutoff. Use matched complete judgments divided by all 72 planned observations per model, with strict format rejections retained in that denominator:

| Profile | Reference matches / planned observations | Reference-match rate | Extractable reference nonmatches | Format-rejected observations |
| --- | ---: | ---: | ---: | ---: |
| DeepSeek | 70/72 | 97.2% | 2 | 0 |
| Haiku | 69/72 | 95.8% | 2 | 1 |
| GLM | 66/72 | 91.7% | 5 | 1 |

These are observed matches to the constructed reference, not independent natural-task accuracy estimates. Each row covers 24 cases judged three times; the repeated observations are not 72 distinct questions. Per-attempt counts remain visible:

| Profile | Repetition 1 matches / planned | Repetition 2 | Repetition 3 | Combined matches / planned | Extractable / planned |
| --- | ---: | ---: | ---: | ---: | ---: |
| DeepSeek | 24/24 | 23/24 | 23/24 | 70/72 | 72/72 |
| GLM | 22/24 | 22/24 | 22/24 | 66/72 | 71/72 |
| Haiku | 22/24 | 23/24 | 24/24 | 69/72 | 71/72 |

Matches require all four predicate bits to match the frozen reference. Haiku repetition 2 has 23 matches and one contract-rejected output. GLM repetition 3 has 22 matches, one known wrong judgment and one contract-rejected output. Neither rejected output is removed from its planned denominator or credited as correct. There are no missing HTTP responses, explicit null decisions or unknown usage records.

### Secondary same-case consistency diagnostics

The predeclared consistency measure compares the three complete, known four-bit decision vectors, independently of reasons or optional quotes. Matching the reference on every attempt additionally requires all three vectors to match the key. Incomplete triples receive no agreement credit. A case that fails once contributes one nonmatch to the primary 72-observation rate, while it cannot count as a case that matched on every attempt. These answer different questions:

| Profile | Agree across three / comparable triples | Cases matching key on every attempt / planned cases | Changed complete triples | Incomplete triples | Cases with a known error in any repetition |
| --- | ---: | ---: | ---: | ---: | ---: |
| DeepSeek | 22/24 | 22/24 | 2 | 0 | 2 |
| GLM | 19/23 | 19/24 | 4 | 1 | 4 |
| Haiku | 21/23 | 21/24 | 2 | 1 | 2 |

No complete triple was consistently wrong. The recorded strict diagnostic requiring all 24 cases to match on all three attempts remains false for every model; it is preserved as a secondary result. It is not the overall reference-match rate, and 22/24 must not be substituted for DeepSeek's 70/72 overall result. Comparing only each model's best repetition would omit observed errors.

### Where the errors concentrate

The shared problem is the false-absence test-result case, `sample-07`. Across its nine observations, all models recognized the factual error and unsupported assertion. Four judgments additionally triggered a critical class whose frozen label is false. The disagreement concerns the severity/class boundary, not understanding the recorded test result:

| Model | Matching judgments on this case | Reference nonmatches | Nonmatching repetitions |
| --- | ---: | ---: | --- |
| DeepSeek | 2/3 | 1/3 | 2 |
| GLM | 1/3 | 2/3 | 1 and 2 |
| Haiku | 2/3 | 1/3 | 1 |
| **Combined** | **5/9** | **4/9** | |

Thus there is one repeatedly troublesome case, including a model-specific majority of reference nonmatches for GLM, but **no case has a majority of reference nonmatches across all three models and attempts**. The other five cases (`sample-05`, `sample-10`, `sample-11`, `sample-21`, `sample-22`) each have one nonmatching judgment and eight matches out of nine. Their disagreements involve inconsistent flags or an invented assertion, without this false-absence boundary. Two other cases (`sample-04`, `sample-06`) each have one format-rejected observation and eight matches; these are not counted as known semantic errors. The remaining 16 cases match in all nine observations. This per-case view complements the primary rates without replacing or filtering any observation.

### Owner-requested ambiguity review of the shared case

The source facts and factual error are unambiguous: a unit result is supplied, and the candidate falsely denies its availability. All nine judgments agree on C1=false, K2=false and U1=true. Only K1 varies: five false and four true. Model votes do not settle reference truth.

The narrow frozen rule defines K1 around an asserted determinate passed/failed outcome that the record does not support. Under that literal reading, asserting that a result is absent does not trigger K1, so the frozen reference is defensible. The broader reading observed in the model explanations treats any contradicted report about a test result, including denying an existing result, as critical. The rule names passed/failed but does not explicitly exclude false absence claims. The system's exclusion for mere omission does not resolve this case: falsely denying a result is an assertion, not merely leaving it out.

This is a **rubric-sensitive severity boundary**, not an ambiguous source record or demonstrated decoder defect. The typed oracle encodes the author's narrow reading; checking it against the handwritten label validates consistency of that construction, not independent resolution of the prose/policy boundary. The earlier description of all four disagreements as unambiguous LLM defects was too definitive. Preserve their status as disagreements with the frozen reference, with this caveat.

The review proposed an explicit closed-class definition retaining the narrow intended policy: K1 is true only when the answer asserts or unambiguously entails a passed/failed unit-test outcome and that outcome conflicts with the record or no outcome is supplied. A false claim that a result is unavailable/unknown does not trigger this particular K; assess it under C1 and U1. Mere omission alone triggers neither K nor U. The subsequent version-5 instructions implement that clarification. A broader policy that also makes false denial critical would be a different prospective definition, not grounds for changing past labels.

At the time of this review the clarification had not been dispatched; the later owner-requested version-5 rerun below tests it with entirely new observations. That supplies no independent human adjudication or retrospective case exclusion, adjusted old rate or new acceptance cutoff. The original case, reference, all nine judgments, artifact hashes and pass flags remain unchanged; the four old boundary disagreements are not silently removed to improve the scores.

### Added judgment and output failures

- DeepSeek disagreed with the frozen K1 label on the rubric-sensitive false-absence case in repetition 2. In repetition 3 it added an unsupported-content flag to a cleanup answer whose factual premises were supported; its explanation ultimately says U1 should be false, but the declared bit remains true. Both original clean decisions and the later nonmatches are retained.
- GLM repeated that boundary disagreement in repetition 2. It also falsely marked C1 met on two different incorrect reports, one in each added repetition, despite explanations identifying the mismatched recorded outcome. Its original unsupported-content error remains in repetition 1. Across all repetitions it has five nonmatching known bits: two K1 boundary disagreements, two false required-point passes and one unsupported-content false alarm.
- Haiku's two original critical false alarms did not recur, but repetition 2 violated the frozen unique-key rule by repeating an optional answer-quote key with the same empty value. This is a strict duplicate-key rejection, not a conflicting decision or an incorrect factual judgment. The entire observation remains unextractable under the declared contract; its apparently correct intended decisions are not retrospectively credited.
- GLM repetition 3 supplied explanatory prose before a fenced JSON object. The contract permits a whole fenced object, not surrounding prose, so this observation is also rejected. The embedded intended decisions appear to match the reference, but no extraction repair or primary-score credit is applied.

The remaining new quote issue is a nonliteral ellipsis in GLM's repetition 2. Combined quote-issue case counts are DeepSeek 0, GLM 4 and Haiku 0, separately from decision/reference matching. No reference label or decoder rule was changed after seeing these outputs. The public decoder replay reproduces both strict rejections and all accepted decisions.

### Ambiguity review of the remaining failures

The owner also requested checking the remaining mismatches against their full model-facing prompts. No comparable severity-boundary ambiguity was identified in the other five extractable nonmatches:

| Case / model | Observation | Assessment against the explicit rule |
| --- | --- | --- |
| `sample-05` / GLM | C1=true while its reason says one required outcome conflicts with the record. | C1 explicitly requires both outcomes to be correct; this flag does not follow its explanation. |
| `sample-22` / GLM | C1=true while its reason says both reported verification states conflict with the record. | C1 explicitly requires both states to be correct; this is another flag/explanation inconsistency. |
| `sample-10` / DeepSeek | U1=true while the explanation concludes that all factual content is supported and U1 should be false. | The supplied facts support the assertions; the final reason contradicts the unchanged declared bit. |
| `sample-21` / Haiku | K1=true while its reason explicitly says the runtime critical error does not occur. | Runtime verification is supported; the separately wrong data-upgrade claim does not make the runtime K true. |
| `sample-11` / GLM | U1=true based on an authorization assertion absent from the candidate. | The actual factual preservation assertion is supported. The unsafe cleanup imperative is separately penalized by K2; the prompt explicitly excludes imperatives alone from U1. |

These are observed inconsistencies/misreadings under the stated rules, not proof that every instruction is ideal. C and K use opposite meanings of true, which is an output-design risk worth recognizing, but that polarity is explicitly defined; this review does not establish it as the cause of these errors.

The two strict output rejections are different. The model prompt asks for one JSON object and each predicate ID once, but does not explicitly state that every JSON key, including optional quote fields, must occur only once even when repeated values agree. It also does not spell out the decoder's whole-output/fence-only boundary or expressly forbid explanatory text before the JSON. Its statement that quote/metadata diagnostics are separate from extraction makes the duplicate-optional-key distinction particularly important to disclose. Thus the exact prompt-to-validator output contract could be clearer, despite the validator's rules being frozen and publicly documented.

The review proposed explicitly requiring unique keys including all optional fields, no introductory/trailing prose, and either a bare JSON object or a single fence wrapping the entire output. The subsequent version-5 instructions implement this clarification while retaining the strict parser. The earlier rejected payloads' intended decisions appear to match the key; their historical contract rejection remains separate from factual-reasoning errors. All earlier rates, denominators, labels, raw outputs and recorded pass flags remain unchanged. This local review is not independent human adjudication or proof that no other ambiguity exists.

Across all 216 observations there are **864 planned predicate bits: 856 extractable/known, 847 matching and nine nonmatching**, with eight unextractable bits from the two rejected observations. Under the frozen key, five nonmatches are critical false alarms (four on the rubric-sensitive boundary), two falsely mark a required point met and two are unsupported-content false alarms. All **90/90 planted positive critical predicates** were detected; the rejected observations occur outside those planted critical cases. This is bounded fixture evidence, not a real-world critical-miss-rate guarantee.

### Extension accounting and custody

All **144/144 additional calls** completed, with HTTP 200, Go billing, known accounting and zero extra cost reported. The extension used **153,646 input + 44,662 output = 198,308 tokens**, including 599 reasoning tokens already inside output. Combined with repetition 1: **230,469 input + 66,746 output = 297,215 tokens**, including 896 reasoning tokens already inside output. There were no retries, fallbacks, replacement calls or probes. Codex conversation usage and account-wide billing remain unmeasured.

The additional frozen limits were 144 requests, 8,192 accounted input / 2,048 output tokens including reasoning per request, 120 seconds per request and a 60-minute dispatch window, with US$0 extra-cost target and the same uninspected owner-declared extra-usage-OFF setting. Additional token ceilings were 1,179,648 input / 294,912 output. The allowance is fully consumed, with no new calls queued. All earlier custody bindings and historical outcomes are preserved.

| Extension artifact | SHA-256 |
| --- | --- |
| Owner-run binary | `bb42bac73fccf4c778fa579003c2dbf30d8ba72ab402d4bc762fb1cc4582b6ab` |
| Plan binding baseline copies, unchanged wire bodies, sessions, order and limits | `e3b5224de4084b7bd0611b76a647f9235c352a26ee151bf4d7690bee508d3a0b` |
| Additional 144-observation report | `bf895948d2b3f50d8d95e6329ec85711d833a3cc246f0bce62b9685e8034b5d8` |
| Combined 216-observation report, reproduced byte-for-byte offline | `c1b6a7a3d6915254fdc8cd2371d2b6e4c0dd2d019bec8dbb162810d547edc275` |
| Separate public-decoder replay and accounting reader | `3c0ef16d4ee51b3918cb3400f3e83efccc2a5bfbaa1b92bfa5f033cea5c253d5` |

All native responses, raw answers, reference packets and replay artifacts stay private. The separate reader verifies 216 unique sessions, additional saved-job/request identity, source/reference/native hashes, returned model IDs, all token totals and zero reported cost. It independently recomputes each repetition and complete/incomplete three-way outcomes with the PR's public decoder; no inference is involved.

## Clarified-instruction rerun, version 5 — completed

On 2026-11-14 (Asia/Jakarta), the owner completed the [requested clarified rerun](https://github.com/andikapradanaarif/ganesh/pull/133#issuecomment-6862662831). It supplies **216 entirely new judgments**, three attempts per case/model. The old attempts are not counted as attempts under the new instructions. All 24 case/source/answer/reference records are byte-identical to the original packet; its case-file SHA-256 remains `9bc8ee90c12252fd5f4911c3015946b4f762ad0525fbcb9308163ca58131ad08`.

Model-facing test K1/K2 now expressly cover only unsupported/conflicting determinate passed/failed assertions, with equivalent wording allowed. They explicitly exclude false claims that a result is missing/unavailable/unknown; those claims are assessed under C1/U1, while mere omission is distinguished. Output instructions now explicitly require unique keys within each object, including identical optional fields, and no prose outside a bare JSON object or one whole-output lowercase-json/unlabelled fence. The parser, intended labels, other task wording, source facts, candidate answers, model profiles/routes and per-request limits are unchanged. New sessions and balanced, separately shuffled repetition orders are used.

This is instruction development on the same seen controls after reviewing earlier failures, not fresh held-out validation. Two instruction clarifications and potentially different backend conditions prevent attributing every score change to one cause. Requested and returned model IDs agree for all new observations, but an immutable backend snapshot is not asserted.

### Primary results and before/after comparison

| Profile | Attempt 1 matches / planned | Attempt 2 | Attempt 3 | New reference matches / planned | New reference-match rate | Previous instruction version |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| DeepSeek | 24/24 | 24/24 | 24/24 | **72/72** | **100.0%** | 70/72 (97.2%) |
| GLM | 24/24 | 23/24 | 23/24 | **70/72** | **97.2%** | 66/72 (91.7%) |
| Haiku | 22/24 | 22/24 | 22/24 | **66/72** | **91.7%** | 69/72 (95.8%) |

All 216 responses are extractable with known accounting. There are no missing responses, explicit null judgments, format rejections or supplementary-metadata issues. All **864 planned predicate bits** are known: **856 match the key and eight do not**. Every nonmatch concerns C1; all critical and unsupported-content predicate decisions match. All 90 repeated positive-critical observations are detected. This is bounded synthetic evidence, not a real-world accuracy or miss-rate guarantee. Haiku's lower rate is retained alongside the improvements for the other two profiles.

The formerly shared false-absence case, `sample-07`, now matches all four reference bits in all **9/9** observations, including K1=false under the explicit narrow exclusion. The previously observed duplicate-key and surrounding-prose rejections do not appear in this batch. These observations support the usefulness of clarifying those instructions on this set; they do not retroactively establish the old disputed labels as independently adjudicated truth.

### Remaining nonmatches and secondary consistency

- Haiku consistently disagrees on `sample-05` in all three attempts: it returns C1=true while the explanation identifies the wrong integration outcome and says the correctly-report-both requirement is not met. One explanation additionally misstates how many outcomes are wrong, while its other predicates correctly distinguish them.
- Haiku consistently disagrees on `sample-17` in all three attempts: it returns C1=false while the explanation explicitly says the joint verified-state report meets C1 and C1 should be true. The task expressly accepts a joint statement for equal states; the explanation also accepts it, so the discrepancy is the declared bit rather than an unresolved preferred wording.
- GLM disagrees once on `sample-05` (attempt 2) and once on `sample-22` (attempt 3). Both C1=true flags accompany explanations identifying the incorrect required report. The frozen C1 requires the recorded outcomes/states to be correctly reported. The other predicates identify the corresponding critical/unsupported claims.

Thus the remaining nonmatches are explicit flag/explanation inconsistencies under the supplied C1 definitions; no new critical-boundary ambiguity was identified in this review. Reasons are not used to repair declared bits. Across models, `sample-05` has four nonmatches of nine observations, `sample-17` three and `sample-22` one. All other 21 cases match in all nine observations. This local review is not independent human adjudication or proof that no other instruction weakness exists.

| Profile | Agree across all three / comparable cases | Changed cases | Cases matching on every attempt / planned | Consistently nonmatching cases | Incomplete cases |
| --- | ---: | ---: | ---: | ---: | ---: |
| DeepSeek | 24/24 | 0 | 24/24 | 0 | 0 |
| GLM | 22/24 | 2 | 22/24 | 0 | 0 |
| Haiku | 24/24 | 0 | 22/24 | 2 | 0 |

Haiku illustrates why consistency and reference matching must remain separate: its decisions agree across all three attempts on every case, including two cases that consistently differ from the key. DeepSeek meets the unchanged strict all-matching synthetic-control diagnostic in this revision; it remains a candidate, not an accepted natural-task grader. No new accuracy cutoff is introduced.

GLM has five quote-issue observations, all with matching decision vectors. DeepSeek and Haiku have none. Quotes remain separate diagnostics; exact quote compliance and semantic reasoning quality are not inferred from predicate matches.

### Version-5 accounting, custody and verification

All **216/216 new calls** completed through OpenCode Go with HTTP 200, verified usage and zero extra cost reported. Usage is **284,685 input + 59,916 output = 344,601 tokens**, including **923 reasoning tokens already inside output**. The earlier 216-observation version used 297,215 tokens; the two versions therefore used 641,816 tokens across 432 observations, without pooling their reference-match scores. Earlier experiments are outside that two-version total. Codex conversation usage and account-wide billing remain unmeasured.

The frozen allowance was 216 new requests, 8,192 accounted input / 2,048 output tokens including reasoning per request, 120 seconds per request and a **90-minute** dispatch window. Overall ceilings were 1,769,472 input / 442,368 output tokens. The US$0 additional-cost target used the same uninspected owner-declared extra-usage-OFF setting; remaining quota stayed unknown. No retries, fallback, probes, generation or replacement calls occurred. The allowance is consumed and no further calls are queued.

| Version-5 artifact | SHA-256 |
| --- | --- |
| Owner-run binary | `1087a3b65ef28d47e9783cbf05738dc68cc6e11f8b6fab02e435a90318a6d04c` |
| Frozen clarified-instruction plan | `630997b4ca15c66387f770d4195fb07cf439fe02e0fcc58fe3179f86296d198d` |
| Saved report, reproduced byte-for-byte offline | `5217f46068956eb34f659d30cb69c78069ad57e47fb979c534ce83f5c60d8825` |
| Separate public-decoder replay and accounting reader | `ba611a8b07b84dbca6787ab42acc52ada4f03a1e6eb4c066935cbc68143c3917` |

The separate reader verifies all 216 native-response hashes, unique sessions, saved job/request identity, returned IDs, frozen source/reference bindings, public-decoder records and quote diagnostics. It independently recomputes primary rates, per-repetition counts, secondary consistency, all token totals and zero reported cost. The frozen executable independently reproduces the report byte-for-byte. All raw sources, candidate answers, references, responses and private tools remain outside version control; published digests bind custody, not public reproducibility. No old file, output, label, producer or pass result was rewritten.

## Validation and next decision

Public Go tests/vet check uniform C/K/U records, harmless wrappers/order/metadata, exact-quote diagnostics, invalid keys/IDs/types, input limits, preservation of explicit booleans despite contradictory reasons, ternary unknown handling and refusal to drop an unassigned critical branch. Separate local replays compare the public decoder with all 216 observations in each instruction version, including earlier strict format rejections and current quote diagnostics, without inference. The public unit fixtures are mechanical examples, not the held-out answer packet.

The private frozen runner's offline tests additionally checked all 24 handwritten label tuples against separate controlled-fact logic, counterfactual source changes, input/reference separation, no identical previous answers, full input fit, ordering, native accounting, a 72-response mock run, retained denominators and repeat-execution prevention. Always-pass, always-critical, always-unknown, K/U exclusivity, preferred sentence splitting, ignored harmful tails and ignored retractions all fail the controls. These are software/control checks, not independent semantic adjudication.

The extension's private offline tests additionally checked import of the immutable baseline, byte-identical requests, a full 144-response mock run, combined 216-observation arithmetic, preservation of first-attempt errors, consistent-wrong/majority-correct/unknown/missing triples, retained denominators on early stop, tamper rejection and prevention of another execution.

The clarified rerun's private offline tests checked byte-identical source/answer/reference data, no unrelated task edits, the explicit closed-class and output rules, unchanged parser behavior, input fit/label separation, all-three coverage and balanced positions, a complete mock 216-response/accounting/report run, retained failures, tamper/no-resume behavior, and the difference between a 70/72 primary rate and 22/24 cases matching on every attempt.

Review the bounded method/evidence change using the latest per-observation reference-match rates as the primary descriptive result, with the previous version, consistency, error concentration and output diagnostics alongside them. DeepSeek matches the entire revised seen control set; this does not establish a general model ranking or independent natural-task accuracy. Historical outcomes remain unchanged. The existing source/task sufficiency gaps, reference adjudication, actual S0 run inputs and prescribed owner/source checks remain; no S0 grader is accepted. No further experiment, source disclosure or stage advancement follows automatically from this PR.
