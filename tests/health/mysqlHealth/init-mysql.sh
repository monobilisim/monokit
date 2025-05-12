#!/bin/sh
set -e

# Install MySQL server
echo "Installing MySQL server..."
sudo apt update
sudo apt install -y mysql-server

# Start MySQL service
echo "Starting MySQL service..."
sudo systemctl start mysql

# Create a MySQL config file in home directory
echo "Creating MySQL configuration file..."
cat > ~/.my.cnf << EOF
[client]
user=root
socket=/var/run/mysqld/mysqld.sock
EOF

# Set proper permissions
chmod 600 ~/.my.cnf

echo "MySQL test environment is ready!"
echo "MySQL configuration available at ~/.my.cnf" 