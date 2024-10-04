#!/bin/sh
set -e

# Update issue with a message
./bin/monokit redmine issue update --service test --message "This is a test message"

# Check if message is sent
./bin/monokit redmine issue existsNote --service test --note "This is a test message"


