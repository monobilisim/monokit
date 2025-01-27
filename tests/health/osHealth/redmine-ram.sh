#!/bin/sh
set -e

# Run it first
./bin/monokit osHealth

# Change the ram_limit to 1
sed -i 's/ram_limit: 91/ram_limit: 1/g' /etc/mono/os.yml

# Run it again
./bin/monokit osHealth

# Wait a minute
sleep 60

# Run it again
./bin/monokit osHealth

# Check if Redmine issue is created
ISSUE_ID="$(cat /tmp/mono/osHealth/ram-redmine.log || true)"

if [ -z "$ISSUE_ID" ]; then
  echo "Redmine issue is not created"
  exit 1
fi

# Change the ram_limit to 91
sed -i 's/ram_limit: 1/ram_limit: 91/g' /etc/mono/os.yml

# Run it again
./bin/monokit osHealth

# Check if Redmine issue is closed
ISSUE_ID="$(cat /tmp/mono/osHealth/ram-redmine.log || true)"

if [ -n "$ISSUE_ID" ]; then
  echo "Redmine issue is not closed"
  exit 1
fi
