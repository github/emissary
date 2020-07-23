#!/bin/bash

set -e

docker-compose rm -f

docker-compose up -d spire-server

docker-compose exec spire-server \
  /opt/spire/bin/spire-server bundle show > conf/agent/bootstrap.crt

TOKEN=$(docker-compose exec spire-server \
  /opt/spire/bin/spire-server token generate -spiffeID spiffe://domain.test/node \
  | awk '{print $2}' | tr -d '\r')

sed "s#TOKEN#${TOKEN}#g" conf/agent/agent.conf.orig > conf/agent/agent.conf

docker-compose up -d emissary
docker-compose up -d envoy
docker-compose up -d haproxy
docker-compose up -d app_container

for test in $(find tests/*base*.sh); do
  bash $test
done

docker-compose exec spire-server \
  /opt/spire/bin/spire-server entry create \
  -parentID "spiffe://domain.test/node" \
  -spiffeID "spiffe://domain.test/app" \
  -selector "unix:uid:0" \
  -ttl 0

docker-compose exec spire-server \
  /opt/spire/bin/spire-server entry create \
  -parentID "spiffe://domain.test/node" \
  -spiffeID "spiffe://domain.test/ingress" \
  -selector "unix:uid:0" \
  -ttl 0

retry=10
while [ -n "$retry" ] && [ "$retry" -gt 0 ]; do
  docker-compose exec emissary \
    spire-agent api fetch jwt \
    -audience "spiffe://domain.test/app" \
    && retry= || retry=$((retry-1))
  sleep 1
  [ -z "$retry" ] || echo "retrying jwt fetch ..."
done

# do the non-base and non-destructive tests
for test in $(find tests -path tests/*base*.sh -prune -o -path tests/*destructive*.sh -prune -o -name '*.sh' -print); do
  bash $test
done

for test in $(find tests/*destructive*.sh); do
  bash $test
done
