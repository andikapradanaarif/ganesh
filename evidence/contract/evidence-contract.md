# Evidence contract — when retrieved evidence settles a question (v1)

Issue #165. Written before any behavior change. This contract defines the
meaning of the two public statuses from the perspective of someone asking
Ganesh a question. It changes no runtime behavior by itself; the scorer and
fixtures make it executable.

## Definitions

**ok** — the permitted returned passages contain enough evidence to *settle the
requested fact*. A question is settled when the evidence establishes any of:

1. an **affirmative answer** — the proposition the question asks about holds
   ("Did R7 reach production?" — the log records R7 deployed to production);
2. an **explicit negative answer** — the evidence states the proposition does
   not hold or did not happen ("Did the team confirm R6?" — the log says the
   team did not confirm R6; the answer is *no*, and that is a settled answer,
   not a missing one);
3. a **requested value** — the quantity, name, or other value the question asks
   for is stated, attached to the right subject and property.

Ganesh does not editorialize: it delivers the answer by returning the passages
that settle it. A useful negative answer needs no special API — the right
passage *is* the negative answer.

**insufficient_context** — a *material part* of the question remains unresolved
after considering the permitted passages. Material parts include the actor, the
action or state asked about, the object or revision, the environment, and any
qualifier the question explicitly carries. Evidence that merely discusses the
subject, or settles a *different* proposition than the one asked, leaves the
question unresolved.

Resolution tests that apply the definitions:

- **Absence is not a negative.** No mention of a confirmation is not evidence
  of non-confirmation. Only an explicit statement settles a negative.
- **A different environment does not transfer.** Evidence about staging leaves
  production unresolved, and vice versa.
- **A different actor does not transfer.** A deployment record that names no
  verifier leaves "who verified" — and "did the team verify" under a strict
  reading — unresolved.
- **A different state does not transfer.** "X remained the default" does not
  resolve whether X became optional: optional and default are compatible
  properties, so the text is silent on the asked proposition.
- **Weaker epistemic states do not settle stronger ones.** Proposing or
  attempting a deployment does not establish that it was verified. Stronger
  states do settle weaker ones within the same chain (a verified deployment
  implies it was attempted).
- **Conflicting evidence** (per the roadmap) is not settlement: if two
  permitted passages genuinely contradict each other on the asked proposition,
  the question is not settled by silently preferring one. Conflicts are out of
  scope for the current synthetic fixtures, which contain none.

## The five reference cases (from the issue table)

| Question and evidence | Contract behavior |
|---|---|
| "Did the team confirm R6?" / log says the team did not confirm R6 | **ok** — explicit negative settles it |
| "Was R6 deployed?" / only evidence is non-confirmation | **insufficient_context** — an unconfirmed deployment could still have occurred |
| "Did R7 reach production?" / log records R7 deployed to production | **ok** — affirmative |
| "Who verified R7 in production?" / deployment recorded, verifier unnamed | **insufficient_context** — actor unresolved |
| "Did lazy evaluation become optional?" / eager became optional, lazy remained default | **insufficient_context** — remaining the default is compatible with either optionality state |

## Audit of the ten historical transition cases

Historical result preserved exactly as recorded: **9/10 on the v1 key** (qB08
leaked). The audit below re-reads each case against the contract. `KEY-v2`
labels are the corrected ones; see `fixtures/contrast-transition/key.v2.json`.

**qB01** "Was the deployment of revision R9 of Northstar proposed?"
Proposition: propose(actor=team, object=deploy(R9)). Material qualifiers: none
beyond the object. Decisive span: "In March, the team proposed deploying
revision R9 of the Northstar service to production." Direct statement, no
assumption. v1 label **ok — correct**.

**qB02** "Was revision R9 of Northstar verified as deployed?"
Proposition: verified(deployed(R9)) — without environment or time qualifier.
Evidence: proposal; attempt whose "outcome was unverified". Two reasonable
readings diverge. Lenient: the handoff log is the whole record and it
affirmatively reports non-verification, so the answer is a settled *no*.
Strict: "the outcome was unverified" predicates over the attempt's outcome at
attempt time; a later verification is simply not in the log either way, so the
question is unresolved. **Split** into unambiguous variants:
qB02a "Did the team verify the outcome of the revision R9 deployment attempt?"
— the log states the outcome was unverified: **ok, explicit negative**.
qB02b "Was revision R9 ever verified running in production?" — the log is
silent on production for R9: **insufficient_context**.

**qB03** "Was revision R8 verified running in production?"
Proposition: verified-running(R8, production). Decisive span confirms staging
only. Environment does not transfer. v1 label **insufficient — correct**.

**qB04** "Was revision R8 verified running in staging?"
Same span, matching environment. v1 label **ok — correct**.

**qB05** "Was revision R7 verified running in production?"
Proposition: verified-running(R7, production). Decisive span: "Revision R7 was
deployed to production after the checklist passed." Assumption required by the
ok label: a passed deployment checklist constitutes verification, and a
completed deployment-to-production entails running in production at least at
completion. The stricter reading (running-as-state never reconfirmed) is
possible but not equally reasonable for a handoff log whose checklist *is* the
verification instrument. Single dominant reading; v1 label **ok — correct**,
assumption recorded.

**qB06** "Was revision R6 of Northstar confirmed?"
Proposition: confirmed(R6). Decisive span: "The team did not confirm revision
R6 in any environment." — an explicit negative that settles the question. The
v1 label **insufficient was wrong** — it treated the absence-of-confirmation
statement as absence of evidence rather than as evidence of the negative.
Benchmark correction: v2 label **ok** (negative). Assumption none.

**qB07** "Did the Northstar team verify revision R7 in production?"
Proposition: verify(actor=team, R7, production). The decisive span is passive
and names no actor. Lenient reading: a team handoff log describing its own
process implies team action — settled yes. Strict reading: no actor is named,
so whether *the team* verified is unresolved. Two reasonable readings diverge.
**Split**: qB07a "Did revision R7 reach production?" — **ok**, affirmative.
qB07b "Who verified revision R7 in production?" — **insufficient** — actor
unresolved per the contract.

**qB08** "Did lazy evaluation become optional in Lazylang?"
Evidence: eager evaluation became optional; type annotations are optional;
lazy evaluation "remained the default in every version". Per the contract,
remaining the default is compatible with being optional-or-mandatory, so the
asked proposition is unresolved. v1 label **insufficient — correct**, and the
v1 leak (accepting it) stands as a genuine failure of lexical binding, not a
labeling error. Note the decisive span *does* affirmatively settle the
different proposition "did lazy evaluation remain the default" — that is not
what was asked.

**qB09** "Did eager evaluation become optional in Lazylang?"
Direct statement. v1 label **ok — correct**.

**qB10** "Are type annotations optional in Lazylang?"
Direct statement. v1 label **ok — correct**.

## Audit summary

- 7 of 10 v1 labels confirmed without assumption changes.
- qB06 relabeled insufficient → ok-negative (**benchmark correction**).
- qB02 and qB07 split into two unambiguous variants each.
- The corrected v2 key has 12 cases: 7 ok (one of them negative), 5
  insufficient (qB02b, qB03, qB07b, qB08, plus qB02a's negative counterpart is
  counted as ok — see the key for exact enumeration).
- Some apparent candidate improvement on the v1 key is partly explained by the
  mislabeled qB06: the v1 reference's "correct" abstention on qB06 was correct
  for the wrong reason. This is reported as a benchmark correction, not a
  regression.
