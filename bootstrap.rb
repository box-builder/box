from "golang"

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
  cp libmruby.a /root/
]

workdir "/root" do
  run "wget https://gist.githubusercontent.com/erikh/b45e9f45e2cd2f2937dfda0d2bd35cfb/raw/28633f70b0c4152eb6361ae8ae8c3ee0d2dfcc44/main.go"
  run "go build -v main.go"
end
