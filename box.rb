from "debian"

after do
  tag "boxbuilder/box:master"
  save file: "box-builder.oci.tar", kind: :oci
end

DOCKER_VERSION = "1.13.1"
GOLANG_VERSION = "1.8.5"

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
  python2.7
]

qq = getenv("CI_BUILD") != "" ? "-qq" : ""

skip do
  workdir "/"

  run "apt-get update #{qq}"
  run "apt-get install -y #{qq} #{PACKAGES.join(" ")}"

  docker_path = "docker-#{DOCKER_VERSION}.tgz"
  run "wget -q https://get.docker.com/builds/Linux/x86_64/#{docker_path}"
  run "tar -xpf #{docker_path} --strip-components=1 -C /usr/bin/"
  run "rm #{docker_path}"

  run "curl -sSL https://storage.googleapis.com/golang/go#{GOLANG_VERSION}.linux-amd64.tar.gz | tar -xz -C /usr/local"

  copy "dind", "/dind"

  run "curl -sSL https://bootstrap.pypa.io/get-pip.py | python2.7"

  run "pip -q install mkdocs mkdocs-bootswatch"

  env "PATH" => "/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin:/usr/local/sbin:/usr/local/go/bin:/go/bin", "GOPATH" => "/go"
  copy ".", "/go/src/github.com/box-builder/box", ignore_file: ".boxignore"
  run "cd /go/src/github.com/box-builder/box && VERSION=#{getenv("VERSION")} make clean install"

  workdir "/go/src/github.com/box-builder/box"
  set_exec entrypoint: ["/dind"], cmd: %w[make docker-test]
  sum = getenv("SUM")
  if sum != ""
    sum = "-#{sum}"
  end
  tag "box-test#{sum}"
end

run "mv /go/bin/box /box"
workdir "/"
set_exec entrypoint: ["/box"], cmd: []
