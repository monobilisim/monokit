#!/bin/sh
set -e

# Create new news
./bin/monokit redmine news create --description test --title test

sleep 5 # Wait for news to be created

# Check if news were created
./bin/monokit redmine news exists --title test --description test > /tmp/news_id # Save news id to file to use in other tests
