#!/bin/sh
set -e
sudo -u postgres ./bin/monokit pgsqlHealth
