from "golang"

cmd "/go/bin/box"
env "GOPATH" => "/go"
run "apt-get update"
run "apt-get install -y build-essential g++ git wget curl ruby bison flex"

run "mkdir -p /go/src/github.com/mitchellh"

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
  cd /go/src/github.com/mitchellh && \
  git clone https://github.com/erikh/go-mruby && \
  cd go-mruby && \
  git fetch && \
  git checkout -b class origin/class
  cd /go/src/github.com/mitchellh/go-mruby && \
  make && \
  cp libmruby.a /root
]

workdir "/root" do
  run "go get -v github.com/erikh/box"
end
