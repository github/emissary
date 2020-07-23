#!/bin/sh
# Usage: . testlib.sh
# Simple shell command language test library.
#
# Tests must follow the basic form:
#
#   begin_test "the thing"
#   (
#        set -e
#        echo "hello"
#        false
#   )
#   end_test
#
# When a test fails its stdout and stderr are shown.
#
# Note that tests must `set -e' within the subshell block or failed assertions
# will not cause the test to fail and the result may be misreported.
#
# Copyright (c) 2011-13 by Ryan Tomayko <http://tomayko.com>
# License: MIT
set -e

export BT_TEST_PREFIX="$(basename $0) $@"

# Put bin path on PATH
ROOTDIR="$(cd $(dirname "$0")/.. && pwd)"
PATH="$ROOTDIR/bin:$PATH"

if [ -e /data/glb/tests/bt.sh ]; then
  . /data/glb/tests/bt.sh
else
  bt_start () {
    true
  }
  bt_end () {
    true
  }
fi

#bt_init
#trap bt_cleanup EXIT INT

# create a temporary work space
export TRASHDIR=`mktemp -d /tmp/$(basename "$0").XXXXXX`

if tty -s ; then
  RED=$(tput setaf 1)
  GREEN=$(tput setaf 2)
  NORMAL=$(tput sgr0)
fi

# keep track of num tests and failures
tests=0
failures=0

if [[ -z $TEST_SUITE ]]
then
  TEST_SUITE=""
else
  TEST_SUITE="[$TEST_SUITE]"
fi

# this runs at process exit
atexit () {
    res=$?
    [ -z "$KEEPTRASH" ] && rm -rf "$TRASHDIR"
    if [ $failures -gt 0 ]
    then exit 1
    elif [ $res -ne 0 ]
    then exit $res
    else exit 0
    fi
}

# create the trash dir
trap "atexit" EXIT
mkdir -p "$TRASHDIR"
cd "$TRASHDIR"

# Mark the beginning of a test. A subshell should immediately follow this
# statement.
begin_test () {
    test_status=$?
    [ -n "$test_description" ] && end_test $test_status
    unset test_status

    tests=$(( tests + 1 ))
    test_description="$1"

    bt_start "$BT_TEST_PREFIX: $test_description"

    exec 3>&1 4>&2
    out="$TRASHDIR/out"
    err="$TRASHDIR/err"
    rm -f "$TRASHDIR/headers" "$TRASHDIR/response"
    exec 1>"$out" 2>"$err"

    # allow the subshell to exit non-zero without exiting this process
    set -x +e
    before_time=$(date '+%s')
}

report_failure () {
  msg=$1
  desc=$2
  failures=$(( failures + 1 ))
  printf "test: %-100s ${RED}$msg\n" "$desc"
  (
      echo "-- stdout --"
      sed 's/^/    /' <"$TRASHDIR/out"
      echo "-- stderr --"
      grep -a -v -e '^\+ end_test' -e '^+ set +x' <"$TRASHDIR/err" |
          sed 's/^/    /'
      [ -f "$TRASHDIR/headers" ] && {
          echo "-- headers --"
          sed 's/^/    /' <"$TRASHDIR/headers"
          echo "-- response size --"
          wc -c "$TRASHDIR/response"
          echo "-- response --"
          cat "$TRASHDIR/response" | sed 's/^/    /'
          echo "-- response end --"
          echo -n "${NORMAL}"
      }
      [ -f "/tmp/syslogmon-logs" ] && {
          echo "-- syslog(mon) capture --"
          sed 's/^/    /' <"/tmp/syslogmon-logs"
          echo "-- end syslog(mon) capture --"
          echo -n "${NORMAL}"
      }
  ) 1>&2
}

# Mark the end of a test.
end_test () {
    test_status="${1:-$?}"
    ex_fail="${2:-0}"
    after_time=$(date '+%s')
    bt_end "$BT_TEST_PREFIX: $test_description"
    set +x -e
    exec 1>&3 2>&4
    elapsed_time=$((after_time - before_time))

    if [ "$test_status" -eq 0 ]; then
      if [ "$ex_fail" -eq 0 ]; then
        printf "test: %-100s ${GREEN}OK (${elapsed_time}s)\n" "  $TEST_SUITE $test_description ..."
        echo -n "${NORMAL}"
      else
        report_failure "OK (unexpected)" "  $TEST_SUITE $test_description ..."
      fi
    else
      if [ "$ex_fail" -eq 0 ]; then
        report_failure "FAILED (${elapsed_time}s)" "  $TEST_SUITE $test_description ..."
      else
        printf "test: %-100s ${GREEN}FAILED (expected)\n" "  $TEST_SUITE $test_description ..."
        echo -n "${NORMAL}"
      fi
    fi
    unset test_description
}

# Mark the end of a test that is expected to fail.
end_test_exfail () {
  end_test $? 1
}
