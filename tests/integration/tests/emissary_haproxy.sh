#!/bin/bash

set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
. "$DIR"/common/testlib.sh
. "$DIR"/common/utils.sh


TEST_SUITE="[haproxy+emissary]"

## happy path
## https://github.com/github/emissary/blob/cd9ac75ca11a7311667f5de108cfbedaad156282/pkg/handlers/handlers.go#L186
begin_test "egress: request contains JWT-SVID"
(
  set -e
  curl_test localhost:17000/get -H "Host:app.domain.test"
  assert_header "200 OK"
  assert_response "X-Emissary-Auth"
  assert_response "bearer"
)
end_test

## make sure envoy passes path and method
begin_test "egress: make sure path and method get passed to emissary"
(
  set -e
  curl_test localhost:17000/put -H "Host:app.domain.test" -X PUT -d '{"chonkyegresshaproxy":123}'
  assert_header "200 OK"
  assert_response "X-Emissary-Auth"
  assert_response "bearer"
  docker-compose -f $DIR/../docker-compose.yaml logs emissary \
    | tail -n 10 | grep 'request received' | grep -q 'path=/put'
  docker-compose -f $DIR/../docker-compose.yaml logs emissary \
    | tail -n 10 | grep 'request received' | grep -q 'method=PUT'
  docker-compose -f $DIR/../docker-compose.yaml logs emissary \
    | tail -n 10 | grep 'request received' | grep -q 'mode_header=egress'
  assert_response "chonkyegresshaproxy"
)
end_test

## envoy does this
## https://github.com/github/emissary/blob/e2e/tests/integration/conf/envoy/envoy.yaml#L90
begin_test "egress: cant spoof mode header over egress port"
(
  set -e
  curl_test localhost:17000/get -H "x-emissary-mode:asdfadf" -H "Host:app.domain.test"
  assert_header "200 OK"
  assert_response "X-Emissary-Auth"
  assert_response "bearer"
)
end_test

## https://github.com/github/emissary/blob/cd9ac75ca11a7311667f5de108cfbedaad156282/pkg/handlers/handlers.go#L150
begin_test "egress: request without a host header that match fails"
(
  set -e
  curl_test localhost:17000/get -H "Host:blah"
  assert_header "403 Forbidden"
)
end_test

## https://github.com/github/emissary/blob/cd9ac75ca11a7311667f5de108cfbedaad156282/pkg/handlers/handlers.go#L132
begin_test "egress: request without a host header fails"
(
  set -e
  curl_test localhost:17000/get
  assert_header "403 Forbidden"
)
end_test

## https://github.com/github/emissary/blob/cd9ac75ca11a7311667f5de108cfbedaad156282/pkg/handlers/handlers.go#L122
begin_test "egress: request with auth header already fails"
(
  set -e
  curl_test localhost:17000/get -H "X-Emissary-Auth:bearer fakeheader"
  assert_header "403 Forbidden"
)
end_test

## envoy does this
## https://github.com/github/emissary/blob/e2e/tests/integration/conf/envoy/envoy.yaml#L31-L39
begin_test "ingress: request without X-Emissary-Auth header does not require auth"
(
  set -e
  curl_test localhost:7070/get
  assert_header "200 OK"
)
end_test

## https://github.com/github/emissary/blob/cd9ac75ca11a7311667f5de108cfbedaad156282/pkg/handlers/handlers.go#L62
begin_test "ingress: request with bad X-Emissary-Auth fails"
(
  set -e
  curl_test localhost:7070/get -H "X-Emissary-Auth:blah"
  assert_header "403 Forbidden"
  ## haproxy doesnt allow setting x-emissary-auth-status in the 403 response
)
end_test

docker-compose -f $DIR/../docker-compose.yaml exec emissary \
  spire-agent api fetch jwt -audience "spiffe://domain.test/ingress" -spiffeID "spiffe://domain.test/app"> $TRASHDIR/jwt

jwt=$(cat $TRASHDIR/jwt | head -n 2 | tail -n 1 | tr -d '[[:space:]]')

## happy path
## https://github.com/github/emissary/blob/cd9ac75ca11a7311667f5de108cfbedaad156282/pkg/handlers/handlers.go#L107
## {"path":"/g","methods":["GET"]}
begin_test "ingress: GET request with good X-Emissary-Auth succeeds"
(
  set -e
  curl_test localhost:7070/get -H "X-Emissary-Auth:bearer $jwt"
  assert_header "200 OK"
  assert_response "$jwt"
  assert_response '"X-Emissary-Auth-Status": "success"'

  docker-compose -f $DIR/../docker-compose.yaml logs emissary \
    | tail -n 5 | grep 'request accepted' | grep -q 'acl_index=2'
)
end_test

