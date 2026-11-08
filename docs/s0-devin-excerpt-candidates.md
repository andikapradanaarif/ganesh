# S0 local Devin excerpt candidates

Task: [#58](https://github.com/andikapradanaarif/ganesh/issues/58), under [S0 #6](https://github.com/andikapradanaarif/ganesh/issues/6). This advances local source preparation after the [Devin source audit](s0-devin-source-audit.md). The [approved execution sheet](s0-execution-sheet.md) still governs calibration and held-out work.

`ganesh-dev devin-candidates` requires an explicitly pinned private source-manifest SHA-256, an unchanged index/manifest/source audit, and the existing four-project grant map. Each file's bytes are checked against its saved hash again before extraction. Only supported `ATIF-v1.7` transcripts with matching session identity are processed. The producer selects the last nonempty recorded agent message with no tool calls. System/user messages, reasoning content, tool arguments/results and other source fields are omitted. This is an excerpt selection rule; it does not infer that the selected message ended a session or accurately summarizes its work.

Before private storage, the message passes the same eight ordered pattern-reduction rules used in [#50's local digest candidates](s0-corpus-calibration-preflight.md): PEM blocks, fenced code, URLs, emails, selected absolute paths, credential assignments, known token forms and generic long tokens. The policy is labelled `bounded-patterns-v1`. The last qualifying message must fit 16 KiB before reduction and 64 KiB after reduction. Unsupported, empty or oversized material gets a content-free exclusion; the producer does not truncate or fall back to an earlier message. Each candidate records the source file hash, zero-based source step index, original selected-message hash, reduced-text hash and redaction counts. All records retain unknown origin/closure and `model_dispatch_authorized=false`.

The local pass accounted for all **86** source-manifest rows:

| Project | Candidates | Missing sources | No qualifying agent message |
| --- | ---: | ---: | ---: |
| Aurum | 0 | 0 | 0 |
| Borneo | 2 | 0 | 0 |
| Valo-backend | 12 | 58 | 1 |
| Kiosk | 13 | 0 | 0 |
| **Total** | **27** | **58** | **1** |

The 28 present source files totalled **11,845,060 bytes**. Selected messages totalled **46,467 bytes**; candidate text totalled **43,724 bytes**. The pass made **55 replacements**: 23 selected absolute paths, 18 URLs, nine generic long tokens, three fenced-code blocks, one email and one known-token form. No candidate text was shown to a model. The output and aggregate report have 0600 modes in git-ignored 0700 storage, and an independent file-hash check matched the reported artifact digest.

The pattern set does not establish general secret safety or semantic fidelity; generic token removal can destroy useful references, and the other omitted messages may contain essential context. These excerpts are unverified local preparation candidates. They do not supply approved scrubbed summaries, source-keyed oracles, a complete D/F corpus or a full-context fit result. Model disclosure is a separate pending decision; exact evaluation models, limits, tasks and rubrics must be frozen before held-out answers. Capture installation remains blocked by [#34](https://github.com/andikapradanaarif/ganesh/issues/34), and prior S0 focused effort remains unknown.

Follow-up [#60](s0-calibration-candidates.md) records the later narrow disclosure decision, six private calibration drafts and an incomplete stop outcome. The counts and no-disclosure statement above describe the original #58 preparation pass.
