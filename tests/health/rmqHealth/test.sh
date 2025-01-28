#!/bin/sh
set -e

export MONOKIT_NOCOLOR=1
./bin/monokit rmqHealth > out.log
cat out.log | grep "rabbitmq-server is active"
cat out.log | grep "Port 5672 is active"

cat out.log | grep "Overview is not reachable"
cat out.log | grep "Node list is not reachable"
