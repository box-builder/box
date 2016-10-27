#!/bin/bash	

dockerd -s vfs &
sleep 5

go test -timeout 30m -v $* -check.v -check.f "${TESTRUN}"
status=$?

killall dockerd
wait
exit $status
