# S0 Codex source-link audit

Task: [#55](https://github.com/andikapradanaarif/ganesh/issues/55), under [S0 #6](https://github.com/andikapradanaarif/ganesh/issues/6). This rechecks the two Kiosk Codex rollout files identified in the private [#50 source preflight](s0-corpus-calibration-preflight.md). It does not enroll other Codex sessions or create a model-ready corpus.

The Go `ganesh-dev codex-audit` reader requires the original private two-row SHA-256 manifest, an explicit Codex sessions root, and the private owner-grant project-root map. It accepts only unique, clean rollout paths within that sessions root, with regular files and no symlinked path components. It re-hashes at most 32 MiB per file and 64 MiB total, and checks that each first `session_meta` line names the exact granted Kiosk working directory. The first metadata line is bounded to 64 KiB. Changed or missing files are reported as drift; unsafe paths, unsupported file types, malformed metadata and unstable reads stop the audit. The original #50 manifest is unchanged.

The local run found **two unchanged files**, **zero changed or missing files**, and **3,393,513 aggregate file bytes**. The private aggregate report is 0600 in git-ignored 0700 storage. No rollout text, private path or session ID was emitted to the model or public documentation. The client store remains mutable; this is a point-in-time source-link check, not a transactional snapshot or provenance proof.

| S0 agent-history source class | Bounded available evidence | Current limitation |
| --- | --- | --- |
| Crush digests | 21 local-only candidate digest rows linked to private raw snapshots in [#50](s0-corpus-calibration-preflight.md). | Pattern reduction is not a general secret-safety or semantic-quality check; other messages are not a prepared corpus. |
| Devin transcripts | 28 present files still match the saved manifest; 58 indexed Valo-backend files remain missing in the [#53 audit](s0-devin-source-audit.md). | Missing files are coverage loss; origin and per-session tool-read classes remain unresolved. |
| Codex rollouts | Two permitted Kiosk files still match the saved manifest in this audit. | Only this selected slice is checked; other rollouts are excluded, not treated as empty or clean. |

These are different source classes and cannot be added into one complete-corpus count. Baseline project notes, generated typed cards, source-keyed calibration oracles and a scrubbed full-context D/F input are still absent or unresolved. The recorded grants allow the bounded local checks but do not authorize model disclosure or held-out answers. No P01/P02 or later-stage gate follows from these checks; installed capture remains blocked by [#34](https://github.com/andikapradanaarif/ganesh/issues/34).
