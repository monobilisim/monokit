#!/bin/sh
set -e

# Create a new project
./bin/monokit redmine issue create --message test --service test --subject test

# Check if project was created
./bin/monokit redmine issue exists --subject test
