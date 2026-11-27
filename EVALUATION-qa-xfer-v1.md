# qa-xfer-v1 evaluation — SQuAD 2.0 unanswerability verifier (candidate 6c593a1d)

## Verdict: RETAINED — supersedes #188 per frozen rule (all four gates pass)

## Model
deepset/minilm-uncased-squad2, BertForQuestionAnswering, safetensors sha256 fcfafe0204b00bab…, 47MB,
tokenizer bert-base-uncased (vocab 30522 verified aligned), tau = 0.0 (natural SQuAD2 boundary), CPU ~0.1-0.3s/query top-3.
Smoke calibration: supported +14.13, missing-unit -4.83, entity-swap +13.89 (predicted uncatchable).

## Frozen gates
1. v4: 23/24 coverage + genuine 5/5 typed abstentions (i01-i04,i06; i05 erratum answered desirably) PASS
2. qa-xfer-v1 fresh frozen paired:
   - coverage: #188 4/8, 3aeba5c6 7/8, candidate 8/8 — improves over #188 PASS
   - abstention: #188 1/6, 3aeba5c6 1/6, candidate 5/6 — improves over both; strict improvement over 3aeba5c6 PASS
   - zero newly unsupported ok (q09 leaks in all arms incl. #188) PASS
3. v2: 28/30 (22/24 + 6/6), v04/v05/v06 hit, v28+v30 abstain; probes 49/49 (event 12, action 8, chain 13, quantity 8, rail 8) PASS
4. fail-loud GANESH_REQUIRE_QA_VERIFIER=1 + GANESH_REQUIRE_RERANKER=1 verified FATAL; digests logged in all 6 reports; 0 provider calls PASS

## qa-xfer-v1 per-case (candidate)
q01-q08 ALL HIT (8/8) — realized dates, irregular verb, count, decimal unit, entity, location, both compounds.
q10 CCF-SouthAfrica entity-swap ABSTAIN correct; q11 Asiatic extinction ABSTAIN; q12 Hungary sill planned-future ABSTAIN;
q13 mtDNA nanometers missing-unit ABSTAIN; q14 Nobel Literature false-premise ABSTAIN.
q09 Pierre Curie Chemistry entity-swap LEAKS — sole insufficient leak; same uncatchable class as calibration predicted.

## Diagnostics on exposed packets
ax4: abstention 2/4 (q03 Apollo-18 NOW correct, up from 1/4 in PR197); q07 entity-swap + q09 inches remain; coverage 6/8 unchanged.
ax3: 6/7 (q08 Artemis III still leaks through dense realized-sibling prose); coverage 8/9 maintained.
ax2: maintained.

## Honest residuals
- Entity-swap class (q09, ax4-q07) is structurally uncatchable by extractive span-vs-null scoring: answer-shaped spans exist.
- v03 newly false-abstains (v2 29/30 -> 28/30, within frozen tolerance); v01 chronic as in all cross-encoder arms.
- ax4-q09 inches: verifier catches absence-of-dimension (q13 nm) but not wrong-unit-when-dimension-exists.
