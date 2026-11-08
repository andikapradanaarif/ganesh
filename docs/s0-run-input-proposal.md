# S0 run-input proposal

Continuation of [#60](https://github.com/andikapradanaarif/ganesh/issues/60), under [S0 #6](https://github.com/andikapradanaarif/ganesh/issues/6). **Proposal only; no evaluation execution is authorized or frozen.** The six [calibration drafts](s0-calibration-candidates.md) and their recorded preparation overrun remain separate evidence. The approved [execution sheet](s0-execution-sheet.md) governs the run.

## Proposed model profile

Checked against official documentation on 2026-11-08. Account availability, API permissions and returned model/version metadata have not been checked.

| Role | Proposed request ID | Documented context | Standard uncached text price per million tokens, input / output |
| --- | --- | ---: | ---: |
| Answering | `claude-opus-5-5` | 1,000,000 tokens | US$4 / US$20 |
| Grading | `gpt-6-astra` | 1,050,000 tokens | US$10 / US$50 |

Anthropic documents Opus 5.5 for coding/knowledge work with adaptive thinking and default medium effort. [Official model specification](https://platform.claude.com/docs/en/models/opus-5-5/overview). OpenAI documents medium reasoning effort for GPT-6 Astra; its evaluation guidance recommends starting with Astra as a strong judge and validating agreement against human labels. [Official model specification](https://developers.openai.com/api/docs/models/gpt-6-astra), [evaluation guidance](https://developers.openai.com/api/docs/guides/evaluation-best-practices).

This is a proposed Claude-answering/OpenAI-grading arrangement consistent with the execution sheet's family separation. It has no empirical calibration result yet. Before freezing, verify account access, supported request settings, exact requested and returned model identifiers, and version stability. Record any absence of an immutable provider version; do not invent a dated snapshot ID or silently substitute a model.

The model identity freeze record must retain the provider/endpoint and API version, literal requested model ID, literal returned model ID (or an explicit absence), any returned version/fingerprint (or `unavailable`), observation time and request-configuration digest. If the returned ID merely repeats the requested name, record that result without treating it as proof of an immutable version. Any unavailable version binding remains a reproducibility limitation.

## Limits and cost

| Input | Proposal or unresolved decision |
| --- | --- |
| Answer output allowance | Propose 8,192 billable output tokens per answer, including thinking/reasoning where the provider accounts it within that limit; verify the API contract before execution. |
| Grader output allowance | Propose 4,096 billable output tokens per grade; calibrate whether this supports the rubric. |
| Effort | Propose medium for both; confirm supported settings and freeze their exact request representation. |
| Total API spend cap | **Owner decision pending.** No spend is authorized by this proposal. |
| Per-answer / grading timeouts | **Owner decision pending.** Count timeouts and interrupted/ambiguous calls rather than replacing them silently. |
| Calibration and retry allowance | **Unapproved.** Start with no automatic retries; freeze any positive allowance and its cost before calls. |
| Provider disclosure | **Pending** for OpenAI/Anthropic evaluation APIs. Current permission covers bounded review in this assistant only. |

Illustrative output-only ceilings at the proposed caps, assuming one grade per generated answer and standard short-context rates:

| Answer/grade pairs | Answer output maximum | Grader output maximum | Combined output-only maximum |
| --- | ---: | ---: | ---: |
| Six calibration pairs | US$0.983040 | US$1.228800 | **US$2.211840** |
| 108 base-answer pairs | US$17.694720 | US$22.118400 | **US$39.813120** |
| 132 triggered-answer pairs | US$21.626880 | US$27.033600 | **US$48.660480** |

These are arithmetic scenarios, not forecasts or approved budgets. They omit input tokens, additional labelled grader-calibration cases, extraction/card preparation, tools, retries and failures. The grading call frame is not frozen. No cache or batch discount is assumed. GPT-6 Astra prompts above 272,000 input tokens have higher whole-request rates, so those rows would need recalculation for long grader prompts. [OpenAI pricing on the model page](https://developers.openai.com/api/docs/models/gpt-6-astra). Count actual usage for both providers, including unsuccessful calls, and stop before a request could exceed the approved remaining allowance.

## Corpus and calibration readiness

The available preparation includes 27 local-only Devin excerpt candidates and 21 local-only Crush digest candidates. They are different source classes with unresolved overlap, redaction loss and semantic coverage; file bytes are not model token counts. The six reviewed excerpts are a calibration selection, not the complete D/F corpus. Raw Devin/Codex transcripts have not been admitted as full-context inputs. Baseline project notes and generated typed cards remain unprepared or unenrolled.

Before a context-fit claim, choose the complete permitted summary corpus and freeze the exact D/F record set, revisions and origin labels. Count the fully assembled request using each selected model's tokenizer, with output/reasoning reserves and wrapper/tool overhead. Provider token-count requests also disclose their input and await the same permission. A context-window specification alone does not establish fit; never omit F or truncate its input silently.

Before grading calibration, prepare contrasting labelled answer cases for the private rubrics, obtain the required owner labels, and test the proposed grader against them. Preserve attributed claims, abstentions, critical/unsupported flags and disagreements. The same assistant authored the current task/oracle drafts; that is not an independent human calibration. The later required owner checks of critical flags/disagreements, six primary pairs and three diagnostic sets still apply. Fresh sessions, held-out task/scenario separation, unchanged baseline inputs and the 108/132 schedule must be frozen before held-out answers. These controls come from the approved execution sheet; none has passed by issue creation or this proposal.
