# Box: A Next-Generation Builder for Docker Images 

[![Build Status](https://travis-ci.org/erikh/box.svg?branch=master)](https://travis-ci.org/erikh/box)
[![Join the chat at https://gitter.im/box-builder/Lobby](https://badges.gitter.im/box-builder/Lobby.svg)](https://gitter.im/box-builder/Lobby?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)
[![Go Report Card](https://goreportcard.com/badge/github.com/erikh/box)](https://goreportcard.com/report/github.com/erikh/box)

Box is a builder for docker that gives you the power of mruby, a limited,
embeddable ruby. It allows for notions of conditionals, loops, and data
structures for use within your builder plan. If you've written a Dockerfile
before, writing a box build plan is easy.

* Unique general features:
  * mruby syntax
  * filtering of keywords to secure builds
  * REPL (shell) for interactive building (see video below)
* In the build plan itself:
  * Tagging
  * Flattening
  * Debug mode (drop to a shell in the middle of a plan run and inspect your container)
  * Ruby block methods for `user` (`with_user`) and `workdir` (`inside`) allow
    you to scope `copy` and `run` operations for a more obvious build plan.

* **[Download Releases](https://github.com/erikh/box/releases/)**
* **[Extended Documentation for Syntax and Usage](https://erikh.github.io/box/)**

## Example

This will fetch the golang image, update APT, and then install the packages set
in the `packages` variable. It then creates a user and copies the dir to its
homedir. If an environment value is provided, it will be used. Then it will tag
the whole image as `mypackages`.

Save it to plan.rb and run it with `box plan.rb`. **Box only copies what it
needs to; your whole directory won't be uploaded to docker.**

```ruby
from "golang"

packages = "build-essential g++ git wget curl ruby bison flex"

run "apt-get update"
run "apt-get install -y #{packages}"

run %q[useradd -m -d /home/erikh -s /bin/bash erikh]

inside "/home/erikh" do
  copy((getenv("MYDIR") || "."), ".")
end

tag "mypackages"
```

## Video

Here's a video of the shell in action (click for more):

*Available in v0.3 and up*

[![Box REPL](http://i.makeagif.com/media/11-14-2016/HH-2BW.gif)](https://youtu.be/znCfrabMUs0)

## Advanced Use

The [documentation](https://erikh.github.io/box/) is the best resource for
learning the different verbs and functions. However, check out
[our own build plan for box](https://github.com/erikh/box/blob/master/build.rb)
for an example of how to use different predicates, functions, and verbs to
get everything you need out of it.

## Development Instructions

* **Requires**: compiler, bison, flex
* `go get -d https://github.com/erikh/box && cd $GOPATH/src/github.com/erikh/box`
* To build on the host (create a dev environment):
  * `make`
* To build a docker image for your dev environment (needed for test and release builds):
  * `make bootstrap`
* To run the tests without a dev environment configured:
  * `make bootstrap-test`
* If you have a dev environment:
  * `make test`
    * note that if you are building both on the hosts and in the test
      containers, `IGNORE_LIBMRUBY=1` may be of interest to you if you get
      linking errors or GC panics.
* To do a release build:
  * `make release`
