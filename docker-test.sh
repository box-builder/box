#!/bin/bash	

set -eu

mkdocs build

dockerd -s vfs &
sleep 5

for i in $*
do
  go test -cover -timeout 30m -v "$i" -check.v -check.f "${TESTRUN}"
done

status=$?

killall dockerd
wait
exit $status
