from "golang"

packages = %w[
  build-essential
  g++
  git
  wget
  curl
  ruby
  bison
  flex
  iptables
  psmisc
]

run "apt-get update"
run "apt-get install -y #{packages.join(" ")}"
env "GOPATH" => "/go"

if getenv("RELEASE") == ""
  run "wget https://get.docker.com/builds/Linux/x86_64/docker-1.12.1.tgz"
  run "tar -xpf docker-1.12.1.tgz --strip-components=1 -C /usr/bin/"
  run "rm docker-1.12.1.tgz"
  copy "dind", "/dind"
end

copy ".", "/go/src/github.com/erikh/box"

if getenv("IGNORE_LIBMRUBY") == ""
  run "cd /go/src/github.com/erikh/box && make"
end

if getenv("RELEASE") != ""
  run "mv /go/bin/box /box"
  entrypoint "/box"
  run "apt-get purge -y #{packages.join(" ")}"
  run "apt-get autoclean"
  run "rm -rf /usr/local /go /var/cache/apt /var/lib/apt"
  flatten
  tag "erikh/box:latest"
else
  workdir "/go/src/github.com/erikh/box"
  entrypoint "/dind"
  tag "box-test"
end
