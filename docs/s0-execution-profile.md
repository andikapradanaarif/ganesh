# S0 execution-profile evidence under #124

**DeepSeek V4.1 Flash through OpenCode Go is already the owner-selected candidate grader for OpenAI-family answers.** [#83 / merged #84](https://github.com/andikapradanaarif/ganesh/pull/84) explicitly records the run-specific exception in the [Go amendment](s0-opencode-go-grader-amendment.md). Recent #124 handoff/status text omitted that decision and incorrectly said another family amendment was required. This corrects that interpretation; the original roadmap/execution sheet remain preserved. Candidate selection still does not establish grading acceptance, processing permission or a call allowance.

Work remains in [existing #124](https://github.com/andikapradanaarif/ganesh/issues/124). Original S0 is **0/108**, with the conditional 24 retained. The [bound run packet](s0-source-backed-task-revision.md#bound-run-packet-proposal-after-pr-135) is unchanged; a separate private profile overlay restores the selection instead of rewriting its frozen null-grader record or hashes.

## Current role proposal

| Role | Recorded choice / proposal | What remains unverified |
| --- | --- | --- |
| Answering | Owner selected OpenAI-family answers. Propose `gpt-6-luna` through Go's native Responses route; installed Crush is a client candidate. | Exact answering profile is not owner-frozen. Client request/tool/accounting controls, future account access, complete request fit and immutable model identity are unqualified. |
| Grading | Owner-selected `deepseek-v4.1-flash`, configuration ID `opencode-go/deepseek-v4.1-flash`, through Go's native Chat Completions route. | Natural-task grading acceptance remains incomplete. The latest seen synthetic controls match 72/72; this is not natural accuracy or a fresh execution grant. |

Checked on **2026-11-15 UTC**: the [official Go catalog](https://opencode.ai/docs/console/go/#endpoints) lists both request IDs and native routes. Its [privacy declarations](https://opencode.ai/docs/console/go/#privacy) state no training use, up to 30-day retention for GPT 6 Luna, and zero-day DeepSeek retention under the agreement through November 30. DeepSeek needs Global-region permission in workspace settings. These are published declarations; the owner's current account settings, access and remaining allowance were not inspected. Additional paid spend stays $0; Extra Usage's earlier owner-declared OFF state is not current verification or a renewed allowance.

## Installed answering-client check

The observed Crush binary reports module version `v0.101.1-0.20261113101334-86897c56a304`, revision `86897c56a304bac133650a6055ab1d091e6b8f83`, darwin/amd64 and unmodified VCS state. Its SHA-256 is `72ca21a626f852f4a89d491cb8ea1ca66613cd9bbf826a768b584b80947b7798`. This identifies an answering-client candidate; the old day-one capture snapshot and #34 remain unchanged.

Pinned client source exposes eval step/token stop controls, but its timeout test explicitly expects retries. An offline test of the installed binary confirmed repeated attempts in a different, synthetic 503 case:

- One synthetic prompt and one eval step, isolated config/data, explicitly listed fake model/provider, dummy key and no provider discovery or metrics.
- An OS sandbox denied network access except loopback; every fake-server reply was HTTP 503. Request bodies and authorization values were not logged, only hashes/counts and dummy-auth checks.
- **Four loopback POSTs to `/v1/responses`: two distinct payloads, each repeated once.** The fixture stopped early after observing repeats; this is not a measurement of maximum retries or a claim about the roles of the two payloads.
- All four requests carried the supplied fixture session header and dummy authentication. This checks that supplied configuration path, not native automatic session handling across every possible client path.
- **Zero real inference requests.** No real answer, provider usage, account availability or success-path accounting was measured.

**Result: the tested client profile does not satisfy the proposed zero-retry contract.** A one-step cap cannot be treated as a one-provider-request cap. The old numerical limits are unapproved proposals; this test neither relaxes them nor proves that every possible Crush configuration fails. Actual baseline/tool isolation, input/output/auxiliary accounting and full request fit remain unqualified. The fixture's filesystem permissions and loopback protection do not establish a production tool sandbox.

## Concrete next requirement

Provide and verify an answering-client profile that enforces the final request/retry/output limits and accounts for auxiliary and failed attempts. A supported configuration, minimal client fix or a different explicitly bounded proposal can be evaluated in #124; no installed binary, external client repository or budget was changed here. Do not send the main S0 packet through the tested configuration while describing it as no-retry.

Then complete source-keyed grading acceptance for the already selected DeepSeek candidate, actual client/tool/processor permissions, exact profiles and context fit, and one whole-run allowance. Preserve the required owner checks and stopped leading-confirmation flow. Another model-family selection question or Anthropic-only requirement would repeat the corrected mistake. No model run is queued.

## Evidence and limits

Private fixture report SHA-256: `59ecfffd23b64ac86bfa364ff726f2b0de96b6cfa999369c8f3e3e6ea1c8389d`.

Private profile overlay SHA-256: `193d9548dbc55865379ce893306e63cfb278296f007251495ebecaf376808fc2`.

Private check report SHA-256: `ca83f4e6972fcc376b7bf0fa0bb82a47ed334653a524abb6e94d7512b5f48435`.

The Go checker verifies the existing owner decision, unchanged bound packet, binary/report joins, four 503 records forming two repeat pairs, dummy-auth/session checks, private modes and disabled dispatch. The initial sandbox-policy syntax preflight failed before client launch; its source/report are preserved separately, and the corrected sandbox was required before the actual fixture. The old sources, labels, model results, manifests and baseline documents remain unchanged. No provider key/account inspection, new project-source enrollment, target application execution, capture study or installation occurred. Private reports/configuration/logs stay outside git. Fixture results and source review do not establish model accuracy, client success-path qualification or a stage pass; Codex conversation usage and overall focused S0 effort remain unmeasured.
