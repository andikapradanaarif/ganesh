# Errata — EVALUATION-m2-phase1.md

Per the evaluation-doc discipline (never rewrite the record; supersede or errata),
these corrections stand against the frozen M2P1 document without modifying it:

1. **"Lifecycle scenario: 14/14 PASS through REST+MCP"** — the `lifecheck` runner
   is REST-only; the xfer scenario is REST-only. The REST+MCP claim is true only
   of the hardcoded m2-lifecycle gate (37/37).
2. **"Reads compute eligibility in SQLite before Stage-1 retrieval"** — eligibility
   is computed first from revision metadata, but the FTS scan itself runs
   unfiltered and is post-filtered in Rust with 4x overfetch. The security
   property holds; the recall consequence is that heavy ineligible ranking can
   crowd out eligible passages within the overfetch window.
3. **"late-arrival replay"** — as originally shipped, the lifecycle gate's
   reimport steps returned AlreadyPresent and wrote nothing, so they passed
   vacuously. Fixed in PR #207 by delete-then-reimport; the 37/37 result with
   genuine out-of-order ingestion is recorded there.
4. **Concurrency note** (PR #207 review): after the lock-free restructure,
   citation opens no longer block behind neural inference and the acceptance
   phase parallelizes, but end-to-end concurrent search throughput remains
   bounded by the per-model `Mutex<Session>` in embed/rerank/qa_verify — a
   multi-user deployment would need ONNX session pooling.
5. **Semantic override entity-binding gap** (PR #207 review): the M2 semantic
   coverage override assumed strong QA margin implies responsive evidence.
   That assumption is false for the entity-swap class — extractive QA cannot
   represent entity binding, so a false-premise entity question (v6-core i04)
   yields a confident span anyway. Fixed in PR #207 by disarming the coverage
   waiver whenever a capitalized proper-noun query term is unmatched in the
   selected passages.
