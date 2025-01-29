#!/bin/sh
set -e


sudo mkdir -p /etc/mono
sudo cp ./config/db.yml /etc/mono/db.yml

sudo -u postgres /bin/sh -c "./bin/monokit pgsqlHealth" > out.log

echo "=== Output ==="
cat out.log
echo "=== End of Output ==="

grep -q "PostgreSQL is accessible" out.log
grep -q "PostgreSQL uptime is 0d" out.log
grep -q "Number of active connections is 6/100" out.log
grep -q "Number of running queries is 1/25" out.log
