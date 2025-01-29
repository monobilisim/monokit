#!/bin/sh
set -e

MONOKIT_NOCOLOR=1 ./bin/monokit redisHealth > out.log

grep -q "Redis is pingable" out.log
grep -q "Role is master" out.log
grep -q "Service redis-server is active" out.log
grep -q "Redis is writeable" out.log
grep -q "Redis is readable" out.log
