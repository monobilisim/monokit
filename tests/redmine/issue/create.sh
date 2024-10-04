#!/bin/sh
set -e

# Create a new issue
./bin/monokit redmine issue create --message test --service test --subject test

# Get issue ID
./bin/monokit redmine issue show --service test

# Check if issue was created
./bin/monokit redmine issue exists --subject test
