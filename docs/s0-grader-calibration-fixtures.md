# S0 grader-calibration fixtures

Continuation of [#60](https://github.com/andikapradanaarif/ganesh/issues/60), under [S0 #6](https://github.com/andikapradanaarif/ganesh/issues/6). **Status: synthetic fixtures prepared; owner labels and grading-model calibration pending.** This uses the existing [six private task drafts](s0-calibration-candidates.md) without expanding the source review or changing their prompts, rubrics or expected outcomes.

Each task now has three synthetic answer variants: an assistant-proposed complete source-grounded answer, a variant with one planted critical error, and a safe but incomplete variant. The latter two are exact single-span edits of the first. A Go producer rejects absent, repeated or unchanged edit anchors and binds each variant to the parent prompt/rubric/oracle hashes. These are prepared fixtures rather than recorded answers from an evaluation model.

The linked variants test whether a grader detects the deliberately inserted defect and omission classes. They do not sample the distribution of naturally occurring errors. Shared wording and edit locality may provide cues, so even perfect agreement on these fixtures would not establish performance on independently generated answers. The 18 variants span six tasks and are not 18 independent natural samples.

| Task role | Proposed complete | Planted critical error | Safe incomplete | Total cases |
| --- | ---: | ---: | ---: | ---: |
| Coder | 2 | 2 | 2 | 6 |
| Planner | 2 | 2 | 2 | 6 |
| Reviewer | 2 | 2 | 2 | 6 |
| **Total** | **6** | **6** | **6** | **18** |

The preparation created **six private output files**, totalling **68,444 bytes**: one answer-case file, one proposed-label file, one blank owner-label file, one owner-review packet, one proposed grader prompt and one fixture manifest. The separate answer-edit input is excluded from those output counts. Answer lengths range from **95 to 133 words**; these are word counts, not model tokens or proof of freedom from verbosity bias.

Case IDs use neutral suffixes, and the review packet omits the proposed class labels. A deterministic hash order assigns the three variants per task to those IDs; this does not establish statistical blinding or independent authorship. The packet contains answers and task rubrics, with links to the already-reviewed source excerpts. Proposed labels are in a separate file so the owner can label first and compare afterward. Copy the private blank label file to a new 0600 adjudication file before editing; the original blank and manifest remain preserved. All **18 owner labels are unset**. The initial draft packet was retained separately after its labeling instructions were corrected; the six-file count above describes the final packet.

A separate local Go checker verified all saved output hashes/private modes, parent-task bindings, answer hashes, exact variant edits, class counts and blank labels. The exact-edit regression fixtures, public document links, git exclusion and diff checks passed. Source text, synthetic answers, proposed labels and task details remain outside version control with 0700/0600 storage.

This preparation pass ran from **16:42:02 to 16:51:01 UTC on 2026-11-08**, or **8m59s elapsed**, including scoping, preparation, the packet correction, validation and PR finalization. The earlier preparation overrun is preserved separately; cumulative S0 focused effort remains unknown.

The same assistant authored the tasks, expected outcomes, answers and proposed labels. The proposed labels are therefore unadjudicated hypotheses, not independent ground truth. Their source support remains limited to attributed agent reports; no live project state was verified. The packet supports one-off experiment labeling, not recurring owner review in the intended product. No grader API calls, agreement measurement, rubric acceptance, model/context fit or held-out answers occurred. These cases are reserved for calibration and cannot become held-out evidence or automatic-memory inputs. The [run-input proposal](s0-run-input-proposal.md) still awaits API budget, timeouts, retries and provider disclosure decisions; the earlier preparation overrun remains recorded.
