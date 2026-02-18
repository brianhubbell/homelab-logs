#!/usr/bin/env bash
set -euo pipefail

CALLER_HOME=$(eval echo "~${SUDO_USER:-$USER}")

# 1. Create dedicated user
if ! id homelab-agent &>/dev/null; then
    useradd -r -s /usr/sbin/nologin homelab-agent
    echo "created homelab-agent user"
fi

# 2. Install binary
cp "$CALLER_HOME"/homelab-agent /usr/local/bin/homelab-agent
chmod 755 /usr/local/bin/homelab-agent
echo "installed binary"

# 3. Create env file
mkdir -p /etc/homelab-agent
cat > /etc/homelab-agent/env <<'EOF'
MQTT_BROKER=gigantic.lan
TOPIC_PREFIX=agent
ALLOWED_SERVICES=bt-to-mqtt,wifi-to-mqtt,homebridge,node_exporter,pm2-piforza
ALLOWED_COMPOSE_PATHS=
METRICS_PORT=9110
METRICS_INTERVAL_SECONDS=60
DEBUG=false
EOF
echo "wrote /etc/homelab-agent/env"

# 4. Scoped sudoers for systemctl
cat > /etc/sudoers.d/homelab-agent <<'EOF'
homelab-agent ALL=(ALL) NOPASSWD: /usr/bin/systemctl start *, /usr/bin/systemctl stop *, /usr/bin/systemctl restart *
EOF
chmod 440 /etc/sudoers.d/homelab-agent
echo "wrote sudoers.d/homelab-agent"

# 5. Install and start service
cp "$CALLER_HOME"/homelab-agent.service /etc/systemd/system/homelab-agent.service
systemctl daemon-reload
systemctl enable homelab-agent
systemctl start homelab-agent
echo "service started"

systemctl status homelab-agent --no-pager
