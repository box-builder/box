#!/bin/bash

sed -e "s/@@VERSION@@/$1/g" release/RELEASE.md >RELEASE.tmp.md

vim RELEASE.tmp.md

cp $GOPATH/bin/box .

lcuname=$(uname -s | tr LD ld)

gzip -c box > "box-${1}.${lcuname}.gz"
sha256sum "box-${1}.${lcuname}.gz"
