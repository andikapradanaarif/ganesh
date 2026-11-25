# Ganesh roadmap

Ganesh gives the owner's agents a shared view of work across laptops, servers and authorized projects. A fresh agent should be able to answer two questions from a short, source-linked brief: **“Where did we leave this work?”** and **“How did we handle a similar problem elsewhere?”** The owner chooses which installations and projects may contribute and receive information. Ordinary handoffs should require neither a new summary from the owner nor credentials for another project's repository.

**First product demonstration.** An agent on a laptop leaves a checkpoint in Project A. A fresh agent on a server, working on A, retrieves the last verified action, blocker and next step. Later, while working on Project B, an agent receives one permitted lesson from A, including its applicability limits. The source repository stays private. When A corrects the lesson or the owner revokes sharing, the next server read reflects the change.

**Current stage.** Planned. First measure whether Ganesh returns sufficient, current, authorized evidence through its normal read path. The earlier S0 evaluation ended incomplete at 0/108 formal held-out answers. Its protocol, research and conditional engineering designs are preserved in the [historical roadmap](https://github.com/andikapradanaarif/ganesh/blob/2fddc9d2abfe288e85b9f5f5dceedc20d89ef27f/docs/personal-second-memory-roadmap.md).

## 1. The information an agent needs

| Question | Useful response | Evidence boundary |
| --- | --- | --- |
| Where did we leave this work? | Objective, completed work, last verified action, blocker, next step, producing installation and observation time. Omit unknown fields. | A local test result establishes what ran on that workspace and revision. A proposed next step remains a proposal. A local commit alone cannot establish deployment. |
| What can I reuse from another project? | A self-contained lesson explaining the problem, what was tried, what happened, where it applies and what the receiving agent should check locally. | A standing sharing rule covers the specific published material, its audience and processor. Keep agent-reported reasoning labelled. Preserve the distinction between a reported outcome and an independently checked fact. |

Capture a checkpoint locally when work reaches a meaningful boundary. Give it a stable event identity so the installation can retry delivery. Distinguish captured locally, accepted by the service and available to the reader. If two installations report incompatible work in progress, keep their source and revision visible. A late upload from an offline laptop must not silently replace a newer verified checkpoint.

## 2. One evidence contract for REST and MCP

Use a small server-hosted Go core with shared retrieval, revision and access logic. REST clients call it directly. Initially, an MCP-capable agent can use a thin local MCP bridge to the same REST service. Add a hosted remote MCP adapter when a selected client needs it. Local capture is a separate adapter at the source installation.

A read supplies the authenticated installation identity, active project, question and optional effective time. The service derives the permitted audience from the credential and policy. Choosing a different project in a request cannot expand access.

Return ranked evidence passages. Each carries a stable item ID, revision ID, scope, provenance label, applicable time when known, and a citation the recipient is allowed to see. Return a short checkpoint or a permitted lesson where it suffices. Project B may see a safe published citation and lesson text while only A can resolve the underlying private evidence. Use one access decision for passages, citations and any derived brief. Exclude restricted information before it can reach a consumer or an unapproved processor.

Return an explicit **insufficient context** result when the permitted evidence cannot support the requested fact. Return **conflicting evidence** with the permitted alternatives when neither supersedes the other. An empty response for B must not reveal the existence, title or path of a hidden source in A.

## 3. How current and historical reads work

- **Identity and time.** Give each fact, decision or lesson a stable identity. Record immutable revision identities, source anchors, effective dates when supplied, and observation and ingestion times. Default reads ask what applies now. An as-of read asks what applied on the requested date according to evidence currently available and currently permitted. Unknown effective dates stay unknown.
- **Corrections.** Add a revision linked to the one it supersedes. Publish it and retire the old current view together. Supersession follows evidence, authority and scope. The last item downloaded does not automatically win.
- **Partial corrections.** Suppose the earlier rule required manual approval for staging and production. A correction makes staging automatic from June 1 and leaves production manual. A current staging query returns the new rule. A production query still returns manual approval. A May staging query can return the earlier rule with its historical status. Importing the May document again in July changes none of those answers.
- **Related discoveries.** Give a lesson about *why* a rollout failed its own identity and link it to the deployment rule. Similar words or a later timestamp alone do not establish replacement. If an anchor changes, re-evaluate its dependants and withdraw unsupported guidance from current reads.
- **Revocation and deletion.** If A withdraws B's permission, B's next current or historical read must exclude the affected passage and citation, including cached or derived results. Keep independent permitted material available. Deletion must also cover derived copies under the retention policy. The service can control future delivery; a recipient may already have read an earlier response.

## 4. How we will test retrieval

**Build a reproducible corpus.** Choose sources permitted for storage and testing. Use a recorded sampling frame and seed for the public articles, and deliberately include near neighbours on the same topic. Freeze the bytes, source identifiers, dates, revisions, permissions and digests. Record genuine corrections where available. Mark synthetic change sequences as fixtures. Tests should replay offline when a live article changes or disappears.

**Write questions independently of ranking.** For each question, specify what would be sufficient evidence, acceptable equivalent passages, revision, effective time, reader scope and expected outcome. Some questions require multiple facts. Include paraphrases, similar but irrelevant passages, partial corrections, late arrivals, historical reads, missing evidence, conflicting sources and denied access. Split development and held-out cases by source family or change sequence before tuning. Keep the held-out answer key outside consumer inputs.

**Score the response directly.** Report the count of answerable questions for which the first three results collectively contain all required evidence, with the denominator and missed passages. Report missing-evidence and conflict decisions separately. For change and access fixtures, compare revision IDs and permitted text through both REST and MCP. List each obsolete current result, restricted disclosure and adapter mismatch individually. Freeze the roster and numerical promotion criteria before inspecting held-out results. Keep reruns after tuning identifiable as development evidence.

**Then exercise the agent.** In one smoke test the agent calls the real tool, receives a permitted passage and cites the returned revision. Record tool delivery and the agent's use separately. The primary retrieval test requires neither a model-generated answer nor an LLM grader.

**Make citations replayable.** For each public corpus revision, keep the publisher, original URL, retrieval date, storage permission and snapshot digest in the manifest. Give each expected passage a locator within that snapshot. During replay, resolve the returned citation to its revision and passage, then check whether the passage supports the requested fact. For example, a link to the correct deployment guide still misses if the cited paragraph describes production and the question asks about staging. For a shared lesson, perform this check against the recipient-visible publication and test source resolution separately under the source owner's permissions.

### Research behind the benchmark

| Research observation | Choice for Ganesh |
| --- | --- |
| KILT grounds tasks in a common Wikipedia snapshot and evaluates the provenance models return alongside downstream performance. [Petroni et al., KILT](https://aclanthology.org/2021.naacl-main.200/) | In M1, pin the corpus and score whether the retrieved passages supply the required evidence. The top-three cutoff is a Ganesh design choice. |
| LongMemEval treats temporal reasoning, knowledge updates and abstention as distinct memory abilities. [Wu et al., LongMemEval](https://arxiv.org/abs/2410.10813v2) | Give the same fact a current question, an as-of question and a question the sources cannot settle. Report the outcomes separately so a good average cannot hide stale responses. |
| TransMem makes transitions between user preferences retrievable. [Chen et al., TransMem](https://arxiv.org/abs/2611.10683v1) | Adapt that question to project evidence in M2: after a partial correction, can the caller find which part changed, when it changed and which part still applies? The Ganesh fixture supplies the expected revisions. |
| MemDev-Bench observed failures when code agents applied earlier experience without checking whether it fit the current task. [Chen et al., MemDev-Bench](https://arxiv.org/abs/2610.27524v1) | In the M3 handoff, check that the published lesson carries the condition the receiving agent needs to verify locally. Record delivery and actual use as separate observations. |

The [earlier research catalog](https://github.com/andikapradanaarif/ganesh/blob/2fddc9d2abfe288e85b9f5f5dceedc20d89ef27f/docs/personal-second-memory-roadmap.md#appendix-f-research-basis-and-implementation-commitments) preserves the broader readings and their limitations. Ganesh's permission and revocation requirements come from the owner's sharing boundary and require direct local tests.

## 5. Delivery milestones

| Milestone | Build | Evidence required to finish |
| --- | --- | --- |
| M1 · Evidence retrieval from a fixed corpus | Import frozen public passages and revision metadata. Implement the shared query function, REST read and thin MCP bridge. Start with the simplest ranking method that can be replayed. | A repeatable offline run over a frozen roster returns sufficient passages, citations and explicit insufficient-context outcomes. Both adapters agree. Publish counts, failures and the predeclared pass bar. |
| M2 · Updates and scoped sharing | Add supersession, historical reads, a permitted publication from A to B, revocation and invalidation of dependent views. | On a two-project fixture, every correction, partial correction, conflict, late-arrival and historical case returns the expected revision or status. B can read the permitted lesson. Denied and revoked reads expose zero restricted passages or metadata, including after restart and through cached views. |
| M3 · Automatic handoff between installations | Connect one selected agent client on one local machine and one server. Capture permitted checkpoints and lessons during ordinary work and deliver a brief on resume or query. | Run the first product demonstration above. Record the source event, revision accepted, actual content served, receiving agent's next action, changes after correction or revocation, and any owner intervention. An owner-written summary cannot fill a capture gap. |

The owner chooses the two installations, client, permitted projects, sharing rule and processor permissions before real enrollment. M1 can start with frozen public material. Record retrieval latency, storage and model use where applicable. Adopt richer ranking, extraction or storage only after a measured failure justifies the change.

### First bounded implementation issue

**Implement a reproducible passage lookup from a frozen public corpus (M1).** The deliverable is a Go importer and retrieval core with a REST read, a thin MCP read over the same core, and an offline evaluation command. Use a small corpus with overlapping subjects, at least one paraphrased lookup and one question the sources cannot answer. Version the corpus manifest and safe fixtures. Document the question split, top-three evidence criterion, numerical pass bar and validation command before running the held-out check.

Acceptance requires (1) a clean offline replay from pinned inputs, (2) the expected passage and revision through both adapters for the known lookup, (3) an explicit insufficient-context result for the missing fact, and (4) a held-out report that meets the frozen pass bar and exposes every miss. Product enrollment, live model grading and a hosted MCP service fall outside this issue. A failed retrieval check gives the next change a concrete target.
