# Automated Bead Store Checkpoint Sync

This system automatically detects and recovers from bead starvation by syncing the bead store from the git-tracked checkpoint when the database becomes stale.

## Problem

When the bead store database (`.beads/beads.db`) becomes older than the git-tracked checkpoint (`.beads/checkpoint/forensic.jsonl`), workers may fail to find ready beads even though work exists. This is a starvation condition that prevents progress.

## Solution

A scheduled script that:
1. Checks if the checkpoint is newer than the database
2. If so, automatically imports from checkpoint
3. Verifies that ready beads are available
4. Escalates if starvation persists after sync

## Components

- `scripts/bead-starvation-sync.sh` - The sync script
- `scripts/bead-starvation-sync.service` - Systemd service definition
- `scripts/bead-starvation-sync.timer` - Systemd timer (runs every 5 minutes)
- `scripts/install-bead-starvation-sync.sh` - Installation script

## Installation

```bash
# Install as a user service (recommended)
./scripts/install-bead-starvation-sync.sh

# Or install system-wide (requires root)
sudo ./scripts/install-bead-starvation-sync.sh
```

## Usage

### Manual Run

```bash
./scripts/bead-starvation-sync.sh
```

### Check Status

```bash
# User service
systemctl --user status bead-starvation-sync.timer
systemctl --user list-timers | grep bead-starvation-sync

# System service (if installed system-wide)
systemctl status bead-starvation-sync.timer
systemctl list-timers | grep bead-starvation-sync
```

### View Logs

```bash
# Script logs
tail -f .beads/diagnostics/starvation-sync-*.log

# Systemd logs
journalctl --user -u bead-starvation-sync.service -f
# Or system-wide
journalctl -u bead-starvation-sync.service -f
```

### Stop/Remove

```bash
# Stop the timer
systemctl --user stop bead-starvation-sync.timer
systemctl --user disable bead-starvation-sync.timer

# Remove the units
rm ~/.config/systemd/user/bead-starvation-sync.*
systemctl --user daemon-reload
```

## Exit Codes

- `0` - Healthy, no issues detected
- `1` - Error during checkpoint import
- `2` - ESCALATION REQUIRED: No ready beads after sync

## Escalation

If the script exits with code 2, this indicates a deeper issue requiring manual intervention:
- The database may be corrupted
- The checkpoint may be invalid
- There may be actual starvation (no work available)

Check the log file for details: `.beads/diagnostics/starvation-sync-*.log`

## Testing

To test the sync mechanism, you can temporarily make the checkpoint newer than the database:

```bash
touch .beads/checkpoint/forensic.jsonl
./scripts/bead-starvation-sync.sh
```

This will trigger a sync even if the contents are the same.
