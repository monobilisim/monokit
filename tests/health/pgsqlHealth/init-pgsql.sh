#!/bin/sh
sudo apt update
sudo apt install -y postgresql postgresql-contrib
sudo systemctl start postgresql

# Change the password for the default postgres user
sudo -u postgres psql -c "ALTER USER postgres PASSWORD 'test';"
