#!/bin/sh
set -e

cd tests 

docker compose up -d

echo "$REDMINE_TEST_SQL_DUMP" | base64 -d | gzip -d > /tmp/redmine_test.sql

sleep 10 # wait for db to start

docker compose exec -it db mysql -uroot -pexample redmine -e "$(cat /tmp/redmine_test.sql)"

docker compose down

docker compose up -d

mkdir -p /etc/mono

cat << EOF | sudo tee /etc/mono/global.yaml
identifier: ci

alarm:
    enabled: false

redmine:
  interval: 1
  project_id: test-project
  url: "http://localhost:8080"
EOF

echo "  api_key: $REDMINE_TEST_API_KEY" | sudo tee -a /etc/mono/global.yaml

cat /etc/mono/global.yaml

sudo cp ../config/os.yml /etc/mono/os.yml
