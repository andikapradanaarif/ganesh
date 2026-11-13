# Owner-run corrected-contract comparison

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

Pending the offline runner checks and final immutable packet. No fresh inference result is claimed.
