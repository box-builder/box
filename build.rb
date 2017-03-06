from "debian"

after { tag "erikh/box:master" }
DOCKER_VERSION = "1.13.1"
GOLANG_VERSION = "1.7.5"
LVM2_VERSION = "2.02.103"
GPGME_VERSION = "1.8.0"

PACKAGES = %w[
  libgpg-error-dev
  libassuan-dev
  btrfs-tools
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
  btrfs-tools
]

qq = getenv("CI_BUILD") != "" ? "-qq" : ""

skip do
  workdir "/"

  run "apt-get update #{qq}"
  run "apt-get install -y #{qq} #{PACKAGES.join(" ")}"

	run "mkdir -p /usr/local/gpgme && curl -sSL https://www.gnupg.org/ftp/gcrypt/gpgme/gpgme-#{GPGME_VERSION}.tar.bz2 | tar -xjC /usr/local/gpgme --strip-components=1"
	run "cd /usr/local/gpgme && ./configure --enable-static && PREFIX=/usr make install"

	# shamelessly taken from docker
	run %Q[mkdir -p /usr/local/lvm2 \
		&& curl -fsSL "https://mirrors.kernel.org/sourceware/lvm2/LVM2.#{LVM2_VERSION}.tgz" \
			| tar -xzC /usr/local/lvm2 --strip-components=1]
	# See https://git.fedorahosted.org/cgit/lvm2.git/refs/tags for release tags

	# Compile and install lvm2
	run %q[cd /usr/local/lvm2 \
		&& ./configure \
			--build="$(gcc -print-multiarch)" \
			--enable-static_link \
		&& make device-mapper \
		&& make install_device-mapper]

  docker_path = "docker-#{DOCKER_VERSION}.tgz"
  run "wget -q https://get.docker.com/builds/Linux/x86_64/#{docker_path}"
  run "tar -xpf #{docker_path} --strip-components=1 -C /usr/bin/"
  run "rm #{docker_path}"

  run "curl -sSL https://storage.googleapis.com/golang/go#{GOLANG_VERSION}.linux-amd64.tar.gz | tar -xz -C /usr/local"

  copy "dind", "/dind"

  run "curl -sSL https://bootstrap.pypa.io/get-pip.py | python2.7"

  run "pip -q install mkdocs mkdocs-bootswatch"

  env "PATH" => "/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin:/usr/local/sbin:/usr/local/go/bin:/go/bin", "GOPATH" => "/go"
  copy ".", "/go/src/github.com/erikh/box"
  run "cd /go/src/github.com/erikh/box && VERSION=#{getenv("VERSION")} make clean install-static"

  workdir "/go/src/github.com/erikh/box"
  set_exec entrypoint: ["/dind"], cmd: %w[make docker-test]
  tag "box-test"
end

run "mv /go/bin/box /box"
workdir "/"
set_exec entrypoint: ["/box"], cmd: []
