#!/bin/sh
set -e

mkdir -p /etc/mono

sudo systemctl stop postgresql
sudo apt remove -y postgresql

docker run -d --name postgres -p 5432:5432 -e POSTGRES_PASSWORD=test -e POSTGRES_DB=postgres -e POSTGRES_USER=postgres postgres:alpine

sleep 10

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

