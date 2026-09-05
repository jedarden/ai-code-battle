# Bead Starvation Sync Deployment Guide

## Overview

The `bead-starvation-sync` script automatically detects and resolves bead starvation by syncing the bead store from the git-tracked checkpoint when it's newer than the database. This prevents worker starvation when the database becomes out of sync with the durable checkpoint.

## Problem It Solves

When `bead list --ready` returns zero results despite open beads existing, NEEDLE workers starve. This happens when:
- The SQLite database (`beads.db`) becomes corrupted or out of sync
- The checkpoint (`checkpoint/forensic.jsonl`) is newer than the database
- Clock skew or filesystem issues cause timestamp mismatches

## Script Features

- **Automatic detection**: Compares timestamps between `beads.db` and `checkpoint/forensic.jsonl`
- **Safe sync**: Creates backups before any destructive operations
- **Verification**: Checks ready frontier before and after sync
- **Escalation**: Can email administrators if starvation persists
- **Comprehensive logging**: Writes detailed logs to `.beads/diagnostics/`

## Deployment Options

### Option 1: Cron Job (Simple)

Add to the crontab for the user running NEEDLE workers:

```bash
# Edit crontab
crontab -e

# Add this line to run every 5 minutes
*/5 * * * * /home/coding/ai-code-battle/scripts/bead-starvation-sync
```

### Option 2: Systemd Timer (Recommended)

Deploy the provided systemd service and timer files:

```bash
# Install as user systemd units
cd /home/coding/ai-code-battle/scripts

# Install the service and timer
systemctl --user link --absolute $PWD/bead-starvation-sync.service
systemctl --user link --absolute $PWD/bead-starvation-sync.timer

# Enable and start the timer
systemctl --user enable bead-starvation-sync.timer
systemctl --user start bead-starvation-sync.timer

# Verify it's running
systemctl --user list-timers | grep bead-starvation
```

## Testing

Before deploying, test the script:

```bash
# Dry run (no changes)
cd /home/coding/ai-code-battle
./scripts/bead-starvation-sync --dry-run

# Test with force sync (careful - will actually sync)
./scripts/bead-starvation-sync --force

# Check the logs
cat .beads/diagnostics/sync-*.log | tail -50
```

## Escalation Configuration

To enable email escalation when starvation persists after sync:

```bash
# Add escalation to cron job
*/5 * * * * /home/coding/ai-code-battle/scripts/bead-starvation-sync --escalate-to admin@example.com

# Or modify the systemd service ExecStart line
ExecStart=/home/coding/ai-code-battle/scripts/bead-starvation-sync --escalate-to admin@example.com
```

## Monitoring

Check sync activity:

```bash
# View recent logs
tail -f .beads/diagnostics/sync-*.log

# Check systemd journal (if using systemd timer)
journalctl --user -u bead-starvation-sync.service -f

# See when last sync ran
systemctl --user list-timers | grep bead-starvation-sync
```

## Troubleshooting

### Script runs but doesn't sync

Check if timestamps actually differ:
```bash
stat -c '%Y %n' .beads/beads.db .beads/checkpoint/forensic.jsonl
```

Use `--force` flag to sync regardless:
```bash
./scripts/bead-starvation-sync --force
```

### Sync fails

Check the log file for detailed error messages:
```bash
cat .beads/diagnostics/sync-*.log | grep ERROR
```

If sync failed, the script automatically restores from backup.

### Starvation persists after sync

This indicates deeper issues. Run the diagnostic script:
```bash
./scripts/bead-starvation-diagnose
```

### What the diagnostic script reports

`bead-starvation-diagnose` re-runs the ready-frontier (pluck) query two
independent ways — `bead list --ready` and `bead analyze-exclusion --show-sql`
— states a reason for every open bead, and cross-checks the two queries
against each other. Its verdict says which is true:

- `NO_STARVATION` — the frontier is workable; the triggering alert was a false positive
- `GENUINE_STARVATION` — the frontier is empty in both queries and every open bead is excluded for a documented reason
- `QUERY_BUG` — the queries disagree with each other or with the open enumeration

Reports land in `.beads/diagnostics/starvation-<timestamp>.txt` (plus `.json`
for machines). Exit codes are distinct so a monitor can tell "nothing to
claim" from "the query is lying": `0` no starvation, `1` genuine starvation,
`2` query bug.

Verify changes to the analysis logic without touching a bead store:
```bash
./scripts/bead-starvation-diagnose --self-test
```

Counting note: `bead list --json` emits compact JSONL (one object per line)
and prints a bare `[]` when the result is empty. Count it with
`jq -s 'length'` (slurp), as the sync script does — never bare
`jq 'length'`, which runs once per input line and prints each object's key
count, so 5 beads of 15 fields report "15" five times. Every count in the
`starvation-2026-09-01T*.txt` reports generated before 2026-09-05 was wrong
for exactly that reason.

## Files Created

- `scripts/bead-starvation-sync` - Main sync script
- `scripts/bead-starvation-sync.service` - Systemd service file
- `scripts/bead-starvation-sync.timer` - Systemd timer file (runs every 5 minutes)
- `scripts/BEAD_SYNC_DEPLOYMENT.md` - This deployment guide

## Log Files

Logs are written to `.beads/diagnostics/`:
- `sync-<timestamp>.log` - Individual sync run logs
- `escalation.log` - Escalation history (if configured)

## Exit Codes

- `0` - No starvation detected or sync resolved it
- `1` - Starvation detected (ready frontier empty despite open beads)

This can be used in monitoring systems:
```bash
#!/bin/bash
if ! /home/coding/ai-code-battle/scripts/bead-starvation-sync; then
    echo "CRITICAL: Bead starvation detected" | mail -s "Bead Alert" admin@example.com
fi
```

## Integration with Existing Scripts

This sync script complements the existing `bead-starvation-diagnose` script:

- `bead-starvation-sync` - **Automated prevention** - runs proactively every 5 minutes
- `bead-starvation-diagnose` - **Manual investigation** - run when issues persist

Use both together for robust starvation detection and resolution.
