---
name: CherryPick Track
about: Track tasks when release branches need cherry-pick.
labels: help wanted
---

**Which PR needs cherry-picks:**
<!--
For example: "PR #1234"
-->
PR #

**Which release branches need this patch:**
<!--
Karmada now maintains 3 latest releases.
If a branch doesn't need this cherry-pick, please explain the reason.
-->
- [ ] release-1.x 
- [ ] release-1.y
- [ ] release-1.z

**How to cherry-pick PRs:**

The `hack/cherry_pick_pull.sh` script can help you initiate a cherry-pick
automatically, please follow the instructions at [this guideline](https://karmada.io/docs/contributor/cherry-picks). 

The script will send the PR for you, please remember `copy the release notes` from
the original PR by to the new PR description part.

**How to join or take the task**:

Just reply on the issue with the message `/assign` in a **separate line**.

Then, the issue will be assigned to you.

**Useful References:**

- Release timeline: https://karmada.io/docs/releases
- How to cherry-pick PRs: https://karmada.io/docs/contributor/cherry-picks

**Anything else we need to know:**

