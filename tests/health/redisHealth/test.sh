#!/bin/sh
set -e

MONOKIT_NOCOLOR=1 ./bin/monokit redisHealth > out.log

# Function to check grep and show output on failure
check_grep() {
    if ! cat out.log | grep "$1" > /dev/null; then
        echo "Failed to find: $1"
        echo "Full output:"
        cat out.log
        exit 1
    fi
}

check_grep "Redis Service.*is active"
check_grep "Redis Connection.*is connected"
check_grep "Redis Writeable.*is writeable"
check_grep "Redis Readable.*is readable"
check_grep "Redis Role.*is master"
