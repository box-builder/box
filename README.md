box: a new type of builder for docker

[![Build Status](https://travis-ci.org/erikh/box.svg?branch=master)](https://travis-ci.org/erikh/box)

build instructions:

* git clone https://github.com/erikh/box
* To build on the host:
  * make
* To build a docker image (needed for test and release builds):
  * make bootstrap
* To run the tests:
  * make test
* To do a release build:
  * make release

Note that if you do not pass a filename or content over stdin, you will be
prompted for input where you can type a script in.

There's no documentation right now, check out the comments in
`builder/verbs.go` and `builder/funcs.go` for more information about the
language.

Here's an example of a dockerfile you can make with it:

```ruby
from "golang"

packages = "build-essential g++ git wget curl ruby bison flex"

run "apt-get update"
run "apt-get install -y #{packages}"

tag "erikh/box:packages"

env "GOPATH" => "/go"

gopaths = [
  "github.com/docker/engine-api",
  "github.com/docker/distribution/reference",
  "github.com/docker/go-connections/nat",
  "github.com/docker/go-units",
  "golang.org/x/net/context",
  "github.com/Sirupsen/logrus",
  "github.com/opencontainers/runc/libcontainer/user",
]

gopaths.each { |gopath| run "go get -v -d #{gopath}" }

run %q[
  mkdir /go/src/github.com/mitchellh && \
  cd /go/src/github.com/mitchellh && \
  git clone https://github.com/mitchellh/go-mruby && \
  cd go-mruby && \
  cd /go/src/github.com/mitchellh/go-mruby && \
  make &&
  cp libmruby.a /root
]

tag "erikh/box:prereqs"

entrypoint "/go/bin/box"

inside "/go/src" do
  copy ".", "github.com/erikh/box"
  # FIXME: target path should not be required
  copy "dockerfile-example.rb", "/dockerfile-example.rb"
end

inside "/root" do
  run "go install -v github.com/erikh/box"
end

run "rm -rf /go/src /go/pkg"
run "apt-get purge -y #{packages}"
run "apt-get autoremove -y"
run "apt-get clean -y"
run "rm -rf /var/lib/apt"

flatten
tag "erikh/box:latest"
```
