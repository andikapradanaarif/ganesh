# Ganesh working rules

## Issue and PR workflow

The owner requires work to start with an issue and use a pull request.

1. Create or identify a GitHub issue before changing tracked files. State the problem, bounded scope and acceptance criteria.
2. Work on an issue-linked branch, such as `docs/1-roadmap-architecture-review`. Use a separate worktree when isolation is needed.
3. Make the result concrete and reviewable. Open a draft PR as soon as a meaningful initial commit is available; keep it updated as work continues.
4. Link the issue in the PR, describe the resulting behavior and report relevant validation and remaining gaps.
5. Keep implementation changes off `main`. Do not merge without the owner's instruction or existing explicit authorization.

The initial empty commit on `main` only established the base history needed for the first PR.

## Product and stage boundaries

- Start with the supplied S0 execution protocol. The full gateway specification is conditional reference material.
- Automatically captured discoveries and useful within/cross-project reuse are required product outcomes. Typed status or optional owner notes cannot substitute.
- Preserve the conservative narrative eligibility policy and separately budgeted A2.3 decision path.
- Do not invent project, client, model, budget, provenance or test-result identifiers. Unknown evidence stays unknown.
- Keep raw transcripts, secrets, captured payloads, local databases and held-out answers out of version control. Enroll sources only under their explicit processing/disclosure permissions.
- Use Nexus as an architectural reference, not an automatically enrolled source or evaluation corpus.
- Go is selected for the bounded S0 tooling. See `docs/s0-local-tooling.md` for the implementation and evidence limits; later service choices remain conditional on the roadmap gates.

## Engineering guidance

Prefer a small core with explicit transaction boundaries and thin adapters. Domain and application code should not depend on HTTP/MCP framework types or ORM handles. Keep local capture, service acceptance, narrative eligibility and publication distinct.

Run checks appropriate to the change. Documentation-only work needs link/content/diff checks; runtime guarantees require executable evidence before being reported as passed.
