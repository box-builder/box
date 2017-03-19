#!/bin/sh
# linux/mac installer for box
set -e
version=0.5.1
if [ "$(uname -s)" = "Linux" ]; then 
  arch="linux"
else
  arch="portable"
  if [ ! -f `which docker` ]; then
    echo "On non-linux platforms, box runs in a docker container. Ensure the docker command is in your PATH and try installing again."   
    exit 1;
  fi
fi

do_install() {
  echo "Installing version v${version}"
  curl -sSL "https://github.com/erikh/box/releases/download/v${version}/box-${version}.${arch}.gz" | gunzip -c > /tmp/box 
  chmod ugo+x /tmp/box 
  sudo="sudo"
  if [ `id -u` -eq 0 ]
  then
    sudo=""
  fi
  $sudo mv /tmp/box /usr/bin/box

  echo "box v${version} is now installed to /usr/bin/box" 
}

do_install
