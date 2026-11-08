# S0 Devin source-link audit

Task: [#53](https://github.com/andikapradanaarif/ganesh/issues/53), under [S0 #6](https://github.com/andikapradanaarif/ganesh/issues/6). This checks the private point-in-time Devin availability manifest prepared in [#50](s0-corpus-calibration-preflight.md). It does not enroll transcripts in a model corpus or change the approved [execution sheet](s0-execution-sheet.md).

The Go `ganesh-dev devin-audit` reader compares the 86-row private session index with the 86-row manifest and an explicitly supplied transcript directory. A separate private project-root map was derived from the two recorded owner grants for Aurum, Borneo, Valo-backend and Kiosk; Nexus was excluded. The reader checks exact index/manifest IDs and working directories, that each project path stays within its declared root, and that each transcript path is the expected `<root>/<session-id>.json`. It re-hashes each present regular file with a 32 MiB per-file and 128 MiB total cap, and reports missing files separately. It rejects final-component symlinks and unstable or unsupported files. Only aggregate states, file bytes, drift counts and evidence digests go to stdout; the original manifest is never rewritten.

| Project | Original present | Current present | Original missing | Current missing |
| --- | ---: | ---: | ---: | ---: |
| Aurum | 0 | 0 | 0 | 0 |
| Borneo | 2 | 2 | 0 | 0 |
| Valo-backend | 13 | 13 | 58 | 58 |
| Kiosk | 13 | 13 | 0 | 0 |
| **Total** | **28** | **28** | **58** | **58** |

The audit found **no drift** from the saved #50 manifest: all 28 present files still matched their recorded sizes and SHA-256 values, totalling **11,845,060 file bytes** in both snapshots. All 58 missing entries remained missing at this check. Missing entries are coverage loss, not empty or clean sessions. The private aggregate report and grant map are 0600 under a git-ignored 0700 directory. The map is a consistency check against recorded owner declarations, not independent proof of source authorship or permission for model disclosure. Ancestor paths and the same-user host remain trusted; the file checks are point-in-time and do not make mutable client stores immutable.

This result does not establish transcript provenance, complete capture, scrub safety, semantic calibration tasks, D/F full-context fit, P01/P02 acceptance or model-run authorization. The original #50 manifest stays historical evidence. Any corpus freeze must record a new source revision and treat unavailable material as coverage loss. Installed capture remains blocked by [#34](https://github.com/andikapradanaarif/ganesh/issues/34); prior S0 focused effort remains unknown.
