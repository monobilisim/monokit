#!/bin/sh
set -e

sudo mkdir -p /etc/mono
sudo cp ./config/db.yml /etc/mono/db.yml

sudo -u postgres ./bin/monokit pgsqlHealth
