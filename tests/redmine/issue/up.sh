#!/bin/sh
set -e

# Close issue using up command
./bin/monokit redmine issue up --message test --service test_updown

# Check if issue was closed
if ./bin/monokit redmine issue exists --subject test_updown; then
  echo "Issue was not closed"
  exit 1
fi
