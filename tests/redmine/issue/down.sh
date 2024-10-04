#!/bin/sh
set -e

# Create a new issue using down cmd
./bin/monokit redmine issue down --message test --service test --subject test_updown

# Wait for the interval to end
sleep 60

# Try to create issue again as the interval has ended
./bin/monokit redmine issue down --message test --service test --subject test_updown

# Get issue ID
./bin/monokit redmine issue show --service test

# Check if issue was created
./bin/monokit redmine issue exists --subject test_updown


# Update the issue by invoking down cmd again
./bin/monokit redmine issue down --message test2 --service test --subject test_updown

# Check if issue was updated
./bin/monokit redmine issue existsNote --note test2 --service test
