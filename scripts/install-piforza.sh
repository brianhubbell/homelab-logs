#!/usr/bin/env bash
set -euo pipefail

DEPLOY_DIR="/opt/homelab-services"
REPO_DIR="${DEPLOY_DIR}/homelab-agent"
BINARY="${REPO_DIR}/build/bin/homelab-agent"

# 1. Create dedicated user
if ! id homelab-agent &>/dev/null; then
    useradd -r -s /usr/sbin/nologin homelab-agent
    echo "created homelab-agent user"
fi

# 2. Create deploy dir owned by homelab-agent user
mkdir -p "${DEPLOY_DIR}"
chown homelab-agent:homelab-agent "${DEPLOY_DIR}"

# 3. Clone repo as homelab-agent user (or pull if exists)
if [ -d "${REPO_DIR}/.git" ]; then
    sudo -u homelab-agent git -C "${REPO_DIR}" pull
else
    sudo -u homelab-agent git clone https://github.com/brianhubbell/homelab-agent.git "${REPO_DIR}"
fi

# 4. Build binary in-place
sudo -u homelab-agent bash -c "cd ${REPO_DIR} && CGO_ENABLED=0 go build -ldflags \"-X main.Version=\$(git describe --always)\" -o build/bin/homelab-agent ./cmd/homelab-agent/"
echo "built binary at ${BINARY}"

# 5. Create env file
mkdir -p /etc/homelab-agent
cat > /etc/homelab-agent/env <<'EOF'
MQTT_BROKER=gigantic.lan
TOPIC_PREFIX=agent
SERVICES=bt-to-mqtt,wifi-to-mqtt,homebridge,node_exporter,pm2-piforza
DEPLOY_DIR=/opt/homelab-services
HEALTH_PORT=9110
METRICS_INTERVAL_SECONDS=60
DEBUG=false
EOF
echo "wrote /etc/homelab-agent/env"

# 6. Scoped sudoers for systemctl (managing OTHER services only)
cat > /etc/sudoers.d/homelab-agent <<'EOF'
homelab-agent ALL=(ALL) NOPASSWD: /usr/bin/systemctl start *, /usr/bin/systemctl stop *, /usr/bin/systemctl restart *
EOF
chmod 440 /etc/sudoers.d/homelab-agent
echo "wrote sudoers.d/homelab-agent"

# 7. Install and start service
cp "${REPO_DIR}/scripts/homelab-agent.service" /etc/systemd/system/homelab-agent.service
systemctl daemon-reload
systemctl enable homelab-agent
systemctl start homelab-agent
echo "service started"

systemctl status homelab-agent --no-pager
