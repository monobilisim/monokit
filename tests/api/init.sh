#!/bin/sh
set -e

mkdir -p /etc/mono

cat <<EOF | sudo tee /etc/mono/server.yml
postgres:
    host: localhost
    port: 5432
    user: postgres
    password: test
    dbname: postgres
EOF

cat <<EOF | sudo tee /etc/mono/client.yml
url: http://localhost:9989
EOF

nohup ./bin/monokit server &

ss -tulpn | grep monokit
