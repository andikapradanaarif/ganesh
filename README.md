# Ganesh

A planned, server-hosted personal memory service for agents working across authorized projects, accessible through REST and MCP. Go is the selected implementation language.

The approved direction and execution protocol are:

1. [Personal Second-Memory Service Roadmap](docs/personal-second-memory-roadmap.md) — product requirements, stages, decision gates and conditional engineering reference.
2. [S0 execution sheet](docs/s0-execution-sheet.md) — preparation, frozen experiment rules and reporting requirements.

Read roadmap sections 0–7 first, then the S0 execution sheet. Start with S0; the fuller gateway specification remains conditional.

Both documents preserve the owner's supplied text. Their planned/not-run status is retained. No product experiment or release acceptance is claimed complete by this import.

Project selection, the first capture installation, exact model IDs and budgets are filled in when their stage requires them. They do not block roadmap planning; required run inputs must be frozen before held-out answers.

Work is organized in [roadmap tracker #5](https://github.com/andikapradanaarif/ganesh/issues/5), starting with [S0 issue #6](https://github.com/andikapradanaarif/ganesh/issues/6). Each task uses an issue, milestone, branch and PR. See [AGENTS.md](AGENTS.md).

The first implementation task provides [S0 run preparation](docs/s0-preparation.md): a Go CLI for local client discovery and private run records. The [permitted day-one inventory and origin screen](docs/s0-day-one-inventory.md) now records bounded aggregate results. Capture integration remains pending.

[S0 corpus and calibration preparation](docs/s0-corpus-calibration-preflight.md) records the source-permission boundaries and a private source-availability pass. Prior focused effort remains unknown; no scrubbed corpus or model run is claimed.

The [Devin source-link audit](docs/s0-devin-source-audit.md) rechecks the saved availability manifest against the current local files without disclosing transcript text.

The [Codex source-link audit](docs/s0-codex-source-audit.md) similarly rechecks the two permitted Kiosk rollouts and summarizes the still-incomplete source-class coverage.

[Local Devin excerpt preparation](docs/s0-devin-excerpt-candidates.md) now produces private source-linked candidates for later calibration review; they remain unverified and local only.

[Calibration candidate preparation](docs/s0-calibration-candidates.md) records six private draft tasks and the incomplete outcome after the planned preparation pass was exceeded.

[Offline calibration comparison](docs/s0-calibration-comparison.md) compares saved owner and grader judgments without model calls; missing labels remain incomplete.

[Issue-state verifier probe](docs/s0-issue-state-probe.md) tests exact enum support against frozen synthetic expectations without inferring resolution or deployment readiness.

[Repository-head verifier probe](docs/s0-repository-head-probe.md) separates protected-branch resolution from commit existence, tags and deployment claims using fixed synthetic inputs.

[CI-result verifier probe](docs/s0-ci-result-probe.md) keeps old-commit run results separate from current-head-qualified results, with exact workflow/run/attempt and head dependencies.

[Decision-authority verifier probe](docs/s0-decision-authority-probe.md) tests why authenticated PR prose cannot become an authoritative decision, using a fake designated registry as its positive control.

[Agent-echo origin probe](docs/s0-echo-origin-probe.md) traces shared declared origins through tools and agents without counting repeated reports or source revisions as independent corroboration.

[Initial origin accounting](docs/s0-origin-screen.md) freezes the handoff-need denominator and reports the A1 route from screener declarations. It does not inspect sources or verify provenance.

[Evaluation planning](docs/s0-evaluation-plan.md) freezes the standard S0 task/diagnostic split and counterbalanced 108-answer schedule, with 24 conditional eligible-origin slots. Real inputs and execution/acceptance remain pending.

[Declared-result reporting](docs/s0-result-reporting.md) accounts for paired outcomes, usage, stratum regressions and the D/F rerun condition from private grading metadata. It neither grades answers nor accepts an experiment.

[Final eligibility accounting](docs/s0-eligibility-recheck.md) pins the initial assessment and six follow-on task definitions, then reports the final first-party thresholds and A2.3 route from declarations. Actual evidence and stage authorization remain separate.

The [local capture primitive](docs/s0-local-capture.md) accepts bounded normalized metadata on stdin and returns durable local receipts. A prototype [Crush hook adapter](docs/s0-crush-hooks.md) reduces native tool, permission, turn-ending and compaction notifications without emitting agent context. Installed-client coverage, hook enrollment, upload and server intake remain pending.

`make build` produces a smaller `bin/ganesh` artifact with Go's `-s -w` linker flags. The [startup investigation](docs/s0-capture-startup.md) records a bounded reduction in observed failures and the remaining cold-launch loss. This build does not qualify an installed capture profile or extend the 750 ms collector budget.

The [standalone collector experiment](docs/s0-standalone-capture.md) extracts shared capture CLI code and provides `make build-collector`. Its dedicated Go binary excludes preparation/discovery, but it did not pass the frozen timing or headroom gates. It remains experimental; `make build` continues to produce the general CLI and no client configuration changes automatically.

Current validation tooling uses Go; see [Go developer tools](docs/go-developer-tools.md). Completed Python experiments remain historical evidence at their original paths.
