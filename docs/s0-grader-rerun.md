# Owner-run corrected-contract comparison

**Completed, grading incomplete.** The owner ran all 180 requests on 2026-11-13. The allowance is consumed; do not rerun the historical command below. The [results](#completed-run-and-remaining-failures) supersede preparation/pending statements. No grader is accepted or further call authorized.

[Issue #130](https://github.com/andikapradanaarif/ganesh/issues/130) follows [the contract fix](s0-grader-contract.md) after the owner requested a fresh run. The owner prefers to execute model scripts locally. This prepares that run; no provider call is made during preparation. Reuse the same 18 synthetic development answers and 12 saved generated answers over six tasks, with frozen same-author references. Do not regenerate answers or claim new held-out tasks.

## Frozen scope and limits

| Item | Prospective setting |
| --- | --- |
| Profiles | `deepseek-v4.1-flash`: native Go Chat Completions, thinking disabled; `glm-5.3-flash`: same native route, thinking enabled/low; `claude-haiku-5-5`: native Go Anthropic Messages, thinking disabled/low effort. Native profile differences remain explicit comparison limits. |
| Requests | 30 answer cases × three graders × two fresh sessions = **180**, one in flight. No generation, probes, retries, replacements, fallback, redirects or automatic resume. |
| Tokens | **12,288 input**, **2,048 output including reasoning** per call; 2,211,840 input / 368,640 combined output overall. Reasoning is included, never added twice. |
| Input preflight | Full system/user UTF-8 bytes + 512 wrapper allowance must fit the input ceiling; wire body at most 32 KiB. This conservative proxy is not exact tokenizer proof. Native actual input/output counts are reconciled and bounded after each call. No truncation or compression. |
| Time | 120 seconds per request, 90 minutes from batch start. |
| Spend | $0 additional paid spend; owner previously declared Extra Usage OFF. Account state/remaining quota are not independently inspected. Missing/nonzero cost or inconsistent accounting stops the batch. |
| Data destinations | The same previously reviewed source excerpts and saved answers through OpenCode Go to the three selected processors; no new sources, raw transcripts or direct paid API route. Existing scoped processor grants persist. |

The previous 8,192 input-token ceiling's byte proxy rejects eight full corrected prompts. Before any new output, this run raises the explicit input ceiling to 12,288 rather than shortening instructions. This changes the ceiling, not the 180-request/output/spend limits or an assertion of actual token use. The earlier #126 allowance remains exhausted; this is a separately recorded prospective run.

## Execution and saved evidence

Build `cmd/ganesh-dev` once into a private local executable, then run its offline `grader-trial-prepare` command against the fixed packet. The resulting `plan.json` pins the executable hash, packet, references, all 90 unique native request bodies, random ordering seed, 180 jobs and progression rule. Preparation validates every body without reading credentials or requesting tokens/models. All artifacts remain outside git in 0700/0600 storage.

```sh
go build -o .local/s0/grader-rerun-130-runner ./cmd/ganesh-dev
.local/s0/grader-rerun-130-runner grader-trial-prepare \
  --dir .local/s0/grader-contract-128 \
  --manifest-sha256 3a1a314a1dc096062653a7fdd0d80d6c2105987a3c9fcdf3ec898cf82ee0f29a \
  --output-dir .local/s0/grader-rerun-130
```

For the actual prepared packet, use the local `run.sh` wrapper described in the validation record. It invokes that same frozen executable with the printed plan hash and `--execute`. Only this explicit execution path reads the existing `GANESH_S0_GO_API_KEY` / `OPENCODE_API_KEY` environment value, or the literal `OPENCODE_API_KEY` assignment in the previously identified local Crush secrets file. It never prints or writes the key or sources that file as shell code. The first actual model call doubles as access validation within its planned slot.

Every attempt saves its request, fresh-session identity, raw native response, extracted answer text, usage, finish reason and layered contract check privately. The terminal prints counters, not source text or key values. Valid usage with malformed/truncated model grading content is a failed observation, not a replacement request. Transport/model identity/accounting/limits failures stop the whole batch and retain its unused/missing denominator. A durable exclusive start marker prevents restarting or resuming, including after a crash.

The automatic `report.json` separates plain JSON/fences, structure, exact evidence, declared critical flags before evidence admission, admitted reference comparison, critical controls/unknowns, false alarms against proposals and repeat changes in known global flags. On completion or a handled stop it is written automatically. After an interrupted process, the same executable can reconstruct an offline report:

```sh
.local/s0/grader-rerun-130-runner grader-trial-report \
  --dir .local/s0/grader-rerun-130 --plan-sha256 ACTUAL_PLAN_SHA256
```

Reporting rereads each raw native response, verifies its saved digest, recomputes accounting and checks against the frozen contract; it makes no calls. Missing/incomplete attempts remain missing/unknown. Failure before a complete artifact write may require inspection of the saved reservation/raw files; never rerun the network command to repair evidence.

## Predeclared interpretation

Development operability requires every one of a profile's 60 planned responses to complete, pass structure and exact evidence checks, and have known derived completion/critical/unsupported decisions. All 12 planted-critical trials must be detected; any miss or unknown blocks progression. Report disagreements and unknown/disputed references separately, keeping all planned cases.

Even a passing development operability result cannot qualify an S0 grader, establish semantic accuracy or waive source-based owner checks. The six disputed reference fields stay excluded and the remaining references are same-author proposals. Exact quotes do not establish semantic relevance or reason/decision coherence. Case repetitions are correlated. This prospective test changes prompts, response structure and fence admission, so comparison with #126 is descriptive; a higher pass count alone does not isolate improved judgment. Original S0 remains 0/108 with four task-history gaps and the 108+conditional24 design intact.

## Validation and owner command

The full tests and vet for `internal/devtools` and `cmd/ganesh-dev` pass, including complete and malformed 180-call mocked batches. Those mocks make no provider requests. The actual private preparation contains 180 frozen jobs, 90 unique native bodies and all 30 development cases, with maximum full prompt bytes + 512 = 11,283. No actual request had been reserved or sent at preparation.

The frozen plan is `a5bb5d1e0a3d13f7ee9a0c7b204d246d7ca8f1e9dbe121eb74abc478dd11dd76`. Its executable, built from source commit `ee08d9f`, is bound by SHA-256 `cf48b6b92ce617efea0b4b89f96a801fc6c3ae127abd3e5d659fb8f42c672dc6`. The local wrapper uses exactly that executable and plan; it does not build or prepare again.

The historical owner command, now consumed, was:

```sh
./.local/s0/grader-rerun-130/run.sh
```

The owner completed this command; its report is saved at `.local/s0/grader-rerun-130/report.json`. Raw responses, sources and keys were not pasted into chat. The exclusive start reservation prevents a second execution.

## Completed run and remaining failures

All **180/180 requests completed**, HTTP 200 with reconciled usage and reported additional cost zero. No retries, replacements, unfinished outputs or truncation occurred. Independent offline accounting verifies **371,900 input + 133,839 output = 505,739 tokens**, including 2,048 reasoning tokens already counted in output. The observed batch-start-to-last-response interval was **23m22.238s**, inside 90 minutes. Provider-reported zero cost is not authenticated billing or remaining quota; separate Codex conversation usage and cumulative focused S0 effort remain unmeasured.

| Grader | Input / output including reasoning | Structure valid / 60 | Exact-evidence valid / 60 | Three global judgments known / 60 |
| --- | ---: | ---: | ---: | ---: |
| DeepSeek V4.1 Flash | 104,846 / 38,066 | 42 | 31 | 23 |
| GLM-5.3-Flash | 100,828 / 39,153 | 0 | 0 | 0 |
| Claude Haiku 5.5 | 166,226 / 56,620 | 45 | 41 | 41 |

The frozen primary result is **72/180 exact-evidence-valid responses**, 64 with all global judgments known. **No profile passes its predeclared development operability rule.** This does not mean 72 judgments are correct. #126's 25/180 strict-valid result used different prompts, response schema and fence admission, so a higher count cannot establish improved accuracy or rank model quality. The earlier 180 grading calls used 349,918 tokens; its 364,800 total also included twelve generator calls. Keep the fresh run's higher usage visible with matched denominators.

### Remaining test defect

The fix introduced an avoidable schema asymmetry: C entries contain `source_quote`, whereas K entries do not. Models added that field to K entries in **80 structurally rejected responses**: DeepSeek 18, GLM 56, Haiku six. Six other Haiku responses mixed K entries into the C array; four GLM responses failed JSON admission, and other criterion/type defects remain. Fifteen responses passed structure but failed exact quotations. Haiku used fences in 59/60 responses, admitted by the prospective policy and reported separately.

A temporary-copy diagnostic, first excluding every frozen-checker invalid-JSON response including duplicate keys, removed only the unrequested K field. **79/80 then passed structure** (18 DeepSeek, 55 GLM, six Haiku); one GLM response had another defect. This identifies a mechanical cause without new calls. It adopts no permissive scoring, repairs no decisions/criteria/quotations, and promotes no primary failure. The first provisional diagnostic was excluded because JSON reserialization could discard duplicate keys; its strict successor records that correction. Original responses and primary results are unchanged. A prospective uniform evidence schema is deferred rather than changing this run's frozen rule after outputs.

### Saved decisions and semantic limits

The separate exploratory analysis reads unchanged K-ID/`violated` values, including K entries in the wrong array, while malformed/duplicate JSON and unknown values stay unresolved. It is not primary admission or independent truth.

| Grader | Planted-critical flags true / 12 planned trials | False / unresolved | Repeat critical/unsupported flag changes / usable pairs |
| --- | ---: | --- | ---: |
| DeepSeek V4.1 Flash | 12 | 0 / 0 | 1/17 |
| GLM-5.3-Flash | 10 | 1 / 1 | 3/15 |
| Claude Haiku 5.5 | 11 | 1 / 0 | 2/28 |

These are six planted controls repeated twice, not twelve independent tasks or an accuracy estimate. Primary evidence-admitted critical detections are only 6/12 for DeepSeek, 0/12 for GLM and 7/12 for Haiku; all other primary outcomes remain unknown. Different usable-pair coverage prevents a stability ranking.

Review of the saved decisions finds semantic failures beyond formatting:

- GLM's migration control, repetition two, marks both K rules false while both explanations explicitly say their rule is violated.
- Haiku's cleanup control, repetition two, treats the unquoted inserted destructive directive as an explicitly rejected quotation, missing the planted violation.
- DeepSeek raises a critical flag on all controls, but marks the migration K2 false in both repetitions. Its explanations discuss chronology/identity while overlooking that the same rule also includes invented deployment completion. A correct global flag does not establish correct per-rule interpretation.

The six known disputed reference fields stay excluded; source/rubric-based assistant review is not independent adjudication. No grader qualifies, original S0 remains 0/108 with four task-history gaps, and required owner/source checks and the 108+conditional24 design remain. All 180 calls are consumed. No further call, automatic resume, schema loop or model switch is selected.

### Verification and custody

The frozen binary's offline report exactly reproduces the saved report. A separate Go reader verifies all native usage, zero reported cost, 180 unique jobs/sessions, identical per-case payloads across graders/repeats, request/response digests and unchanged packet/reference hashes. Private report SHA-256: `631e1a5c969e7e7ff41da5203ffbe409677800fc58f649af1b3a01eea3050e6c`; independent `result-audit.json`: `a86b8a700333f97199fd513309bbedece8e3771dbe1c140102f35d1e8e440b16`; strict `extra-field-diagnostic-v2.json`: `00830c633ad973a9f3bbfa65b6616cf354efd90ddf44c491c613de583120d447`. These bind local custody, not public reproducibility or semantic truth. Raw artifacts remain private/untracked. Post-run review made no model call and changed no runtime code or frozen input; runtime tests were not repeated for this results-only documentation update.
