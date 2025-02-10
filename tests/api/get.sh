#!/bin/sh
set -e

MONOKIT_NOCOLOR=1 ./bin/monokit client get | grep Disabled


