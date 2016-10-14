Box is a small utility for the building of docker images. Through use of mruby,
we provide additional flexibility over the vanilla `docker build` command by
adding control structures and basic predicates. We also add new verbs that
allow new actions, such as flattening and tagging images.

## Getting Box

That's complicated. You can either follow the [Development Instructions](https://github.com/erikh/box/blob/master/README.md)
or you can pull `erikh/box:latest` for now. See `Invocation` for how to run it.

## Invocation

The commandline tool `box` will either accept a file as a commandline argument:

```shell
$ box myplan.rb
```

Or from stdin:

```shell
$ box < myplan.rb
```

The current working directory that Box runs in is very important, it is the
jumping-off point for most copy operations. If you run the `erikh/box`
container, you may wish to run it in this way:

```shell
$ docker run -i -v $PWD:$PWD -v /var/run/docker.sock:/var/run/docker.sock -w $PWD erikh/box:latest < myplan.rb
```

### Running without cache

For now, set `NO_CACHE=1` in your shell's environment (or docker's environment
with the `-e` switch to `docker run`) which will trump the entire cache. Note
that parameterized statements and copy statements where the files have changed
on disk will also bust the cache.

## Making Box scripts

Box scripts are written in mruby, an embedded, smaller variant of ruby. If you
are new to ruby, here is a tutorial that only [covers the basics](https://github.com/jhotta/chef-fundamentals-ja/blob/master/slides/just-enough-ruby-for-chef/01_slide.md).
You will not need to be an advanced ruby user to leverage Box.

Box script terms are either functions or verbs.

Verbs typically create a layer and are meant to run at the top level of the
script; they are not intended to return a sane value other than success/fail.
Operations like `run` and `copy` fit into the "verb" category. These are very
similar to the verbs you'd find in `docker build`.

Functions are unique to Box and allow you to pass data both from the image into
the build system and pass it to other calls, or just print it out for later
use. Functions like `getuid` exist to retrieve the UID of a user as the
container sees it, for the purposes of using it for future operations.

Please take a look at our [verbs reference](/verbs) and [functions
reference](/functions) for more information.

## Example Box script

This is the Box script we use to build Box itself. It uses many of its
features.

You can find the latest version of it [here](https://github.com/erikh/box/blob/master/build.rb) too.

```ruby
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
```

## Caveats

Things that Box still needs to fix that are documentation worthy:

* Set `NO_CACHE=1` if you want to bypass the build cache. There is a ticket to
  make this a command-line flag [here](https://github.com/erikh/box/issues/9).
* Currently we require a TTY and access to stdout to function.
  [This issue](https://github.com/erikh/box/issues/4) was made to correct that.

Of course, you can see [all of our issues](https://github.com/erikh/box/issues) too.
