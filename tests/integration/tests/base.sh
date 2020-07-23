#!/bin/bash

set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
. "$DIR"/common/testlib.sh
. "$DIR"/common/utils.sh

TEST_SUITE="[base]"

begin_test "make sure envoy is running"
(
  set -e
  curl_test localhost:9901/ready
  assert_response "LIVE"
)
end_test

begin_test "make sure haproxy is running"
(
  set -e
  curl_test localhost:7070/_health_check
  assert_response "Service ready."
)
end_test

begin_test "make sure app is running"
(
  set -e
  curl_test localhost:8000/get
  assert_header "200 OK"
)
end_test

begin_test "make sure emissary is running"
(
  set -e
  curl_test localhost:9090
  assert_header "412 Precondition Failed"
)
end_test

TEST_SUITE="[pre-registration]"

## health check port should fail pre-reg
## https://github.com/github/emissary/blob/4c3b73c598d533fee246c1a1666f6ed6ff90f853/pkg/handlers/handlers.go#L51
begin_test "make sure emissary health check port is failing"
(
  set -e
  curl_test localhost:9191
  assert_header "503 Service Unavailable"
)
end_test

## happy path, but this should fail before registration
## https://github.com/github/emissary/blob/cd9ac75ca11a7311667f5de108cfbedaad156282/pkg/handlers/handlers.go#L186
begin_test "egress: envoy egress request fails"
(
  set -e
  curl_test localhost:18000/get -H "Host:app.domain.test"
  assert_header "403 Forbidden"
)
end_test

## happy path, but this should fail before registration
## https://github.com/github/emissary/blob/cd9ac75ca11a7311667f5de108cfbedaad156282/pkg/handlers/handlers.go#L186
begin_test "egress: haproxy egress request fails"
(
  set -e
  curl_test localhost:17000/get -H "Host:app.domain.test"
  assert_header "403 Forbidden"
)
end_test

## happy path, but this should fail before registration
## https://github.com/github/emissary/blob/cd9ac75ca11a7311667f5de108cfbedaad156282/pkg/handlers/handlers.go#L186
begin_test "egress: emissary egress request fails"
(
  set -e
  curl_test localhost:9090/get -H 'x-emissary-mode:egress' -H "Host:app.domain.test"
  assert_header "403 Forbidden"
)
end_test
