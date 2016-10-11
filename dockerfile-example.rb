from "golang"

packages = "build-essential g++ git wget curl ruby bison flex"

run "apt-get update"
run "apt-get install -y #{packages}"

env "GOPATH" => "/go"

tag "erikh/box:prereqs"

inside "/go/src" do
  copy ".", "github.com/erikh/box"
end

inside "/go/src/github.com/erikh/box" do
  run "make"
end

cmd "cd /go/src/github.com/erikh/box && box"

flatten
tag "erikh/box:latest"
