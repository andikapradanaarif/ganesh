# Ganesh working rules

## Read first

- `docs/personal-second-memory-roadmap.md` is the approved product direction, stage gates and conditional engineering reference.
- `docs/s0-execution-sheet.md` is the S0 execution protocol.
- These are the owner's supplied documents, preserved as the baseline. Discuss questions or proposed changes with the owner; do not reopen product review or silently change their requirements.

Go and a server-hosted service with REST and MCP over one core are agreed. Follow the documented distinction between local capture, the lite MCP bridge and conditional hosted remote MCP work.

## Issue, milestone and PR workflow

1. Identify or create an issue before changing tracked files. State its bounded scope, dependencies and acceptance criteria, and assign the appropriate milestone.
2. Work on an issue-linked branch. Use a separate worktree when isolation is useful. Keep changes off `main`.
3. Open a draft PR once a meaningful initial commit is available. Link the issue and report what changed, validation and remaining gaps.
4. Complete the issue's authorized scope and report milestone evidence. Apply the roadmap's next-stage gates; issue creation is not evidence that those gates passed.
5. Do not merge without the owner's instruction or existing explicit authorization.

The roadmap tracker is issue #5; S0 is issue #6. Earlier closed review and tooling attempts are historical artifacts, not active prerequisites or acceptance evidence.

## Stage and evidence boundaries

- Begin with S0. Part II does not authorize an immediate gateway build.
- Select permitted projects and the first capture client during S0 day one. Exact client/model IDs, permissions and budgets are stage inputs, not prerequisites for importing documents or planning issues. Freeze all required run inputs before held-out answers.
- Do not invent identifiers, provenance, budgets, measurements or outcomes. Unknown evidence stays unknown; fixture success does not establish production integration.
- Preserve the automatic discovery-to-reuse and unattended-upkeep requirements. Typed status and optional owner notes cannot substitute.
- Preserve the conservative narrative gate and separately budgeted A2.3 routing. No production narrative activation follows from planning or an isolated experiment.
- Keep raw transcripts, secrets, captured payloads, local databases and held-out answers outside version control. Enroll sources only under explicit processing/disclosure permissions. Nexus is an architectural reference, not an enrolled corpus.

## Engineering

Prefer a small Go core with explicit transaction boundaries and thin adapters. Keep domain/application code independent of transport framework and ORM types. Distinguish local capture, durable service acceptance, narrative eligibility and publication.

Run checks appropriate to the change. Document imports require source-fidelity and link checks; runtime guarantees require executable evidence.
