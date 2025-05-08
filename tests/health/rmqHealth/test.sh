#!/bin/sh
set -e

export MONOKIT_NOCOLOR=1
./bin/monokit rmqHealth > out.log

# Function to check grep and show output on failure
check_grep() {
    if ! cat out.log | grep "$1" > /dev/null; then
        echo "Failed to find: $1"
        echo "Full output:"
        cat out.log
        exit 1
    fi
}

check_grep "RabbitMQ Service.*active"
check_grep "AMQP Port (5672).*open"
check_grep "Overview API.*reachable"
check_grep "RabbitMQ cluster node list is now reachable"
