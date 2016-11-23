#!/bin/bash	

dockerd -s vfs &
sleep 5

for i in $*
do
  go test -timeout 30m -v "$i" -check.v -check.f "${TESTRUN}"
done

status=$?

killall dockerd
wait
exit $status
