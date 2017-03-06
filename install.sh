#!/bin/sh

# linux installer for box
# TODO: Check that we have perms. *for now just assume that 
#                                  we are already root or in sudo
# TODO: This has no failure recovery or cleanup

if [ -z "$1" ]; then
	echo "No version specified, attempting to get latest"
	version=`curl -sSL https://raw.githubusercontent.com/cmaujean/box/installer-script/LATEST`
  if [ "$version" = "404: Not Found" ]; then
    echo "latest not found, setting default"
		version=0.4.1
  fi
else
  version=$1
fi

echo "Installing version v${version}"
curl -sSL "https://github.com/erikh/box/releases/download/v${version}/box-${version}.linux.gz" | gunzip -c > /usr/bin/box && chmod ugo+x /usr/bin/box

echo "box v${version} is now installed to /usr/bin/box" 
