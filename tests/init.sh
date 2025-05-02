#!/bin/sh
set -e

export MONOKIT_LOGLEVEL=debug

cd tests 

docker compose up db -d

echo "$REDMINE_TEST_SQL_DUMP" | base64 -d | gzip -d > /tmp/redmine_test.sql

sleep 10 # wait for db to start

# delete existing database
docker compose exec -it db mysql -u root -pexample -e "DROP DATABASE IF EXISTS redmine;"

docker compose exec -it db mysql -u root -pexample -e "CREATE DATABASE redmine;"

docker compose exec -it db mysql -u root -pexample redmine < /tmp/redmine_test.sql

docker compose exec -it db mysql -u root -pexample redmine -e "SELECT * FROM projects;"

docker compose down

docker compose up -d

sudo mkdir -p /etc/mono

cat << EOF | sudo tee /etc/mono/global.yaml
identifier: ci

alarm:
    enabled: false

redmine:
  enabled: true
  interval: 1
  project_id: test-project
  url: "http://localhost:8080"
EOF

echo "  api_key: $REDMINE_TEST_API_KEY" | sudo tee -a /etc/mono/global.yaml

cat /etc/mono/global.yaml

sudo cp ../config/os.yml /etc/mono/os.yml


until curl http://localhost:8080; do
sleep 1
done
