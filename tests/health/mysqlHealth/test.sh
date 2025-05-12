#!/bin/sh
set -e

# Run MySQL health check
MONOKIT_NOCOLOR=1 ./bin/monokit mysqlHealth > out.log

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
check_grep "Process Count.*within limit"
check_grep "Waiting Processes.*within limit"
check_grep "PMM Status.*is Inactive"

echo "All MySQL health checks passed!" 