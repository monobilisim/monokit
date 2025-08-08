#!/bin/sh
set -e

# Create new news
./bin/monokit redmine news create --description test --title test

sleep 5 # Wait for news to be created

# Check if news were created (ensure exists returns an ID non-empty)
NEWS_ID=$(./bin/monokit redmine news exists --title test --description test || true)
if [ -z "$NEWS_ID" ]; then
  echo "Failed to create or find the news"
  exit 1
fi
