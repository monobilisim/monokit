#!/bin/sh
set -e

sudo mkdir -p /etc/mono
sudo touch /etc/mono/pgsql.yaml
export MONOKIT_NOCOLOR=1

su postgres -c "./bin/monokit pgsqlHealth" > out.log

# Function to check grep and show output on failure
check_grep() {
    if ! cat out.log | grep "$1" > /dev/null; then
        echo "Failed to find: $1"
        echo "Full output:"
        cat out.log
        exit 1
    fi
}

check_grep "Connection.*is Connected"
check_grep "Active Connections.*within limit"
check_grep "PostgreSQL Version.*is.*"
check_grep "Updates.*is Up-to-date"
check_grep "Running Queries.*within limit"
check_grep "Long Running Queries.*is None"
