#!/bin/sh
# linux/mac installer for box
set -e
version=0.5.6
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
  curl -sSL "https://github.com/box-builder/box/releases/download/v${version}/box-${version}.${arch}.gz" | gunzip -c > /tmp/box 

  sudo="sudo"
  target="/usr/local/bin"

  if [ -z "$(which sudo)" -a `id -u` -ne 0 ]
  then
    echo "Cannot find sudo and not UID 0; installing to home directory..."
    sudo=""
    target=${HOME}/bin
  elif [ `id -u` -eq 0 ]
  then
    sudo=""
  fi

  mkdir -p $target
  ${sudo} /usr/bin/install -m 0755 /tmp/box $target

  echo "box v${version} is now installed to ${target}/box"
}

do_install
