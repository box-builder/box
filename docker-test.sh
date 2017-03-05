#!/bin/bash	

term() {
  killall dockerd
  wait
}

set -eu

mkdocs build

dockerd -s vfs &>/tmp/docker.log &
sleep 5

trap term INT TERM

for i in $*
do
  DIND=1 go test -cover -timeout 120m -v "$i" -check.v -check.f "${TESTRUN}"
done
