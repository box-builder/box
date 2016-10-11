from "golang"

packages = "build-essential g++ git wget curl ruby bison flex"

run "apt-get update"
run "apt-get install -y #{packages}"

env "GOPATH" => "/go"

tag "erikh/box:prereqs"

inside "/go/src" do
  copy ".", "github.com/erikh/box"
  # FIXME: target path should not be required
  copy "dockerfile-example.rb", "/dockerfile-example.rb"
end

inside "/go/src/github.com/erikh/box" do
  run "make"
end

entrypoint "/box"

run "mv /go/bin/box /box"
run "rm -r /usr/local /usr/lib /usr/share"

flatten
tag "erikh/box:latest"
