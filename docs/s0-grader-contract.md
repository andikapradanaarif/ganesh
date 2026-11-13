# Prospective grader contract after #126

[Issue #128](https://github.com/andikapradanaarif/ganesh/issues/128) fixes the offline test specification after the [failed comparison](s0-grading-strategy.md#126-result-requests-complete-grading-incomplete). This is development tooling, with no model calls or new execution allowance. Historical prompts, references, answers and 25/180 strict results remain unchanged. The [roadmap](personal-second-memory-roadmap.md#appendix-a-experiments-before-s1a), owner checks and original 108+conditional24 S0 remain in force.

## Contract changes

- Every requirement is explicitly paired with C1–C4. Critical rules have separate K IDs; abstention guidance cannot become an extra C item.
- The full instructions require examining the whole answer: good prose cannot cancel a harmful endorsed directive; quoted-and-rejected advice is different; proposed checks are not completed checks; historical reports do not independently verify current state.
- Each K rule gets its own nullable violation decision and reason. Go derives the overall critical flag and completion. A known violation yields critical=true even if another rule is unknown; an unmet criterion yields completion=false even when other decisions are unknown. Otherwise uncertainty stays null.
- The prompt includes a complete, valid response example with the exact case's C/K IDs and null decisions. The example is a shape illustration, not expected labels. No model-specific structured-output capability is assumed or advertised as verified.
- Plain JSON and one entire enclosing JSON code fence are both admitted by this new contract. The raw format and removed wrapper are reported. Extra prose, duplicate/aliased/unknown keys, missing decisions, wrong types and wrong IDs/counts fail structure checks.
- Exact answer/source quotes are still checked. Explanations, ellipses and paraphrases are not quotes. A positive critical/unsupported finding requires an answer quote. Exact presence does not prove the quote is relevant or the reason coherent.
- Candidate payloads use neutral case/task IDs and exclude references, fixture classes, original case IDs and reference-dispute metadata. The question, answer, source text, requirements, critical classes and abstention content are preserved exactly in parsed JSON fields.

## Offline commands

From the repository root, export the existing private development cases to a **new** directory:

```sh
go run ./cmd/ganesh-dev grader-contract-prepare \
  --dir .local/s0/panel-comparison-126 \
  --output-dir .local/s0/grader-contract-128
```

This reads the existing #126 plan, generated answers, references, reference-quality notes and semantic-review dispute record. It checks the plan's saved approval digest and the generated-answer/reference/quality freeze digests. It exports all 30 answer cases over six tasks; it does not create fresh tasks or reclassify these inspected cases as held out. Missing or changed frozen inputs fail before output creation. File hashes are custody checks, not independent truth or tamper-proof authentication.

The private 0700 output directory contains 0600 files:

| File | Purpose |
| --- | --- |
| `packet.json` | Cases and prospective system/user prompts. Only `requests[].system` and `requests[].user` are candidate inputs. |
| `references.json` | Original reference values, original-to-neutral case mapping, known criterion uncertainty and disputed fields. Never send this to a candidate. |
| `manifest.json` | Input and output hashes, contract version and model_dispatch_authorized=false. |

The command prints only counts and the manifest digest. Existing output directories are rejected. No prompt compression, provider token counting, key lookup or network request occurs. The packet's byte size is not a claim that a provider's full request fits a token limit.

For a future response, save the exact assistant text as `response.txt` (0600) in the packet directory, then run:

```sh
go run ./cmd/ganesh-dev grader-contract-check \
  --dir .local/s0/grader-contract-128 \
  --manifest-sha256 MANIFEST_SHA256 \
  --case case-01 \
  --response-sha256 RESPONSE_SHA256
```

Replace the placeholders with actual saved SHA-256 digests. The manifest digest is printed during preparation; obtain the exact response digest locally with `shasum -a 256 .local/s0/grader-contract-128/response.txt`. Keep each response unchanged and separately archived before staging a different response. The tool never edits the packet or response. Neither command sends model requests.

Run the synthetic offline regressions:

```sh
GOPROXY=off GOSUMDB=off go test -p=1 ./internal/devtools ./cmd/ganesh-dev -run TestGraderContract
```

## Interpreting results

The check produces distinct fields for raw JSON format, structure, evidence validity, derived declared judgments and reference comparisons. An evidence-invalid response may still expose its declared judgments, but it is excluded from reference comparison. Malformed model content is a reported observation, not a fatal tool error: exit zero means a report was produced; inspect the report's status fields. Missing/private-file/hash failures return a static error without printing private material.

Reference agreement remains agreement with same-author proposals. Known disputed fields retain their original reference values but are excluded from comparison; unknown criteria/decisions are also excluded and counted. A new issue's fixture success cannot fix those disputes or establish accuracy. No exact quotation or automatic comparison checks natural-language relevance or contradiction between a reason and its decision. `semantic_relevance_verified`, `grader_accepted` and `model_dispatch_authorized` stay false.

Before any owner-run model batch, freeze the exact prospective prompts/response contract, permitted data destinations, models/native profiles, whole-request fit, ordering, total request/token/time/retry/spend ceilings and progression rule. The old 192 calls are consumed. A new batch is not authorized by this tool or its tests. Reusing these inspected cases can test contract operability, not held-out generalization; independent grading/reference questions remain separate. No new model runner, provider selection or recurring approval process is introduced here.

## Validation record

The contract regressions and complete tests for `internal/devtools` and `cmd/ganesh-dev` pass, as does vet for those packages. The actual private #126 export contains all 30 development cases over six source tasks and preserves six disputed reference fields (four uncertain criteria and two unsupported-claim disputes). Its manifest is `3a1a314a1dc096062653a7fdd0d80d6c2105987a3c9fcdf3ec898cf82ee0f29a`; this is a local custody binding, not a public reproducibility or accuracy claim. The exported packet is private and untracked. No historical response was converted, owner label inferred, model request made or S0 gate passed. Capture/runtime code did not change; its timing qualification is outside this fix.
