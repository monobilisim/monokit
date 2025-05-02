#!/bin/sh
set -e

export MONOKIT_LOGLEVEL=debug

# Run it first
./bin/monokit osHealth

# Change the part_use_limit to 1
sed -i 's/part_use_limit: 90/part_use_limit: 1/g' /etc/mono/os.yml

# Run it again
./bin/monokit osHealth

# Wait a minute
sleep 60

# Run it again
./bin/monokit osHealth

# Check if Redmine issue is created
ISSUE_ID="$(cat /tmp/mono/osHealth/disk-redmine.log || true)"

if [ -z "$ISSUE_ID" ]; then
  echo "Redmine issue is not created"
  exit 1
fi

# Change the part_use_limit to 90
sed -i 's/part_use_limit: 1/part_use_limit: 90/g' /etc/mono/os.yml

# Run it again
./bin/monokit osHealth

# Check if Redmine issue is closed
ISSUE_ID="$(cat /tmp/mono/osHealth/disk-redmine.log || true)"

if [ -n "$ISSUE_ID" ]; then
  echo "Redmine issue is not closed"
  exit 1
fi
