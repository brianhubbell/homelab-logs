#!/usr/bin/env bash
set -euo pipefail

# Usage: bash install-macos.sh <DEPLOY_DIR> [SERVICES]
# Example: bash install-macos.sh /Volumes/dev "bt-to-mqtt,wifi-to-mqtt"
# No sudo required — installs as a user-level LaunchAgent.

DEPLOY_DIR="${1:?Usage: bash install-macos.sh <DEPLOY_DIR> [SERVICES]}"
SERVICES="${2:-}"

REPO_DIR="${DEPLOY_DIR}/homelab-agent"
BINARY="${REPO_DIR}/build/bin/homelab-agent"
PLIST_TEMPLATE="${REPO_DIR}/scripts/com.homelab-agent.plist"
PLIST_DEST="${HOME}/Library/LaunchAgents/com.homelab-agent.plist"
DOMAIN="gui/$(id -u)"

# 1. Verify repo exists
if [ ! -d "${REPO_DIR}/.git" ]; then
    echo "ERROR: repo not found at ${REPO_DIR} — clone it first"
    exit 1
fi

# 2. Build if binary doesn't exist
if [ ! -f "${BINARY}" ]; then
    echo "building homelab-agent..."
    cd "${REPO_DIR}"
    make build
fi

# 3. Ensure logs directory exists
mkdir -p "${HOME}/Library/Logs"

# 4. Substitute plist template
mkdir -p "${HOME}/Library/LaunchAgents"
sed \
    -e "s|__BINARY_PATH__|${BINARY}|g" \
    -e "s|__SERVICES__|${SERVICES}|g" \
    -e "s|__DEPLOY_DIR__|${DEPLOY_DIR}|g" \
    -e "s|__HOME__|${HOME}|g" \
    "${PLIST_TEMPLATE}" > "${PLIST_DEST}"
echo "wrote ${PLIST_DEST}"

# 5. Unload if previously loaded, then load
launchctl bootout "${DOMAIN}/com.homelab-agent" 2>/dev/null || true
launchctl bootstrap "${DOMAIN}" "${PLIST_DEST}"
echo "service loaded"

sleep 2
launchctl list | grep homelab-agent && echo "homelab-agent is running" || echo "WARNING: homelab-agent not found in launchctl list"
