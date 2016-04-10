#!/bin/bash

set -eu

if [ $# -ne 1 ]; then
  echo >&2 "Usage: $0 <cassandra_version>"
  exit 1
fi
VERSION="$1"

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
go test -v --tags='redis cassandra'

