# box: a new type of builder for docker [![Build Status](https://travis-ci.org/erikh/box.svg?branch=master)](https://travis-ci.org/erikh/box) 

Box is a builder for docker that gives you the power of mruby, a limited,
embeddable ruby. It allows for notions of conditionals, loops, and data
structures for use within your builder plan. If you've written a Dockerfile
before, writing a box build plan is easy.

**[Extended Documentation for Syntax and Usage](https://erikh.github.io/box/)**

## Example

This will fetch the golang image, update APT and then install the packages set
in the `packages` variable. Then it will tag the whole image as `mypackages`.

Save it to plan.rb and run it with `box plan.rb` or `box < plan.rb` to read it
from stdin. **Box only copies what it needs to; your whole directory won't be
uploaded to docker.**

```ruby
from "golang"

packages = "build-essential g++ git wget curl ruby bison flex"

run "apt-get update"
run "apt-get install -y #{packages}"

tag "mypackages"
```

## Advanced Use

The [documentation](https://erikh.github.io/box/) is the best resource for
learning the different verbs and functions. However, check out
[our own build plan for box](https://github.com/erikh/box/blob/master/build.rb)
for an example of how to use different predicates, functions, and verbs to
get everything you need out of it.

## Development Instructions

* `go get -d https://github.com/erikh/box && cd $GOPATH/src/github.com/erikh/box`
* To build on the host:
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
