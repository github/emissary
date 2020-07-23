#!/bin/bash

set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
. "$DIR"/common/testlib.sh
. "$DIR"/common/utils.sh

TEST_SUITE="[emissary]"

## https://github.com/github/emissary/blob/4c3b73c598d533fee246c1a1666f6ed6ff90f853/pkg/handlers/handlers.go#L51
begin_test "make sure emissary health check port is running"
(
  set -e
  curl_test localhost:9191
  assert_header "200 OK"
  assert_response "emissary and spire-agent are live"
)
end_test

## https://github.com/github/emissary/blob/cd9ac75ca11a7311667f5de108cfbedaad156282/pkg/handlers/handlers.go#L45
begin_test "request without a mode header fails"
(
  set -e
  curl_test localhost:9090 -H "Host:blah"
  assert_header "412 Precondition Failed"
)
end_test

## https://github.com/github/emissary/blob/cd9ac75ca11a7311667f5de108cfbedaad156282/pkg/handlers/handlers.go#L45
begin_test "request with a bad mode header fails"
(
  set -e
  curl_test localhost:9090 -H "x-emissary-mode:asdfadf"
  assert_header "412 Precondition Failed"
)
end_test

## https://github.com/github/emissary/blob/cd9ac75ca11a7311667f5de108cfbedaad156282/pkg/handlers/handlers.go#L132
begin_test "egress: request without a host header fails"
(
  set -e
  curl_test localhost:9090 -H "x-emissary-mode:egress"
  assert_header "403 Forbidden"
)
end_test

## https://github.com/github/emissary/blob/cd9ac75ca11a7311667f5de108cfbedaad156282/pkg/handlers/handlers.go#L150
begin_test "egress: request without a host header match fails"
(
  set -e
  curl_test localhost:9090 -H "x-emissary-mode:egress" -H "Host:blah"
  assert_header "403 Forbidden"
)
end_test

## https://github.com/github/emissary/blob/cd9ac75ca11a7311667f5de108cfbedaad156282/pkg/handlers/handlers.go#L45
begin_test "egress: request with a host header match but bad mode fails"
(
  set -e
  curl_test localhost:9090 -H "x-emissary-mode:asdfasdf" -H "Host:app.domain.test"
  assert_header "412 Precondition Failed"
)
end_test

## happy path
## https://github.com/github/emissary/blob/cd9ac75ca11a7311667f5de108cfbedaad156282/pkg/handlers/handlers.go#L186
begin_test "egress: request with a host header match succeeds"
(
  set -e
  curl_test localhost:9090 -H "x-emissary-mode:egress" -H "Host:app.domain.test"
  assert_header "200 OK"
  assert_header "X-Emissary-Auth"
  assert_header "bearer"
)
end_test

## https://github.com/github/emissary/blob/cd9ac75ca11a7311667f5de108cfbedaad156282/pkg/handlers/handlers.go#L122
begin_test "egress: request with auth header already fails"
(
  set -e
  curl_test localhost:9090/get -H "x-emissary-mode:egress" -H "X-Emissary-Auth:bearer fakeheader"
  assert_header "403 Forbidden"
)
end_test

## https://github.com/github/emissary/blob/cd9ac75ca11a7311667f5de108cfbedaad156282/pkg/handlers/handlers.go#L62
begin_test "ingress: request with bad auth header fails"
(
  set -e
  curl_test localhost:9090 -H "x-emissary-mode:ingress" -H "X-Emissary-Auth:fakeheader"
  assert_header "403 Forbidden"
  assert_header "X-Emissary-Auth-Status: failure"
)
end_test

## https://github.com/github/emissary/blob/cd9ac75ca11a7311667f5de108cfbedaad156282/pkg/handlers/handlers.go#L62
begin_test "ingress: request with bad auth header fails"
(
  set -e
  curl_test localhost:9090 -H "x-emissary-mode:ingress" -H "X-Emissary-Auth:bearer fakeheader"
  assert_header "403 Forbidden"
  assert_header "X-Emissary-Auth-Status: failure"
)
end_test

docker-compose -f $DIR/../docker-compose.yaml exec emissary \
  spire-agent api fetch jwt -audience "spiffe://domain.test/ingress" -spiffeID "spiffe://domain.test/app"> $TRASHDIR/jwt

jwt=$(cat $TRASHDIR/jwt | head -n 2 | tail -n 1 | tr -d '[[:space:]]')

## happy path
## https://github.com/github/emissary/blob/cd9ac75ca11a7311667f5de108cfbedaad156282/pkg/handlers/handlers.go#L107
begin_test "ingress: request with good X-Emissary-Auth succeeds"
(
  set -e
  curl_test localhost:9090/get -H "x-emissary-mode:ingress" -H "X-Emissary-Auth:bearer $jwt"
  assert_header "200 OK"
  assert_header "$jwt"
  assert_header "X-Emissary-Auth-Status: success"
)
end_test

begin_test "ingress: request with good X-Emissary-Auth but bad path/method fails"
(
  set -e
  curl_test localhost:9090/patch -H "x-emissary-mode:ingress" -H "X-Emissary-Auth:bearer $jwt"
  assert_header "403 Forbidden"
  assert_header "X-Emissary-Auth-Status: failure"
)
end_test

## https://github.com/github/emissary/blob/cd9ac75ca11a7311667f5de108cfbedaad156282/pkg/handlers/handlers.go#L45
begin_test "ingress: request with good X-Emissary-Auth but bad mode fails"
(
  set -e
  curl_test localhost:9090 -H "x-emissary-mode:asdfasdf" -H "X-Emissary-Auth:bearer $jwt"
  assert_header "412 Precondition Failed"
)
end_test

docker-compose -f $DIR/../docker-compose.yaml exec emissary \
  spire-agent api fetch jwt -audience "spiffe://domain.test/badaudience" -spiffeID "spiffe://domain.test/app"> $TRASHDIR/jwt

jwt=$(cat $TRASHDIR/jwt | head -n 2 | tail -n 1 | tr -d '[[:space:]]')

## https://github.com/github/emissary/blob/cd9ac75ca11a7311667f5de108cfbedaad156282/pkg/handlers/handlers.go#L84
## https://github.com/github/emissary/blob/cd9ac75ca11a7311667f5de108cfbedaad156282/pkg/spire/spire.go#L54
begin_test "ingress: request with bad audience fails"
(
  set -e
  curl_test localhost:9090 -H "x-emissary-mode:ingress" -H "X-Emissary-Auth:bearer $jwt"
  assert_header "403 Forbidden"
  assert_header "X-Emissary-Auth-Status: failure"
)
end_test
