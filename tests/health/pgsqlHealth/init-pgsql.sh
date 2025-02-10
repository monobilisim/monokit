#!/bin/sh
sudo apt update
sudo apt install -y postgresql postgresql-contrib
sudo systemctl start postgresql

sudo -u postgres psql -c "CREATE USER pguser WITH PASSWORD 'pguser';"
sudo -u postgres psql -c "CREATE DATABASE pgdb;"
sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE pgdb TO pguser;"
