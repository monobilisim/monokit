#!/bin/sh
set -e

# Create a MySQL config file in home directory
echo "Creating MySQL configuration file..."
cat > ~/.my.cnf << EOF
[client]
user=root
password=example
host=localhost
port=3306
EOF

# Set proper permissions
chmod 600 ~/.my.cnf

echo "MySQL test environment is ready!"
echo "MySQL configuration available at ~/.my.cnf" 