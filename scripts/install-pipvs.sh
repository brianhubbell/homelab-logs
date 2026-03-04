#!/usr/bin/env bash
set -euo pipefail

DEPLOY_DIR="/opt/homelab-services"
REPO_DIR="${DEPLOY_DIR}/homelab-logs"
BINARY="${REPO_DIR}/build/bin/homelab-logs"

# 1. Create dedicated user
if ! id homelab-logs &>/dev/null; then
    useradd -r -s /usr/sbin/nologin homelab-logs
    echo "created homelab-logs user"
fi

# 2. Create deploy dir owned by homelab-logs user
mkdir -p "${DEPLOY_DIR}"
chown homelab-logs:homelab-logs "${DEPLOY_DIR}"

# 3. Clone repo as homelab-logs user (or pull if exists)
if [ -d "${REPO_DIR}/.git" ]; then
    sudo -u homelab-logs git -C "${REPO_DIR}" pull
else
    sudo -u homelab-logs git clone https://github.com/brianhubbell/homelab-logs.git "${REPO_DIR}"
fi

# 4. Build binary in-place
sudo -u homelab-logs bash -c "cd ${REPO_DIR} && CGO_ENABLED=0 go build -ldflags \"-X main.Version=\$(git describe --always)\" -o build/bin/homelab-logs ./cmd/homelab-logs/"
echo "built binary at ${BINARY}"

# 5. Create env file
mkdir -p /etc/homelab-logs
cat > /etc/homelab-logs/env <<'EOF'
MQTT_BROKER=gigantic.lan
TOPIC_PREFIX=agent
JOURNAL_UNIT=homelab-logs
DEBUG=false
EOF
echo "wrote /etc/homelab-logs/env"

# 6. Install and start service
cp "${REPO_DIR}/scripts/homelab-logs.service" /etc/systemd/system/homelab-logs.service
systemctl daemon-reload
systemctl enable homelab-logs
systemctl start homelab-logs
echo "service started"

systemctl status homelab-logs --no-pager