## matches this rule in the emissary config:
## {"path":"/p","methods":["PATCH"]}]}
begin_test "ingress: PATCH request with good X-Emissary-Auth succeeds"
(
  set -e
  curl_test localhost:7070/patch -X PATCH -H "X-Emissary-Auth:bearer $jwt" -d '{"test":123}'
  assert_header "200 OK"
  assert_response "$jwt"
  assert_response '"X-Emissary-Auth-Status": "success"'

  docker-compose -f $DIR/../docker-compose.yaml logs emissary \
    | tail -n 5 | grep 'request accepted' | grep -q 'acl_index=1'
)
end_test

## because this is a GET to /patch it matches the path but not the method
## {"path":"/p","methods":["PATCH"]}]}
begin_test "ingress: GET request with good X-Emissary-Auth but bad path fails"
(
  set -e
  curl_test localhost:7070/patch -H "X-Emissary-Auth:bearer $jwt"
  assert_header "403 Forbidden"
  ## haproxy doesnt allow setting x-emissary-auth-status in the 403 response
)
end_test

## matches this rule in the emissary config:
## "path":"/put","methods":["PUT"]}
begin_test "ingress: PUT request with good X-Emissary-Auth succeeds"
(
  set -e
  curl_test localhost:7070/put -X PUT -H "X-Emissary-Auth:bearer $jwt" -d '{"test":123}'
  assert_header "200 OK"
  assert_response "$jwt"
  assert_response '"X-Emissary-Auth-Status": "success"'

  docker-compose -f $DIR/../docker-compose.yaml logs emissary \
    | tail -n 5 | grep 'request accepted' | grep -q 'acl_index=0'
)
end_test

begin_test "ingress: request with good X-Emissary-Auth fails with bad path"
(
  set -e
  curl_test localhost:7070/nomatch -H "X-Emissary-Auth:bearer $jwt"
  assert_header "403 Forbidden"
  ## haproxy doesnt allow setting x-emissary-auth-status in the 403 response
)
end_test

begin_test "ingress: request with good X-Emissary-Auth fails with bad method"
(
  set -e
  curl_test localhost:7070/get -X PUT -d '{}' -H "X-Emissary-Auth:bearer $jwt"
  assert_header "403 Forbidden"
  ## haproxy doesnt allow setting x-emissary-auth-status in the 403 response
)
end_test

## make sure envoy passes path and method
begin_test "ingress: make sure path and method get passed to emissary"
(
  set -e
  curl_test localhost:7070/put -H "X-Emissary-Auth:bearer $jwt" -X PUT -d '{"chonkyingresshaproxy":123}'
  assert_header "200 OK"
  assert_response '"X-Emissary-Auth-Status": "success"'
  assert_response "$jwt"
  assert_response "chonkyingresshaproxy"

  docker-compose -f $DIR/../docker-compose.yaml logs emissary \
    | tail -n 10 | grep 'request received' | grep -q 'path=/put'
  docker-compose -f $DIR/../docker-compose.yaml logs emissary \
    | tail -n 10 | grep 'request received' | grep -q 'method=PUT'
  docker-compose -f $DIR/../docker-compose.yaml logs emissary \
    | tail -n 10 | grep 'request received' | grep -q 'mode_header=ingress'
  docker-compose -f $DIR/../docker-compose.yaml logs emissary \
    | tail -n 10 | grep 'request accepted' | grep -q 'acl_index=0'
)
end_test

docker-compose -f $DIR/../docker-compose.yaml exec emissary \
  spire-agent api fetch jwt -audience "spiffe://domain.test/ingress" -spiffeID "spiffe://domain.test/app"> $TRASHDIR/jwt

jwt=$(cat $TRASHDIR/jwt | head -n 2 | tail -n 1 | tr -d '[[:space:]]')

## envoy does this
## https://github.com/github/emissary/blob/e2e/tests/integration/conf/envoy/envoy.yaml#L52
begin_test "ingress: request with good X-Emissary-Auth succeeds with spoofed mode header"
(
  set -e
  curl_test localhost:7070/get -H "x-emissary-mode:asdfadf" -H "X-Emissary-Auth:bearer $jwt"
  assert_header "200 OK"
  assert_response '"X-Emissary-Auth-Status": "success"'
  assert_response "$jwt"

  docker-compose -f $DIR/../docker-compose.yaml logs emissary \
    | tail -n 5 | grep 'request accepted' | grep -q 'acl_index=2'
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
  curl_test localhost:7070/get -H "X-Emissary-Auth:bearer $jwt"
  assert_header "403 Forbidden"
  ## haproxy doesnt allow setting x-emissary-auth-status in the 403 response
)
end_test
