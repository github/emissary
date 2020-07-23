#!/bin/bash

curl_test () {
  curl -sv \
  --user-agent "emissary-test" \
  "$@" \
  -o "$TRASHDIR/response" 2> "$TRASHDIR/headers"
}

assert_header () {
  grep "$@" "$TRASHDIR/headers"
}

retrieve_header () {
  grep "$@" "$TRASHDIR/headers" | tail -1 | sed 's/.*: //' | tr -d '\r\n'
}

refute_header () {
  grep -v "$@" "$TRASHDIR/headers"
}

refute_response () {
  grep -v "$@" "$TRASHDIR/response"
}

assert_response () {
  grep -a "$@" "$TRASHDIR/response"
}
