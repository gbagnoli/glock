#!/bin/bash

set -eu

if [ $# -ne 1 ]; then
  echo >&2 "Usage: $0 <memory|redis|cassandra:x.y.z>"
  exit 1
fi

DB="$1"
VERSION=""
CASSANDRA_WAIT_TIME="${CASSANDRA_WAIT_TIME:-10}"

case $DB in
  "memory")
    TAGS='memory'
    ;;

  "redis")
    TAGS='redis'
    ;;

  "cassandra")
    echo >&2 "Missing cassandra version. (cassandra:x.y.z)"
    exit 1
    ;;

  cassandra*)
    TAGS='cassandra'
    IFS=':' read -ra PARTS <<< "$DB"
    if [ "${#PARTS[@]}" -lt 2 ]; then
      echo >&2 "Missing cassandra version. (cassandra:x.y.z)"
      exit 1
    fi
    VERSION="${PARTS[1]}"
    ;;

  *)
    echo >&2 "Invalid DB $DB"
    exit 1
    ;;
esac

if [ ! -z "$VERSION" ]; then
  set +e
  ccm=$(which ccm)
  if [ $? -ne 0 ]; then
    echo >&2 "Cannot found ccm in PATH:$PATH"
    exit 1
  fi
  set -e

  function cleanup {
    $ccm remove test &>/dev/null || true
  }

  cleanup
  echo "Creating cassandra cluster with version $VERSION"
  $ccm create test -v "$VERSION" -n 1 -d --vnodes &>/dev/null
  trap cleanup EXIT

  echo "Starting cluster"
  $ccm start

  echo -n "Waiting for cassandra to settle "
  for (( i=0; i < CASSANDRA_WAIT_TIME; i++ )); do
    set +e
    echo -n "."
    $ccm node1 cqlsh -e 'DESCRIBE KEYSPACES' &>/dev/null
    [ $? -eq 0 ] && break
    sleep 1
  done
  set -e
  if [ "$i" -eq "$CASSANDRA_WAIT_TIME" ]; then
    echo >&2 "Cassandra failed to start in $CASSANDRA_WAIT_TIME seconds"
    exit 1
  fi
  echo " ok"
fi

go test -v --tags="$TAGS"
exit $?
