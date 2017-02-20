#!/bin/bash	

term() {
  killall dockerd
  wait
}

set -eu

mkdocs build

dockerd -s vfs &>/tmp/docker.log &
sleep 5

set +e

for i in $*
do
  DIND=1 go test -cover -timeout 60m -v "$i" -check.v -check.f "${TESTRUN}"
  if [ $? != 0 ]
  then
    status=$?
    tail -n 100 /tmp/docker.log
    term
    exit $status
  fi
done

set -e

term
exit 0
