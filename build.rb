from "golang"

skip do
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
    python-pip
  ]

  workdir "/"

  run "apt-get update"
  run "apt-get install -y #{PACKAGES.join(" ")}"
  env "GOPATH" => "/go"

  docker_path = "docker-#{DOCKER_VERSION}.tgz"
  run "wget https://get.docker.com/builds/Linux/x86_64/#{docker_path}"
  run "tar -xpf #{docker_path} --strip-components=1 -C /usr/bin/"
  run "rm #{docker_path}"
  copy "dind", "/dind"

  copy ".", "/go/src/github.com/erikh/box"

  if getenv("IGNORE_LIBMRUBY") == ""
    run "cd /go/src/github.com/erikh/box && make clean all"
  end

  run "pip install mkdocs mkdocs-bootswatch"

  workdir "/go/src/github.com/erikh/box"
  set_exec entrypoint: ["/dind"], cmd: ["make", "docker-test"]
  tag "box-test"
end

run "mv /go/bin/box /box"
set_exec entrypoint: ["/box"], cmd: []
