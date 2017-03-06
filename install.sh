#!/bin/sh

# linux installer for box

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
curl -sSL "https://github.com/erikh/box/releases/download/v${version}/box-${version}.linux.gz" | gunzip -c > /tmp/box 
chmod ugo+x /tmp/box 
sudo mv /tmp/box /usr/bin/box

echo "box v${version} is now installed to /usr/bin/box" 
