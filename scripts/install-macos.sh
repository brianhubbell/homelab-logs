#!/usr/bin/env bash
set -euo pipefail

# Usage: sudo bash install-macos.sh <ALLOWED_SERVICES>
# Example: sudo bash install-macos.sh "bt-to-mqtt,govee-to-mqtt,wifi-to-mqtt"

ALLOWED_SERVICES="${1:?Usage: sudo bash install-macos.sh <ALLOWED_SERVICES>}"
CALLER_HOME=$(eval echo "~${SUDO_USER:-$USER}")

# 1. Install binary
cp "$CALLER_HOME"/homelab-agent /usr/local/bin/homelab-agent
chmod 755 /usr/local/bin/homelab-agent
echo "installed binary to /usr/local/bin/homelab-agent"

# 2. Create LaunchDaemon plist
cat > /Library/LaunchDaemons/com.homelab-agent.plist <<PLISTEOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.homelab-agent</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/homelab-agent</string>
    </array>
    <key>EnvironmentVariables</key>
    <dict>
        <key>MQTT_BROKER</key>
        <string>gigantic.lan</string>
        <key>TOPIC_PREFIX</key>
        <string>agent</string>
        <key>ALLOWED_SERVICES</key>
        <string>${ALLOWED_SERVICES}</string>
        <key>METRICS_PORT</key>
        <string>9110</string>
        <key>METRICS_INTERVAL_SECONDS</key>
        <string>60</string>
        <key>DEBUG</key>
        <string>false</string>
    </dict>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/var/log/homelab-agent.log</string>
    <key>StandardErrorPath</key>
    <string>/var/log/homelab-agent.log</string>
</dict>
</plist>
PLISTEOF
echo "wrote /Library/LaunchDaemons/com.homelab-agent.plist"

# 3. Unload if previously loaded, then load
launchctl unload /Library/LaunchDaemons/com.homelab-agent.plist 2>/dev/null || true
launchctl load /Library/LaunchDaemons/com.homelab-agent.plist
echo "service loaded"

sleep 2
launchctl list | grep homelab-agent && echo "homelab-agent is running" || echo "WARNING: homelab-agent not found in launchctl list"
