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

inside "/go/src" do
  copy ".", "github.com/erikh/box"
  # FIXME: target path should not be required
  copy "dockerfile-example.rb", "/dockerfile-example.rb"
end

inside "/root" do
  run "go install -v github.com/erikh/box"
end

entrypoint "/box"

run "mv /go/bin/box /box"
run "rm -r /usr/local /usr/lib /usr/share"

flatten
tag "erikh/box:latest"
