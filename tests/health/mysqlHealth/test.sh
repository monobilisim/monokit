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

check_grep "MySQL Service.*is active"
check_grep "MySQL Connection.*is connected"
check_grep "MySQL Writeable.*is writeable"
check_grep "MySQL Readable.*is readable"

echo "All MySQL health checks passed!" 