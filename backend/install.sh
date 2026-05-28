#!/bin/bash
# Run this script ON the Raspberry Pi from /home/pi/backend
set -e

echo "==> Checking .env..."
if [ ! -f .env ]; then
  echo "    !! /home/pi/backend/.env is missing."
  echo "    !! Run from your laptop: scp backend/.env.example pi@<pi>:/home/pi/backend/.env"
  exit 1
fi

echo "==> Making binary executable..."
chmod +x ./backend

echo "==> Writing systemd unit..."
sudo tee /etc/systemd/system/backend.service > /dev/null << 'UNIT'
[Unit]
Description=Pi Device Registry Backend
After=network-online.target
Wants=network-online.target

[Service]
WorkingDirectory=/home/pi/backend
ExecStart=/home/pi/backend/backend
Restart=on-failure
RestartSec=5
User=pi
EnvironmentFile=/home/pi/backend/.env

[Install]
WantedBy=multi-user.target
UNIT

echo "==> Enabling and starting service..."
sudo systemctl daemon-reload
sudo systemctl enable backend
sudo systemctl restart backend

echo "==> Waiting for startup..."
sleep 2

echo "==> Health check:"
curl -sf http://localhost:3000/health && echo ""
echo ""
echo "Done. Backend running at http://$(hostname -I | awk '{print $1}'):3000"
echo "Subscribed to MQTT_TOPIC_PREFIX/+ on MQTT_BROKER (see .env)."
