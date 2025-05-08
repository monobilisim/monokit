#!/bin/sh
set -e

export MONOKIT_NOCOLOR=1
./bin/monokit rmqHealth > out.log
cat out.log | grep "RabbitMQ Service.*active"
cat out.log | grep "AMQP Port (5672).*open"

cat out.log | grep "Overview API.*reachable"
cat out.log | grep "RabbitMQ cluster node list is now reachable"
