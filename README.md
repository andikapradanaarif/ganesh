# Ganesh

Ganesh is a planned personal memory service for agents working across authorized projects. Go, a server-hosted core, REST and MCP are the approved direction.

**Current stage: S0, incomplete. Recorded held-out answers: 0/108.** Capture components exist, but installed capture profiles remain disabled. The synthetic judgment-contract comparison is complete; no grader is accepted for natural-task S0 scoring. No usable service or memory-value result is claimed.

The owner has reaffirmed the original **30-task, 108-answer S0**, with all required arms and controls. [Task rework inside #124](docs/s0-source-backed-task-revision.md) supplies source-backed replacement drafts for the four unsupported designs and two overlap corrections. The other 24 tasks are retained. This is a definition revision, not a test score or accepted run; task admission, full run-input binding and grading remain incomplete. Further worker/IPC development and grader-format iterations are off the critical path; no smaller pilot is selected.

Start here:

- [Current S0 status and decisions](docs/s0-current-status.md): filled companion to the execution sheet, actual gaps and next work.
- [Approved roadmap](docs/personal-second-memory-roadmap.md) and [execution sheet](docs/s0-execution-sheet.md): preserved product direction, controls and stage gates.
- [Go developer tools](docs/go-developer-tools.md): current local commands.
- [Development record](DEVELOPMENT.md): detailed implementation history and links to individual studies/components.
- [Working rules](AGENTS.md): issue-linked branches, review and owner-directed merge.

Build with `go build -p=1 ./...`; run correctness checks with `go vet ./...` and `go test -p=1 ./...`. The compiled capture CLI test can reject startup at its unchanged 750 ms deadline on affected hosts; retain that failure under [#34](https://github.com/andikapradanaarif/ganesh/issues/34), rather than treating a warm retry as qualification. No GitHub Actions workflow is configured yet.

Current work is tracked in [S0 #6](https://github.com/andikapradanaarif/ganesh/issues/6), [readiness #124](https://github.com/andikapradanaarif/ganesh/issues/124) and [capture #34](https://github.com/andikapradanaarif/ganesh/issues/34). Historical preparation issues are closed; no new issue is needed for remaining readiness. No further model calls or timing studies are currently authorized.
