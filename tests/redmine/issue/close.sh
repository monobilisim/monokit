#!/bin/sh
set -e

ISSUE_ID=$(cat /tmp/mono/test-redmine.log)

if [ "$ISSUE_ID" = "0" ] || [ -z "$ISSUE_ID" ]; then
  echo "Issue not found"
  exit 1
fi

./bin/monokit redmine issue close --service test --message test

if ./bin/monokit redmine issue exists -j test; then
  echo "Issue not closed"
  exit 1
fi

