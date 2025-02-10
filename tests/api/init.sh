#!/bin/sh
set -e

mkdir -p /etc/mono

cat <<EOF | sudo tee /etc/mono/server.yml
postgres:
    host: localhost
    port: 5432
    user: pguser
    password: pguser
    dbname: pgdb
EOF

cat <<EOF | sudo tee /etc/mono/client.yml
url: http://localhost:9989
EOF

nohup ./bin/monokit server &
