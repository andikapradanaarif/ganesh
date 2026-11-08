# S0 corpus and calibration preflight

Task: [#50](https://github.com/andikapradanaarif/ganesh/issues/50), under [S0 #6](https://github.com/andikapradanaarif/ganesh/issues/6). This is a preparation status report, not a frozen corpus, calibration set or S0 result. The approved [roadmap](personal-second-memory-roadmap.md#appendix-a-experiments-before-s1a) and [execution sheet](s0-execution-sheet.md) remain authoritative.

## Recorded inputs and boundaries

The two private owner grants from the [day-one inventory](s0-day-one-inventory.md) match all five selected project paths in the private S0 run record. Both grant local read and transcript processing/disclosure for agent history in the proposed Codex, Crush and Devin clients. This is a consistency check of recorded declarations, not independent proof of source provenance or permission for model dispatch.

| Source | Current #50 disposition |
| --- | --- |
| Nexus | Architectural reference; excluded from corpus enrollment under [AGENTS.md](../AGENTS.md). Its day-one screening grant does not change that boundary. |
| Aurum, Borneo, Valo-backend, Kiosk agent history | Candidate classes under the recorded transcript grants. The owner directed continuation with prior S0 effort unknown; local source availability was checked, but no scrubbed corpus is ready. |
| Repository files and notes | Local-read grants do not by themselves enroll their content as a processed or disclosed corpus. |
| Generated typed cards, scrubbed summaries and eligible-origin subset | No artifacts prepared. They remain distinct inputs under the execution sheet; unknown or mixed origin cannot be promoted to first-party. |

Any candidate processing is limited to a private local workspace until its source class and disclosure destination are confirmed. No source is enrolled by repository ownership, project selection or the #48 screen alone. Capture enrollment remains blocked by [#34](https://github.com/andikapradanaarif/ganesh/issues/34).

## Effort gate

The execution sheet allocates **30–56 focused hours to S0**, including **8–12 hours for sources and calibration**. Completed capture studies record their own ceilings, and the day-one screen records completion within one hour. Those records do not establish cumulative focused hours or the remaining allowance. The existing evidence reports this total as unknown. On 2026-11-08 the owner directed continuation with prior effort unknown. Issue #50 now permits bounded local preparation with prospective effort tracking; overall S0 cap compliance remains **incomplete**, and no hours or budget are inferred from commit timestamps or experiment ceilings.

The private #50 preflight and source records are stored with 0700/0600 modes under git-ignored `.local/`. The initial preflight read no corpus source content. The later availability pass hashed selected transcript files and exported selected client digest rows into private raw snapshots. No scrubbed summary/card/calibration artifact was generated, and no model call was made.

## Local source availability pass

The private Devin session index returned **86** sessions mapped to the exact Aurum, Borneo, Valo-backend and Kiosk roots or their subdirectories. Three sessions under Valo's parent directory were excluded because that parent is not the selected Valo-backend project. All selected session IDs were distinct. The private Go producer found **28** matching regular transcript files and recorded their size and SHA-256; **58** Valo-backend index entries had no matching transcript file. Every manifest row remains `unknown_unreconciled` for origin. Missing files are coverage loss, not empty or clean sessions.

| Project | Devin index rows | Present transcript files | Missing files |
| --- | ---: | ---: | ---: |
| Aurum | 0 | 0 | 0 |
| Borneo | 2 | 2 | 0 |
| Valo-backend | 71 | 13 | 58 |
| Kiosk | 13 | 13 | 0 |

Read-only Crush metadata queries found **19** Valo-backend and **5** Kiosk sessions, with **16** and **5** existing session-digest rows. Those digest rows were exported only to private raw snapshots. A private Go manifest records per-field sizes and hashes for all 21 rows, with independent hash checks on one checkpoint from each store. Two Codex rollout files matched Kiosk's exact working directory and were hashed privately. The client stores are mutable; these are point-in-time availability observations, not an installed capture profile, a complete source corpus or a full-context fit check.

A conservative private Go reduction pass produced **21 local-only candidate records** from the Crush digest rows. Synthetic fixtures for planted sensitive forms passed, and a local scan found no remaining matches within the reducer's declared URL, email, path, known-token and long-token patterns. The pass made **155 generic long-token replacements**; that rule can remove valid commit IDs and useful source references. Every candidate remains `unknown_unreconciled`, `candidate_unverified_local_only` and `model_dispatch_authorized=false`. Pattern checks do not establish general secret-scrubbing coverage, safe disclosure, semantic usefulness or a model-ready corpus. No candidate text was sent to a model.

## Follow-up local audit

The issue-linked Go command `ganesh-dev corpus-audit` independently rechecks private Crush raw snapshots against the field hashes in their manifest, then checks each candidate against the exact declared reduction and local-only status. It requires 0700/0600 private inputs, rejects duplicate or missing rows, and emits aggregate JSON without source text. The command is intentionally specific to the current digest snapshot format; it does not process Devin or Codex transcripts. Run it against private files with `go run ./cmd/ganesh-dev corpus-audit --dir <private-directory> --raw <raw-basename-1>,<raw-basename-2>`.

The follow-up audit linked all **21 raw, manifest and candidate rows**: five Kiosk and 16 Valo-backend. Source-field hashes and candidate reductions matched, and the declared patterns were absent from candidate fields. The raw digest fields totalled **11,232 bytes**; the candidate fields totalled **7,892 bytes**. All **155** replacements were from the generic long-token rule. These checks establish reproducibility and coverage only for this narrow reducer. They cannot detect secrets outside its patterns, establish semantic fidelity after redaction, resolve origin, or grant model disclosure. The candidates remain local only.

No calibration task or source-keyed oracle has been finalized from these candidates. Meaningful task wording and oracle answers require source-content review under an explicit processing/disclosure boundary; metadata-only questions would not calibrate memory usefulness. The current 7,892-byte candidate slice is also not the complete authorized corpus: the 58 missing Devin files, other unprocessed available transcripts, baseline notes and any typed cards remain unresolved. **D/F full-context fit is unknown** until the complete frozen corpus and exact answering model/context limits are recorded; no F input may be silently shortened.

## Next evidence

1. Track effort prospectively and retain prior S0 effort as unknown. Any overall cap claim remains incomplete until the earlier work is reconciled.
2. Review the bounded reduction's residual leakage and source-link loss before using digest or transcript text as model input. Preserve private raw snapshots, source links and unknown/mixed origin; do not treat file presence as provenance.
3. Prepare private scrubbed candidate summaries, source-keyed oracle references and separate calibration candidates under confirmed permissions. Record digests, exclusions, coverage loss and full-context diagnostic fit.
4. Freeze held-out corpus/tasks/rubrics, permissions, models, limits and order before answers. This preparation grants no P01/P02 pass, first-party eligibility, A2.3 funding, production narrative or later-stage build.
