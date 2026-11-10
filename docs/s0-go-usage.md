# Offline OpenCode Go usage reconciliation

[Issue #87](https://github.com/andikapradanaarif/ganesh/issues/87) moves the accounting checks from private calibration scripts into reusable Go tooling. Earlier saved responses were rejected because a redundant standard reasoning count was required, or cached prompt tokens were treated as uncached input. The original stopped runs and their recovery records remain preserved under [#60](https://github.com/andikapradanaarif/ganesh/issues/60).

This tool handles one native Chat Completions `usage` object. It does not read a grading answer, SSE stream, whole completion response, reference label or credential. `ParseGoUsage` is the pure parser; `CheckGoUsage` adds private-file and caller-digest checks. Neither performs network requests or grants a new model allowance. Future runner integration still requires separate binding, stream/model/answer validation and a frozen approved protocol.

## Invocation and private boundary

```sh
go run ./cmd/ganesh-dev go-usage-check \
  --dir .local/s0/saved-usage \
  --usage-sha256 "$S0_GO_USAGE_SHA256"
```

Save the native usage object itself as `go-usage.json`, without a surrounding `usage` or response wrapper. The caller supplies its 64-character lowercase SHA-256 from the preserved run record. The command checks exact bytes; it does not discover or authenticate the expected digest.

The directory must be 0700 and the file regular 0600, capped at 32 KiB. Final-component directory/file symlinks are rejected. Ancestor paths and the same-user host remain trusted. Keep the closed snapshot outside git; this reader does not run git to establish exclusion or prevent deliberate rewriting of both the file and supplied digest. Original response/model identity and the relationship between this object and its run remain separate evidence.

Malformed JSON, invalid UTF-8, duplicate keys (including escaped spellings), more than 64 JSON nesting levels, unsafe files or a mismatched digest return the static error `private bounded zero-cost Go usage required`, exit 1 and no JSON report. Case aliases of recognized accounting keys fail at every interpreted level. Unrelated provider metadata is bounded and parsed as JSON but ignored: it does not become accounting evidence or appear in output. This permits harmless provider extensions without pretending to verify the whole API schema.

## Accounting profile

The [official OpenCode API documentation](https://opencode.ai/docs/console/api/inference/) separates native token usage from billable components. Go's input counter excludes cache reads/writes, while its output includes reasoning. Its allowance field describes plan consumption, separately from the reported additional balance charge. Those distinctions motivate this parser; a metadata assertion is not authenticated billing evidence.

This implementation pins a narrow calibration profile:

| Field | Required rule |
|---|---|
| `prompt_tokens` | Explicit positive integer |
| `completion_tokens` | Explicit integer from 1 through 2,048 |
| `total_tokens` | Exactly prompt plus output, checked without overflowing addition |
| `cost` | Explicit JSON number with mathematically zero value |
| `opencode.billing_source` | Exactly `go`; other plans/sources require a deliberate profile change |
| `opencode.allowance` | Explicit nonnegative numeric value; Float64 conversion must not error or produce infinity; original number text retained |
| `opencode.tokens.input` | Explicit nonnegative integer, at most native prompt input |
| `opencode.tokens.output` | Explicit integer equal to native output |
| `opencode.tokens.reasoning` | Explicit integer zero; missing/null is not zero |
| `completion_tokens_details.reasoning_tokens` | Optional; if supplied/non-null, integer zero |
| `prompt_tokens_details.cached_tokens` | Optional; if supplied/non-null, must agree with the Go cache-read count |
| Go `cache_read`, `cache_write_5m`, `cache_write_1h` | Optional nonnegative integer counters; supplied buckets must explain the prompt/noncached-input difference exactly |

The optional cache buckets are inside `opencode.tokens`. Missing or null optional counters remain null in the report. They are never presented as provider-reported zeroes. A positive difference without enough explicit cache evidence fails. All-cached input with explicit zero uncached input is valid. When a standard cache-read count is supplied but the Go read count is absent, only zero is consistent with a fully reconciled input; a positive standard count cannot substitute for missing Go accounting.

Numeric strings, booleans and fractional/exponent tokens for integer counters fail. Cost and allowance may use valid decimal/exponent number representations. Exact zero-cost validation examines the number's mantissa, so `1e-9999` fails even if floating-point conversion would underflow to zero; signed zero remains zero. Negative allowance values fail independently of float rounding. An allowance that overflows the conversion check fails; a tiny positive value retains its original text even if this check underflows. Accepted allowance text is returned unchanged as a string, avoiding decimal rounding in output. This is not a pricing calculator or a remaining-quota estimate.

Prompt reconciliation subtracts noncached input and each supplied cache bucket from the native prompt bound. Negative, excess, unexplained and standard/Go-conflicting counts fail without summation overflow. Zero reasoning makes native/Go output equality appropriate for this specific profile. A general reasoning-enabled, other-provider or larger-output reader would need different rules; this command does not silently widen them.

## Output and authority

Success returns `status: reconciled_zero_cost_go`, integer counts, nullable optional counters, billing source, exact numeric cost/allowance text and the input digest. Unknown provider metadata, source paths, identifiers and arbitrary strings are not echoed. Even the numeric billing metadata and hashes may be private. Output-writer failures normalize to the same static error; report bytes already written cannot be retracted.

Every successful report keeps `model_identity_verified`, `grading_validated`, `calibration_accepted` and `model_dispatch_authorized` false. Correct accounting alone does not establish valid stream framing, absence of reasoning text or tools, a model's identity, the answer/evidence schema, criterion relevance, compliance with a dispatch budget or safe provider disclosure. The separate [structured evidence checker](s0-grader-evidence.md) addresses mechanical answer evidence and also leaves semantic relevance unverified. Neither accepts a grader or passes S0.

## Saved-record evidence

A read-only private extraction/audit rechecked 23 retained usage objects: the original pilot, eight attempts from the stopped first batch, nine from its separately approved continuation, and five from the stopped repair check. All 23 reconciled, including one absent optional standard reasoning counter and one cached-input case. Original response bytes were compared before/after and were unchanged; no original producer, failure flag, label or grading outcome was rewritten.

Observed totals over these saved objects are 33,096 native prompt tokens = 32,584 uncached + 512 cache-read tokens, 5,204 output tokens and 38,300 total tokens. All 23 explicitly report zero reasoning and zero additional cost. Provider-reported allowance sums exactly to 0.00503300 in its reported units. These are saved declarations, not an authenticated ledger, a quota/reset forecast or proof that no other account use occurred. The audit generated zero provider calls and created only private usage snapshots/reports outside git.

Synthetic executable tests cover both earlier rejection classes, cache reads/writes/all-cached input, absent optional counters, exact zero versus tiny nonzero cost, contradictory/negative/type-confused counters, overflow, aliases/duplicates, unrelated-metadata privacy, private-file boundaries and command/output failures. The existing semantic calibration failures remain unresolved; no new model run follows from this implementation.
