#!/bin/bash
# Run this script ON the Raspberry Pi from /home/pi/backend
set -e

echo "==> Installing dependencies..."
npm ci --omit=dev

echo "==> Setting up .env..."
if [ ! -f .env ]; then
  cp .env.example .env
  echo "    Created .env from .env.example — edit it if needed."
fi

echo "==> Writing systemd unit..."
sudo tee /etc/systemd/system/backend.service > /dev/null << 'UNIT'
[Unit]
Description=Pi Device Registry Backend
After=network.target

[Service]
WorkingDirectory=/home/pi/backend
ExecStart=/usr/bin/node src/index.js
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
sleep 3

echo "==> Health check:"
curl -sf http://localhost:3000/health && echo ""
echo ""
echo "Done. Backend is running at http://$(hostname -I | awk '{print $1}'):3000"
