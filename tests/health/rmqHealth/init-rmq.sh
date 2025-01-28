#!/usr/bin/env bash
#
# install_rabbitmq_debian.sh
#
# Installs RabbitMQ (and Erlang) on a Debian-based system, then enables
# the management plugin to allow browser-based administration.

set -e

# --- 1. Ensure system is up-to-date and necessary packages are installed ---
echo "[INFO] Updating package list and installing dependencies..."
sudo apt-get update -y
sudo apt-get install -y curl gnupg apt-transport-https lsb-release

# --- 2. Import RabbitMQ repository signing key ---
echo "[INFO] Adding RabbitMQ signing key..."
curl -1sLf 'https://keys.cloudsmith.io/public/rabbitmq/rabbitmq/sign.asc' | sudo apt-key add -

# --- 3. Add the RabbitMQ + Erlang repositories ---
# Detect your Debian release codename (e.g. buster, bullseye)
DIST_CODENAME="$(lsb_release -sc)"
echo "[INFO] Detected codename: $DIST_CODENAME"

# Create /etc/apt/sources.list.d/rabbitmq.list if it doesn't exist
echo "[INFO] Adding RabbitMQ and Erlang repositories to /etc/apt/sources.list.d/rabbitmq.list..."
cat <<EOF | sudo tee /etc/apt/sources.list.d/rabbitmq.list
# Erlang repository
deb https://dl.cloudsmith.io/public/rabbitmq/rabbitmq-erlang/deb/debian $DIST_CODENAME main

# RabbitMQ repository
deb https://dl.cloudsmith.io/public/rabbitmq/rabbitmq-server/deb/debian $DIST_CODENAME main
EOF

# --- 4. Update package list again now that new repos are added ---
echo "[INFO] Updating package list with new repositories..."
sudo apt-get update -y

# --- 5. Install RabbitMQ Server ---
echo "[INFO] Installing rabbitmq-server..."
sudo apt-get install -y rabbitmq-server

# --- 6. Enable and start RabbitMQ service ---
echo "[INFO] Enabling and starting the RabbitMQ service..."
sudo systemctl enable rabbitmq-server
sudo systemctl start rabbitmq-server

# --- 7. (Optional) Enable the RabbitMQ Management Plugin ---
echo "[INFO] Enabling the RabbitMQ management plugin..."
sudo rabbitmq-plugins enable rabbitmq_management

# --- 8. Confirm status ---
sudo systemctl status rabbitmq-server --no-pager
