from "golang"
workdir "/" # affects copy statements later

DOCKER_VERSION = "1.12.2"

PACKAGES = %w[
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
run "apt-get install -y #{PACKAGES.join(" ")}"
env "GOPATH" => "/go"

if getenv("RELEASE") == ""
  docker_path = "docker-#{DOCKER_VERSION}.tgz"
  run "wget https://get.docker.com/builds/Linux/x86_64/#{docker_path}"
  run "tar -xpf #{docker_path} --strip-components=1 -C /usr/bin/"
  run "rm #{docker_path}"
  copy "dind", "/dind"
end

copy ".", "/go/src/github.com/erikh/box"

if getenv("IGNORE_LIBMRUBY") == ""
  run "cd /go/src/github.com/erikh/box && make clean all"
end

if getenv("RELEASE") != ""
  run "mv /go/bin/box /box"
  set_exec entrypoint: ["/box"], cmd: []
  run "apt-get purge -y #{packages.join(" ")}"
  run "apt-get autoclean"
  run "rm -rf /usr/local /go /var/cache/apt /var/lib/apt"
  flatten
  tag "erikh/box:latest"
else
  workdir "/go/src/github.com/erikh/box"
  set_exec entrypoint: ["/dind"], cmd: ["make", "docker-test"]
  tag "box-test"
end
