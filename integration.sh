#!/bin/bash

set -eu

if [ $# -ne 1 ]; then
  echo >&2 "Usage: $0 <memory|redis|cassandra:x.y.z>"
  exit 1
fi

DB="$1"
VERSION=""

case $DB in
  "memory")
    TAGS=''
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
    $ccm remove test || true
  }

  cleanup
  $ccm create test -v "$VERSION" -n 1 -d --vnodes
  trap cleanup EXIT

  sleep 1
  $ccm list
  $ccm start
  $ccm status
  $ccm node1 nodetool status
fi

go test -v --tags="$TAGS"

