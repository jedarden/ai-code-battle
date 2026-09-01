#!/usr/bin/env bash
# Install the bead starvation sync systemd timer

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKSPACE="${SCRIPT_DIR}/.."

SERVICE_NAME="bead-starvation-sync"
SERVICE_FILE="${SCRIPT_DIR}/${SERVICE_NAME}.service"
TIMER_FILE="${SCRIPT_DIR}/${SERVICE_NAME}.timer"

echo "Installing ${SERVICE_NAME} systemd timer..."

# Check if running as root (for system-wide install) or as user (for user unit)
if [[ $EUID -eq 0 ]]; then
    # System-wide installation
    echo "Installing system-wide systemd units..."
    cp "${SERVICE_FILE}" "/etc/systemd/system/${SERVICE_NAME}.service"
    cp "${TIMER_FILE}" "/etc/systemd/system/${SERVICE_NAME}.timer"
    systemctl daemon-reload
    systemctl enable "${SERVICE_NAME}.timer"
    systemctl start "${SERVICE_NAME}.timer"
    echo "✅ System-wide timer installed and started"
else
    # User-level installation
    echo "Installing user systemd units..."
    USER_SYSTEMD_DIR="${HOME}/.config/systemd/user"
    mkdir -p "${USER_SYSTEMD_DIR}"
    cp "${SERVICE_FILE}" "${USER_SYSTEMD_DIR}/${SERVICE_NAME}.service"
    cp "${TIMER_FILE}" "${USER_SYSTEMD_DIR}/${SERVICE_NAME}.timer"

    # Reload and enable
    systemctl --user daemon-reload
    systemctl --user enable "${SERVICE_NAME}.timer"
    systemctl --user start "${SERVICE_NAME}.timer"
    echo "✅ User timer installed and started"
fi

echo ""
echo "To check status:"
if [[ $EUID -eq 0 ]]; then
    echo "  systemctl status ${SERVICE_NAME}.timer"
    echo "  systemctl list-timers | grep ${SERVICE_NAME}"
else
    echo "  systemctl --user status ${SERVICE_NAME}.timer"
    echo "  systemctl --user list-timers | grep ${SERVICE_NAME}"
fi

echo ""
echo "To view logs:"
if [[ $EUID -eq 0 ]]; then
    echo "  journalctl -u ${SERVICE_NAME}.service -f"
else
    echo "  journalctl --user -u ${SERVICE_NAME}.service -f"
fi

echo ""
echo "To stop the timer:"
if [[ $EUID -eq 0 ]]; then
    echo "  systemctl stop ${SERVICE_NAME}.timer"
    echo "  systemctl disable ${SERVICE_NAME}.timer"
else
    echo "  systemctl --user stop ${SERVICE_NAME}.timer"
    echo "  systemctl --user disable ${SERVICE_NAME}.timer"
fi
