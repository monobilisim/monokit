#!/bin/sh
set -e

# Resolve news ID dynamically (avoid reading from temp file)
NEWS_ID=$(./bin/monokit redmine news exists --title test --description test || true)

if [ -z "$NEWS_ID" ]; then
  echo "Could not resolve news id for deletion"
  exit 1
fi

# Delete the news
./bin/monokit redmine news delete --id $NEWS_ID

# Check if the news is deleted
if ! ./bin/monokit redmine news exists --title test --description test; then
  echo "The news is deleted successfully"
else
  echo "Failed to delete the news"
  exit 1
fi
