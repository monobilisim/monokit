#!/bin/sh
set -e

NEWS_ID=$(cat /tmp/news_id)

# Delete the news
./bin/monokit redmine news delete --id $NEWS_ID

# Check if the news is deleted
if ! ./bin/monokit redmine news exists --title test --description test; then
  echo "The news is deleted successfully"
else
  echo "Failed to delete the news"
  exit 1
fi
