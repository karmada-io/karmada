# Overview

This document explains how cherry picks are managed on release branches within
the `karmada-io/karmada` repository.
A common use case for this task is backporting PRs from master to release
branches.

> This doc is lifted from [Kubernetes cherry-pick](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-release/cherry-picks.md).

- [Prerequisites](#prerequisites)
- [What Kind of PRs are Good for Cherry Picks](#what-kind-of-prs-are-good-for-cherry-picks)
- [Initiate a Cherry Pick](#initiate-a-cherry-pick)
- [Cherry Pick Review](#cherry-pick-review)
- [Troubleshooting Cherry Picks](#troubleshooting-cherry-picks)
- [Cherry Picks for Unsupported Releases](#cherry-picks-for-unsupported-releases)

## Prerequisites

- A pull request merged against the `master` branch.
- The release branch exists (example: [`release-1.0`](https://github.com/karmada-io/karmada/tree/release-1.0))
- The normal git and GitHub configured shell environment for pushing to your
  karmada `origin` fork on GitHub and making a pull request against a
  configured remote `upstream` that tracks
  `https://github.com/karmada-io/karmada.git`, including `GITHUB_USER`.
- Have GitHub CLI (`gh`) installed following [installation instructions](https://github.com/cli/cli#installation).
- A github personal access token which has permissions "repo" and "read:org".
  Permissions are required for [gh auth login](https://cli.github.com/manual/gh_auth_login)
  and not used for anything unrelated to cherry-pick creation process
  (creating a branch and initiating PR).

## What Kind of PRs are Good for Cherry Picks

Compared to the normal master branch's merge volume across time,
the release branches see one or two orders of magnitude less PRs.
This is because there is an order or two of magnitude higher scrutiny.
Again, the emphasis is on critical bug fixes, e.g.,

- Loss of data
- Memory corruption
- Panic, crash, hang
- Security

A bugfix for a functional issue (not a data loss or security issue) that only
affects an alpha feature does not qualify as a critical bug fix.

If you are proposing a cherry pick and it is not a clear and obvious critical
bug fix, please reconsider. If upon reflection you wish to continue, bolster
your case by supplementing your PR with e.g.,

- A GitHub issue detailing the problem

- Scope of the change

- Risks of adding a change

- Risks of associated regression

- Testing performed, test cases added

- Key stakeholder reviewers/approvers attesting to their confidence in the
  change being a required backport

It is critical that our full community is actively engaged on enhancements in
the project. If a released feature was not enabled on a particular provider's
platform, this is a community miss that needs to be resolved in the `master`
branch for subsequent releases. Such enabling will not be backported to the
patch release branches.

## Initiate a Cherry Pick

- Run the [cherry pick script][cherry-pick-script]

  This example applies a master branch PR #1206 to the remote branch
  `upstream/release-1.0`:

  ```shell
  hack/cherry_pick_pull.sh upstream/release-1.0 1206
  ```

  - Be aware the cherry pick script assumes you have a git remote called
    `upstream` that points at the Karmada github org.

  - You will need to run the cherry pick script separately for each patch
    release you want to cherry pick to. Cherry picks should be applied to all
    active release branches where the fix is applicable.

  - If `GITHUB_TOKEN` is not set you will be asked for your github password:
    provide the github [personal access token](https://github.com/settings/tokens) rather than your actual github
    password. If you can securely set the environment variable `GITHUB_TOKEN`
    to your personal access token then you can avoid an interactive prompt.
    Refer [https://github.com/github/hub/issues/2655#issuecomment-735836048](https://github.com/github/hub/issues/2655#issuecomment-735836048)

## Cherry Pick Review

As with any other PR, code OWNERS review (`/lgtm`) and approve (`/approve`) on
cherry pick PRs as they deem appropriate.

The same release note requirements apply as normal pull requests, except the
release note stanza will auto-populate from the master branch pull request from
which the cherry pick originated.

## Troubleshooting Cherry Picks

Contributors may encounter some of the following difficulties when initiating a
cherry pick.

- A cherry pick PR does not apply cleanly against an old release branch. In
  that case, you will need to manually fix conflicts.

- The cherry pick PR includes code that does not pass CI tests. In such a case
  you will have to fetch the auto-generated branch from your fork, amend the
  problematic commit and force push to the auto-generated branch.
  Alternatively, you can create a new PR, which is noisier.

## Cherry Picks for Unsupported Releases

The community supports & patches releases need to be discussed.

[cherry-pick-script]: https://github.com/karmada-io/karmada/blob/master/hack/cherry_pick_pull.sh
