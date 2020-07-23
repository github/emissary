#!/bin/bash

set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
. "$DIR"/common/testlib.sh
. "$DIR"/common/utils.sh


TEST_SUITE="[destructive/envovy+emissary]"

## create a spiffeid that is not in EMISSARY_INGRESS_MAP
docker-compose -f $DIR/../docker-compose.yaml exec spire-server \
  /opt/spire/bin/spire-server entry create \
  -parentID "spiffe://domain.test/node" \
  -spiffeID "spiffe://domain.test/badapp" \
  -selector "unix:uid:0" \
  -ttl 0

docker-compose -f $DIR/../docker-compose.yaml exec emissary \
  spire-agent api fetch jwt -audience "spiffe://domain.test/ingress" \
  -spiffeID "spiffe://domain.test/badapp" > $TRASHDIR/jwt

jwt=$(cat $TRASHDIR/jwt | head -n 2 | tail -n 1 | tr -d '[[:space:]]')

## https://github.com/github/emissary/blob/85659aa2b10a05e4c850368a00f5e85df403183c/pkg/handlers/handlers.go#L157
begin_test "ingress: request from app not in the access list fails"
(
  set -e
  curl_test localhost:8080/get -H "X-Emissary-Auth:bearer $jwt"
  assert_header "403 Forbidden"
  assert_header "x-emissary-auth-status: failure"
)
end_test

TEST_SUITE="[destructive/haproxy+emissary]"

## https://github.com/github/emissary/blob/85659aa2b10a05e4c850368a00f5e85df403183c/pkg/handlers/handlers.go#L157
begin_test "ingress: request from app not in the access list fails"
(
  set -e
  curl_test localhost:7070/get -H "X-Emissary-Auth:bearer $jwt"
  assert_header "403 Forbidden"
  ## haproxy doesnt allow setting x-emissary-auth-status in the 403 response
)
end_test

TEST_SUITE="[destructive/emissary]"

## https://github.com/github/emissary/blob/85659aa2b10a05e4c850368a00f5e85df403183c/pkg/handlers/handlers.go#L157
begin_test "ingress: request from app not in the access list fails"
(
  set -e
  curl_test localhost:9090 -H "x-emissary-mode:ingress" -H "X-Emissary-Auth:bearer $jwt"
  assert_header "403 Forbidden"
  assert_header "X-Emissary-Auth-Status: failure"
)
end_test

docker-compose -f $DIR/../docker-compose.yaml exec emissary killall spire-agent

## https://github.com/github/emissary/blob/cd9ac75ca11a7311667f5de108cfbedaad156282/pkg/handlers/handlers.go#L84
begin_test "ingress: request when spire-agent is down fails"
(
  set -e
  curl_test localhost:9090 -H "x-emissary-mode:ingress" -H "X-Emissary-Auth:bearer asdfasdf"
  assert_header "403 Forbidden"
  assert_header "X-Emissary-Auth-Status: failure"
  docker-compose -f $DIR/../docker-compose.yaml logs emissary | tail -n 5 | grep -q 'svid validation error'
)
end_test

## https://github.com/github/emissary/blob/cd9ac75ca11a7311667f5de108cfbedaad156282/pkg/handlers/handlers.go#L174
begin_test "egress: request when spire-agent is down fails"
(
  set -e
  curl_test localhost:9090 -H "x-emissary-mode:egress" -H "Host:app.domain.test"
  assert_header "403 Forbidden"
  docker-compose -f $DIR/../docker-compose.yaml logs emissary | tail -n 5 | grep -q 'error fetching jwt svid'
)
end_test

## https://github.com/github/emissary/blob/4c3b73c598d533fee246c1a1666f6ed6ff90f853/pkg/handlers/handlers.go#L38
begin_test "make sure emissary health check port is failed after killing spire-agent"
(
  set -e
  curl_test localhost:9191
  assert_header "503 Service Unavailable"
  assert_response "spire-agent is unhealthy"
)
end_test
