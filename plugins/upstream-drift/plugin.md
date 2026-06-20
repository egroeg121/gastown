+++
name = "upstream-drift"
description = "File a bead when the gastown fork's main is behind gastownhall/gastown upstream"
version = 1

[gate]
type = "cooldown"
duration = "24h"

[tracking]
labels = ["plugin:upstream-drift", "rig:gastown", "category:maintenance"]
digest = true

[execution]
timeout = "5m"
notify_on_failure = true
severity = "medium"
+++

# Upstream Drift Check

Fetches from `upstream` (gastownhall/gastown) and counts how many commits
`origin/main` is behind. If the count exceeds the threshold (default: 5),
files an `hq-*` bead requesting a rebase and escalates to the Mayor.

**SAFETY**: This plugin NEVER rebases, force-pushes, or modifies any branch.
It is read-only — it only files a bead and records a wisp.

## Gate Check

Deacon evaluates cooldown (24h) before dispatch.

## Step 1: Fetch upstream

```bash
BARE_REPO="${GT_TOWN_ROOT:-$(gt town root 2>/dev/null)}/gastown/.repo.git"
git -C "$BARE_REPO" fetch upstream --quiet 2>&1
```

## Step 2: Count drift

```bash
BEHIND=$(git -C "$BARE_REPO" rev-list --count upstream/main ^origin/main 2>/dev/null || echo 0)
```

## Step 3: Threshold check

Default threshold is 5 commits. If `BEHIND <= THRESHOLD`, record success wisp and exit.

```bash
THRESHOLD="${UPSTREAM_DRIFT_THRESHOLD:-5}"
if [ "$BEHIND" -le "$THRESHOLD" ]; then
  log "Fork is $BEHIND commit(s) behind upstream — within threshold ($THRESHOLD). OK."
  gt plugin record-run --plugin upstream-drift --result success --rig gastown \
    --title "upstream-drift: fork is $BEHIND commit(s) behind (OK)" >/dev/null 2>&1 || true
  exit 0
fi
```

## Step 4: File bead and escalate

If drift exceeds threshold:

```bash
bd create --repo hq \
  --title "Rebase gastown fork onto upstream/main ($BEHIND commits behind)" \
  --type task --priority 2 \
  --description "upstream-drift plugin detected the fork is $BEHIND commits behind gastownhall/gastown upstream/main. Manual rebase required: cd ~/code/gastown && git fetch upstream && git rebase upstream/main && git push --force-with-lease origin main. Do NOT auto-rebase or force-push without review."
```

```bash
gt escalate --severity=medium \
  --subject="upstream-drift: fork is $BEHIND commits behind upstream" \
  --body="gastownhall/gastown has $BEHIND commits not yet in egroeg121/gastown fork. File bead created. To fix: cd ~/code/gastown && git fetch upstream && git rebase upstream/main && git push --force-with-lease origin main"
```

```bash
gt plugin record-run --plugin upstream-drift --result success --rig gastown \
  --title "upstream-drift: fork is $BEHIND commits behind — bead filed" \
  --description "Fork is $BEHIND commits behind upstream. Bead filed." >/dev/null 2>&1 || true
```

## On failure

```bash
gt plugin record-run --plugin upstream-drift --result failure --rig gastown \
  --title "upstream-drift: FAILED" \
  --description "Upstream drift check failed: $ERROR" >/dev/null 2>&1 || true

gt escalate --severity=medium \
  --subject="Plugin FAILED: upstream-drift" \
  --body="$ERROR" \
  --source="plugin:upstream-drift"
```
