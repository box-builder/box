from "golang"

run "apt-get update"
run "apt-get install -y build-essential g++ git wget curl ruby bison flex"

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
  make && \
  cp libmruby.a /root
]

tag "erikh/box:prereqs"

entrypoint "/go/bin/box"
inside "/root" do
  copy "example.rb", "example.rb"
  run "go get -v github.com/erikh/box"
end

tag "erikh/box:latest"
