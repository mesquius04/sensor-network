#!/bin/bash
# Deploy backend to Raspberry Pi.
# Requires: rsync, ssh, sshpass (install with: sudo apt install sshpass  OR  brew install sshpass)
# On Windows: run this from Git Bash or WSL.
set -e

PI_USER=pi
PI_HOST=raspberrypi.local
PI_PASS=upf2019
PI_DIR=/home/pi/backend

echo "==> Syncing backend/ to $PI_USER@$PI_HOST:$PI_DIR ..."
sshpass -p "$PI_PASS" rsync -avz --progress \
  --exclude node_modules \
  --exclude central.db \
  --exclude .env \
  backend/ "$PI_USER@$PI_HOST:$PI_DIR/"

echo "==> Running install.sh on Pi..."
sshpass -p "$PI_PASS" ssh -o StrictHostKeyChecking=no \
  "$PI_USER@$PI_HOST" "cd $PI_DIR && bash install.sh"
