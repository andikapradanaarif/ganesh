# Actions vs outcomes — contract addition (issue #169)

A recorded event proves only what it states, for the subject and scope it
states it about. The receiving agent's reading:

| Recorded evidence | Question | Verdict |
|---|---|---|
| "The restore started." | Did it finish? | insufficient — a started action says nothing about completion |
| "The restore finished." | Did it complete? | ok — finish/complete are equivalent wording for the same event |
| "The restore did not finish." | Did it finish? | ok — explicit negative |
| "The check was not run." | Was it run? | ok — explicit negative |
| "The check was not run." | Did it pass? / Did it fail? | insufficient each — an unperformed check has no outcome |
| "R7 was deployed to production after the checklist passed." | Was R7 deployed? | ok — deployment recorded |
| same | Was R7 verified running afterward? | insufficient — deployment is not a post-deployment observation |
| "After deployment, the production health check confirmed R7 was running." | Was R7 verified running in production? | ok — post-deployment observation recorded |

Rules: (1) positive evidence must establish the requested action or a justified
equivalent — finish/complete synonymy, or a documented directional inference
(completion entails the action started, never the reverse); (2) a shared
category such as "verification" is too broad — one event never proves another
event merely because both are verify-class; (3) outcome questions (passed/failed)
require the outcome to be observed, not the activity to have occurred or not
occurred; (4) subject and scope binding still apply per the v1 contract.

## Key audit consequence

qB05 in contrast-transition v2 ("Was revision R7 verified running in
production?", keyed ok on "deployed to production after the checklist passed")
credits deployment evidence as post-deployment verification. Under this
contract the label is wrong: no post-deployment observation is recorded.
Corrected in key.v3 as a **benchmark correction** (historical v2 scores
preserved). qB04 is unaffected: "the deployment check confirmed R8 was running
in staging" is itself an observation statement. qB07a ("did R7 reach
production") remains ok on the deployment record.
