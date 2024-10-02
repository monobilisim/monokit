#!/bin/sh
set -e

cd tests 

docker compose up db -d

echo "$REDMINE_TEST_SQL_DUMP" | base64 -d | gzip -d > /tmp/redmine_test.sql

docker compose exec db mysql -u root -pexample -e "$(cat /tmp/redmine_test.sql)"

docker compose up -d

mkdir -p /etc/mono

cat << EOF > /etc/mono/global.yaml
identifier: ci

alarm:
    enabled: false

redmine:
  interval: 1
  project_id: test-project
  url: "http://localhost:8080"
EOF

echo "  api_key: $REDMINE_TEST_API_KEY" >> /etc/mono/global.yaml

cp ../config/os.yml /etc/mono/os.yml
