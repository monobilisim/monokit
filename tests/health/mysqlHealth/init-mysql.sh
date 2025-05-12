#!/bin/sh
set -e

# Install MySQL server
echo "Installing MySQL server..."
sudo apt update
sudo apt install -y mysql-server

# Start MySQL service
echo "Starting MySQL service..."
sudo systemctl start mysql

# Secure the MySQL installation
echo "Securing MySQL installation..."
# Set root password to 'test_password'
sudo mysql -e "ALTER USER 'root'@'localhost' IDENTIFIED WITH mysql_native_password BY 'test_password';"

# Create test database and user
echo "Setting up test database and user..."
sudo mysql -e "
  CREATE DATABASE IF NOT EXISTS test_db;
  CREATE USER IF NOT EXISTS 'test_user'@'localhost' IDENTIFIED BY 'test_user_password';
  GRANT ALL PRIVILEGES ON test_db.* TO 'test_user'@'localhost';
  FLUSH PRIVILEGES;
"

# Create config file for mysqlHealth
echo "Creating configuration file..."
cat > /tmp/db.yaml << EOF
mysql:
  host: localhost
  port: 3306
  username: test_user
  password: test_user_password
  database: test_db
EOF

echo "MySQL test environment is ready!"
echo "Test configuration is available at /tmp/db.yaml" 