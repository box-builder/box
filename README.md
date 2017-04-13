# Box: A Next-Generation Builder for Docker Images 

[![Build Status](http://jenkins.hollensbe.org:8080/job/box-master/badge/icon)](http://jenkins.hollensbe.org:8080/job/box-master/)
[![Join the chat at https://gitter.im/box-builder/Lobby](https://badges.gitter.im/box-builder/Lobby.svg)](https://gitter.im/box-builder/Lobby?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)
[![Go Report Card](https://goreportcard.com/badge/github.com/box-builder/box)](https://goreportcard.com/report/github.com/box-builder/box)

Box is a builder for docker that gives you the power of mruby, a limited,
embeddable ruby. It allows for notions of conditionals, loops, and data
structures for use within your builder plan. If you've written a Dockerfile
before, writing a box build plan is easy.

<img style="align: center" src="https://raw.githubusercontent.com/box-builder/box/master/docs-theme/img/box-logo.png"></img>

## Box Build Plans are Programs

Exploit this! Use functions! Set variables and constants!

run this plan with:

```shell
GOLANG_VERSION=1.7.5 box <plan file>
```

```ruby
from "ubuntu"

# this function will create a new layer running the command inside the
# function, installing the required package.
def install_package(pkg)
  run "apt-get install '#{pkg}' -y"
end

run "apt-get update"
install_package "curl" # `run "apt-get install curl -y"`

# get the local environment's setting for GOLANG_VERSION, and set it here:
go_version = getenv("GOLANG_VERSION")
run %Q[curl -sSL \
    https://storage.googleapis.com/golang/go#{go_version}.linux-amd64.tar.gz \
    | tar -xvz -C /usr/local]
```

### Powered by mruby

Box uses the [mruby programming language](https://mruby.org/). It does
this to get a solid language syntax, functions, variables and more.
However, it is not a fully featured Ruby such as MRI and contains almost
zero standard library functionality, allowing for only the basic types,
and no I/O operations outside of the box DSL are permitted.

You can however:

* Define classes, functions, variables and constants
* Access the environment through the
  [getenv](https://box-builder.github.io/box/user-guide/functions/#getenv) box function (which is also
  [omittable](https://box-builder.github.io/box/user-guide/cli/#-omit-o) if you don't want people to use
  it)
* Retrieve the contents of container files with [read](https://box-builder.github.io/box/user-guide/functions/#read)
* [import](https://box-builder.github.io/box/user-guide/functions/#import) libraries (also written in
  mruby) to re-use common build plan components.

### Tagging and Image Editing

You can tag images mid-plan to create multiple images, each subsets (or
supersets, depending on how you look at it) of each other.

Additionally, you can use functions like
[after](https://box-builder.github.io/box/user-guide/verbs/#after), [skip](https://box-builder.github.io/box/user-guide/functions/#skip),
and [flatten](https://box-builder.github.io/box/user-guide/verbs/#flatten) to manipulate images in ways
you may not have considered:

```ruby
from :ubuntu
skip do
  run "apt-get update"
  run "apt-get install curl -y"
  run "curl -sSL -O https://github.com/box-builder/box/releases/download/v0.4.2/box_0.4.2_amd64.deb"
  tag :downloaded
end

run "dpkg -i box*.deb"
after do
  flatten
  tag :installed
end
```

### And more!

All the standard docker build commands such as user, env, and a few new ones:

* `with_user` and `inside` temporarily scope commands to a specific user or
  working directory respectively, allowing you to avoid nasty patterns like
  `cd foo && thing`.
* `debug` drop-in statement: drops you to a container in the middle of a build
  where you place the call.

### REPL (Shell)

REPL is short for "read eval print loop" and is just a fancy way of saying this
thing has readline support and a shell history. Check the thing out by invoking
`box repl` or `box shell`.

Here's a video of the shell in action (click for more):

[![Box REPL](https://asciinema.org/a/c1n0h0g73f10x4cuzjf1i51vg.png)](https://asciinema.org/a/c1n0h0g73f10x4cuzjf1i51vg)

## Install

* Easy install: `curl -sSL box-builder.sh | sudo bash`
* **[Download Releases](https://github.com/box-builder/box/releases/)**
* **[Homebrew Tap](https://github.com/box-builder/homebrew-box)**

### Using the Homebrew Tap

```bash
brew tap box-builder/box && brew install box-builder/box/box
```
## Advanced Use

The [documentation](https://box-builder.github.io/box/) is the best resource for
learning the different verbs and functions. However, check out
[our own build plan for box](https://github.com/box-builder/box/blob/master/build.rb)
for an example of how to use different predicates, functions, and verbs to
get everything you need out of it.

## Development Instructions

* **Requires**: compiler, bison, flex, and libgpgme, libdevmapper, btrfs headers.
* `go get -d github.com/box-builder/box && cd $GOPATH/src/github.com/box-builder/box`
* To build on the host (create a dev environment):
  * `make`
* To build a docker image for your dev environment (needed for test and release builds):
  * `make build`
* If you have a dev environment:
  * `make test`
* To do a release build:
  * `VERSION=<version> make release`
