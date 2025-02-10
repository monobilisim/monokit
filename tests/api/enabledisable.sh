#!/bin/sh
set -e

MONOKIT_NOCOLOR=1 ./bin/monokit client disable -c osHealth ci

MONOKIT_NOCOLOR=1 ./bin/monokit osHealth | grep -q disable

MONOKIT_NOCOLOR=1 ./bin/monokit client enable -c osHealth ci
MONOKIT_NOCOLOR=1 ./bin/monokit osHealth | grep -q Disk

