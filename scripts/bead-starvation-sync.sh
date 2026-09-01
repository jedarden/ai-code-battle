#!/usr/bin/env bash
# Automated bead store checkpoint sync
# Runs periodically to detect and recover from bead starvation
# When checkpoint is newer than beads.db, imports from checkpoint and verifies

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKSPACE="${SCRIPT_DIR}/.."
BEADS_DIR="${WORKSPACE}/.beads"
BEADS_DB="${BEADS_DIR}/beads.db"
CHECKPOINT_FILE="${BEADS_DIR}/checkpoint/forensic.jsonl"
LOG_DIR="${BEADS_DIR}/diagnostics"
LOG_FILE="${LOG_DIR}/starvation-sync-$(date +%Y%m%d-%H%M%S).log"

mkdir -p "${LOG_DIR}"

log() {
    echo "[$(date -Iseconds)] $*" | tee -a "${LOG_FILE}"
}

error_log() {
    echo "[$(date -Iseconds)] ERROR: $*" | tee -a "${LOG_FILE}" >&2
}

# Check if files exist
if [[ ! -f "${BEADS_DB}" ]]; then
    error_log "beads.db not found at ${BEADS_DB}"
    exit 1
fi

if [[ ! -f "${CHECKPOINT_FILE}" ]]; then
    error_log "checkpoint file not found at ${CHECKPOINT_FILE}"
    exit 1
fi

# Get timestamps
DB_TIMESTAMP=$(stat -c %Y "${BEADS_DB}")
CHECKPOINT_TIMESTAMP=$(stat -c %Y "${CHECKPOINT_FILE}")

log "Checking bead store health..."
log "beads.db timestamp: ${DB_TIMESTAMP}"
log "checkpoint timestamp: ${CHECKPOINT_TIMESTAMP}"

if (( CHECKPOINT_TIMESTAMP > DB_TIMESTAMP )); then
    log "⚠️  Checkpoint is newer than database - potential starvation detected"
    log "Starting recovery sync..."

    # Check if database is empty
    ISSUE_COUNT=$(cd "${WORKSPACE}" && bead list --json 2>/dev/null | jq -s 'length' || echo "0")

    log "Current database issue count: ${ISSUE_COUNT}"

    if (( ISSUE_COUNT == 0 )); then
        log "Database is empty - using restore-into-empty mode"
        SYNC_MODE="--restore-into-empty"
    else
        log "Database has existing issues - using merge mode"
        SYNC_MODE="--merge"
    fi

    # Import from checkpoint
    if cd "${WORKSPACE}" && bead sync import-only \
        --input "${CHECKPOINT_FILE}" \
        ${SYNC_MODE} \
        --actor automated-sync 2>&1 | tee -a "${LOG_FILE}"; then
        log "✅ Checkpoint import completed"
    else
        error_log "❌ Checkpoint import failed"
        exit 1
    fi

    # Verify ready beads
    READY_COUNT=$(cd "${WORKSPACE}" && bead list --ready --json 2>/dev/null | jq -s 'length' || echo "0")
    log "Ready beads after sync: ${READY_COUNT}"

    if (( READY_COUNT == 0 )); then
        error_log "❌ ESCALATION REQUIRED: Still no ready beads after sync"
        log "This indicates a deeper issue - manual intervention required"
        log "Log file: ${LOG_FILE}"
        exit 2
    else
        log "✅ Recovery successful - ${READY_COUNT} ready beads available"
    fi
else
    log "✅ Database is up-to-date - no sync needed"
    READY_COUNT=$(cd "${WORKSPACE}" && bead list --ready --json 2>/dev/null | jq -s 'length' || echo "0")
    log "Current ready beads: ${READY_COUNT}"

    if (( READY_COUNT == 0 )); then
        error_log "⚠️  Database is current but has NO ready beads - possible actual starvation"
        exit 2
    fi
fi

exit 0
