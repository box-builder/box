Box is a utility for the building of docker images. Through use of mruby,
we provide additional flexibility over the vanilla `docker build` command by
adding control structures and basic predicates. We also add new verbs that
allow new actions, such as flattening and tagging images.

Some features that differentiate it from `docker build`:

* Unique general features:
    * mruby syntax
    * filtering of keywords to secure builds
    * Simultaneous build of multiple projects
    * Read-Eval-Print-Loop (Shell) mode
* In the build plan itself:
    * Tagging
    * Flattening
    * Debug mode (drop to a shell in the middle of a plan run and inspect your container)
    * Ruby block methods for `user` ([with\_user](verbs/#with95user)) and `workdir` ([inside](verbs/#inside)) allow
      you to scope `copy` and `run` operations for a more obvious build plan.

## Getting Box

**[Download a Release](https://github.com/box-builder/box/releases/)**

### Installation

Quick Install: `curl -sSL box-builder.sh | sudo bash`

Just `gunzip` the downloaded file and put it in your path:

```bash
$ gunzip box.$(uname -s).gz
$ chmod 755 box.$(uname -s)
$ sudo mv box.$(uname -s) /usr/local/bin/box
```

Alternatively, we have a [homebrew tap](https://github.com/box-builder/homebrew-box)
and debian and redhat packages on the [releases page](https://github.com/box-builder/box/releases).

## Invocation

### Default plan

If you have `box.rb` in the current directory, you can just run `box`.

### Use the shell

If you want to try out box quickly, you can use the shell interface, AKA
repl (read-eval-print loop):


```bash
$ box repl
# or
$ box shell
```

This video gives a quick demo of the shell:

<script type="text/javascript" src="https://asciinema.org/a/c1n0h0g73f10x4cuzjf1i51vg.js" id="asciicast-c1n0h0g73f10x4cuzjf1i51vg" async></script>

### With a Plan 

The commandline tool `box` accepts a file (your "build plan") as a commandline
argument:

```bash
$ box myplan.rb
```

The current working directory that Box runs in is very important, it is the
jumping-off point for most copy operations. If you run the `box-builder/box`
container, you may wish to run it in this way:

```bash
$ docker run -i \
  -v $PWD:$PWD \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -w $PWD \
  box-builder/box:latest myplan.rb
```

For additional flags and functionality, see the help:

```bash
$ box --help
```

### With Multiple Plans

To initiate multi-plan builds (where all builds are done at the same time) just
invoke `box multi` and specify all the plans you want, e.g.:

```
$ box multi *.rb
```

Which will build all the `.rb` files in the current dir.

**Note**: it is important to use the [tag](/user-guide/verbs/#tag) verb to
avoid losing track of your images!

## Making Box Plans

Box plans are written in mruby, an embedded, smaller variant of ruby. If you
are new to ruby, here is a tutorial that only [covers the basics](https://github.com/jhotta/chef-fundamentals-ja/blob/master/slides/just-enough-ruby-for-chef/01_slide.md#variables)
You will not need to be an advanced ruby user to leverage Box.

Box plan terms are either functions or verbs.

Verbs typically create a layer and are meant to run at the top level of the
plan; they are not intended to return a sane value other than success/fail.
Operations like `run` and `copy` fit into the "verb" category. These are very
similar to the verbs you'd find in `docker build`.

Functions are unique to Box and allow you to pass data both from the image into
the build system and pass it to other calls, or just print it out for later
use. Functions like `getuid` exist to retrieve the UID of a user as the
container sees it, for the purposes of using it for future operations.

Please take a look at our [verbs reference](/user-guide/verbs) and [functions
reference](/user-guide/functions) for more information.

## Example Box Plan

Here's a basic example that downloads the newest (1.7.3) version of golang with
curl and unpacks it. If you set an environment variable called
`GO_VERSION`, it will use that version instead.

```ruby
from "debian"

run "apt-get update"
run "apt-get install curl -y"

go_version = getenv("GO_VERSION")

if go_version.empty?
  go_version = "1.7.3"
end

url = "https://storage.googleapis.com/golang/go#{go_version}.linux-amd64.tar.gz"

run "curl -sSL '#{url}' | tar -xvz -C /usr/local"
```

### Ignoring files

Just like Docker, if a `.dockerignore` file exists, the patterns, filenames,
and directories specified in this file will be ignored from all copy operations.

The [copy](/user-guide/verbs/#copy) verb also has additional functionality to
scope ignore rules down to specific copy statements.

### The Build Cache

The build cache is enabled by default. It is not an exact cache but constructs
the layer graph in a non-standard way using docker's image Comment field,
populating it with sums and command instructions in a very similar way that
`docker build` does.

If you find the behavior surprising, you can turn it off:

```
$ box --no-cache myplan.rb
```


## Example Box Plan (advanced version)

This is the Box plan we use to build Box itself. It uses many of its
features. Be sure to check the [verbs](https://box-builder.github.io/box/verbs/) to
refer to different constructs used in the file.

You can find the latest version of it
[here](https://github.com/box-builder/box/blob/master/build.rb) too.

```ruby
from "golang"

skip do
  DOCKER_VERSION = "1.12.4"

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

  copy ".", "/go/src/github.com/box-builder/box"
  run "cd /go/src/github.com/box-builder/box && make clean install"

  workdir "/go/src/github.com/box-builder/box"
  set_exec entrypoint: ["/dind"], cmd: ["make", "docker-test"]
  tag "box-test"
end

run "mv /go/bin/box /box"
set_exec entrypoint: ["/box"], cmd: []
```

## Caveats

You can see [all of our issues](https://github.com/box-builder/box/issues) here.
