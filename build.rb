from "golang"

skip do
  DOCKER_VERSION = "1.12.6"

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
    python-pip
  ]

  workdir "/"
  
  qq = getenv("CI_BUILD") != "" ? "-qq" : ""

  run "apt-get update #{qq}"
  run "apt-get install -y #{qq} #{PACKAGES.join(" ")}"
  env "GOPATH" => "/go"

  docker_path = "docker-#{DOCKER_VERSION}.tgz"
  run "wget -q https://get.docker.com/builds/Linux/x86_64/#{docker_path}"
  run "tar -xpf #{docker_path} --strip-components=1 -C /usr/bin/"
  run "rm #{docker_path}"
  copy "dind", "/dind"

  run "pip -q install mkdocs mkdocs-bootswatch"

  copy ".", "/go/src/github.com/erikh/box"
  run "cd /go/src/github.com/erikh/box && make clean install"

  workdir "/go/src/github.com/erikh/box"
  set_exec entrypoint: ["/dind"], cmd: ["make", "docker-test"]
  tag "box-test"
end

run "mv /go/bin/box /box"
set_exec entrypoint: ["/box"], cmd: []
