#!/bin/bash	

dockerd -s vfs &
sleep 5

go test -v ./... -check.v
status=$?

killall dockerd
wait
exit $?
