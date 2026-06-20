#!/usr/bin/env bash
#
# fix-depends-on-id-column.sh (hq-fu3)
#
# Normalises the `depends_on_id` column in the `dependencies` table across
# every served rig Dolt database so that the `bd` CLI (beads >= 1.0.x) can
# create parent-child / dependency edges.
#
# ## Why
#
# The bd CLI issues an INSERT into `dependencies` that EXPLICITLY lists
# `depends_on_id` in its column list. Across the rig DBs the schema has
# drifted into three shapes:
#
#   1. depends_on_id is a STORED GENERATED column
#        -> INSERT fails: "value for generated column depends_on_id not allowed"
#   2. depends_on_id column does not exist (modern split-target schema:
#      depends_on_issue_id / depends_on_wisp_id / depends_on_external)
#        -> INSERT fails: "Unknown column 'depends_on_id'"
#   3. depends_on_id is already a plain writable column
#        -> INSERT works (no change needed)
#
# This was originally mis-diagnosed (hq-fu3) as a Dolt generated-column bug.
# In fact Dolt 2.1.4 correctly accepts INSERTs that OMIT a generated column;
# the breakage is bd writing the generated column explicitly. The in-reach
# fix (beads is an external upstream project) is to make `depends_on_id` a
# plain writable varchar in every DB. Read paths use
# COALESCE(depends_on_issue_id, depends_on_wisp_id, depends_on_external)
# rather than the stored column, so dropping the GENERATED behaviour is safe.
#
# ## Behaviour
#
# Idempotent and state-aware per DB:
#   - GENERATED  -> ALTER ... MODIFY COLUMN depends_on_id varchar(255)
#   - MISSING    -> ALTER ... ADD COLUMN depends_on_id varchar(255)
#                   (only for the split-target schema; legacy single-column
#                    DBs already have a plain depends_on_id and are skipped)
#   - PLAIN      -> skip
#
# Re-running after success is a no-op.
#
# Usage:
#   scripts/fix-depends-on-id-column.sh            # apply to all served DBs
#   DRY_RUN=1 scripts/fix-depends-on-id-column.sh  # print plan, change nothing
#
set -euo pipefail

HOST="${DOLT_HOST:-127.0.0.1}"
PORT="${DOLT_PORT:-3307}"
USER="${DOLT_USER:-root}"
DRY_RUN="${DRY_RUN:-0}"

# dolt CLI reads the (empty) password from this env var without prompting.
export DOLT_CLI_PASSWORD="${DOLT_CLI_PASSWORD:-}"

dsql() {
  # dsql <db> <query>
  local db="$1" q="$2"
  dolt --host "$HOST" --port "$PORT" --user "$USER" --no-tls --use-db "$db" \
    sql -q "$q" -r csv 2>&1
}

# Discover served databases from `gt dolt status` (the authoritative list of
# what the running server actually serves).
mapfile -t DBS < <(gt dolt status 2>/dev/null \
  | sed -n 's/^    - \([a-zA-Z0-9_]*\).*/\1/p')

if [[ ${#DBS[@]} -eq 0 ]]; then
  echo "ERROR: no served databases found (is the Dolt server running?)" >&2
  exit 1
fi

echo "Target databases: ${DBS[*]}"
[[ "$DRY_RUN" == "1" ]] && echo "(DRY RUN — no changes will be applied)"
echo

rc=0
for db in "${DBS[@]}"; do
  schema="$(dsql "$db" "SHOW CREATE TABLE dependencies;" || true)"

  if ! grep -q "depends_on_id" <<<"$schema"; then
    has_col=0
  else
    has_col=1
  fi
  has_split=0
  grep -q "depends_on_issue_id" <<<"$schema" && has_split=1
  is_generated=0
  grep -q "GENERATED ALWAYS" <<<"$schema" && is_generated=1

  if [[ "$has_col" == "1" && "$is_generated" == "1" ]]; then
    action="MODIFY (generated -> plain)"
    stmt="ALTER TABLE dependencies MODIFY COLUMN depends_on_id varchar(255);"
  elif [[ "$has_col" == "0" && "$has_split" == "1" ]]; then
    action="ADD (missing -> plain)"
    stmt="ALTER TABLE dependencies ADD COLUMN depends_on_id varchar(255);"
  elif [[ "$has_col" == "0" && "$has_split" == "0" ]]; then
    action="SKIP (no dependencies table / unknown schema)"
    stmt=""
  else
    action="SKIP (already plain)"
    stmt=""
  fi

  printf "%-18s %s\n" "$db" "$action"

  if [[ -n "$stmt" && "$DRY_RUN" != "1" ]]; then
    if out="$(dsql "$db" "$stmt")"; then
      echo "    applied: $stmt"
    else
      echo "    FAILED:  $out" >&2
      rc=1
    fi
  fi
done

echo
if [[ "$DRY_RUN" == "1" ]]; then
  echo "Dry run complete."
else
  echo "Migration complete (rc=$rc)."
fi
exit "$rc"
