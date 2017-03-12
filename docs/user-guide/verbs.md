Verbs take action on a container and usually create a layer. Some commands can
be used to move data into and out of containers, or set properties and run
commands.

## label

`label` creates a label inside the image. It will append any labels that are
specified in each label command, for example, if you were to:

```ruby
label foo: "bar"
label quux: "baz"
```

If you were to `docker inspect` this, your ending labels would be:

```json
{
  "foo": "bar",
  "quux": "baz"
}
```

You can also do this all as one label:

```ruby
label foo: "bar", quux: "baz"
```

Or if you'd like, store them in a variable for a final commit at the end:

```ruby
mylabels = { }
mylabels["foo"] = "baz"
# ... some stuff that generates mydata
mylabels["quux"] = mydata

label mylabels # voila!
```

## debug

`debug` drops to a container's shell (bash by default, but an argument can be
provided to change this) at the current place in the build cycle when invoked.
Changes to the container will be persisted through the rest of the run, and a
layer will be created.

If the shell exits non-zero, like `run` it will abort the build.

There is currently no way to detach from a debug session. Close the shell
and/or programs.

Example:

```ruby
from "debian"
copy ".", "/test"
debug # verify all files landed in test like you expected them to
run "chown -R erikh:erikh /test" # this will run after you close the shell
```

## after

`after` coordinates with `skip` to provide layer editing facilities. After the
image is recomposed, this hook will run. Great for tagging and flattening.

Example:

```ruby
from "debian"

# after the skip layers are removed, run this hook.
after do
  # tag the edited image as `dev`
  tag "dev"
end

skip do
  run "apt-get update -qq"
end

run "apt-get install tmux -y"
```

## set\_exec
`set_exec` sets both the entrypoint and cmd at the same time.

`set_exec` takes a dictionary consisting of two known elements as symbols:
entrypoint and cmd. They each take a string array which is then propagated
to the respective properties in the container's configuration.

This command does not modify the entrypoint or cmd for `run` operations.

Example:

```ruby
from "debian"
# this sets the what will be run with `/bin/echo foo`
set_exec entrypoint: ["/bin/echo"], cmd: ["foo"]
```

## workdir

workdir sets the working directory in the docker environment. It sets this
throughout the image creation; all run/copy statements will respect this
value. If you wish to break out or work within it further, look at the
`inside` call.

The workdir, if left empty by either the parent in `from` or from no
interaction from the plan, is set to `/` to avoid inheriting accidentally from
the parent image.

Example:

```ruby
from "debian"
# each container that runs this image without the `-w` flag will start as
# `/test` for the current working directory.
workdir '/test'
```

## user

user sets the username this container will use by default. It also affects
following run statements (but not copy, which always copies as root
currently). If you wish to switch to a user temporarily, consider using
`with_user`.

An empty user is always set to `root` in the final image.

Example:

```ruby
from "debian"
# all containers started with this image will use user `foo`
# %q[] just means, "quote this as a string without interpolation"
user %q[foo]
```

## flatten

flatten requires no argumemnts and flattens all layers and commits a new
layer. This is useful for reducing the size of images or making them easier
to distribute.

NOTE: flattening will always bust the build cache.

NOTE: flattening requires downloading the image and re-uploading it. This
can take a lot of time over remote connections and is not advised.

Example:

```ruby
from "debian"
# create some layers
run "true"
copy ".", "/test"
flatten # image is shrunk to one layer here
tag "erikh/test"
```

## tag

tag tags an image within the docker daemon, named after the string provided.
It must be a valid tag name.

Example:

```ruby
from "debian"
run "true" # create a layer
tag "erikh/true" # tag the latest image as "erikh/true"
```

## entrypoint

entrypoint sets the entrypoint for the image at runtime. It will not be
used for run invocations.

Example:

```ruby
from "debian"
# if you pass nil or an empty array, it will clear any inherited cmd from the debian image.
entrypoint []
entrypoint %w[/bin/echo -e] # arrays also work
entrypoint "/bin/echo"      # all `docker run` commands will be preceded by this
cmd "foo"                   # this will equate to `/bin/echo foo`
```

## from

from sets the initial image and if necessary, pulls it from the registry. It
also sets the initial layer and must be called before several operations.

Using `from` overwrites all container configuration, including `workdir`,
`user`, `env`, `cmd`, and `entrypoint`.

It is expected that `from` is called first in a build plan.

If `from :scratch` is provided, the build plan will start out with no files and
no configuration. You will want to use `copy`, `set_exec`, etc to configure
your container image.

Example:

```ruby
from "debian"
```

or other images with full tags:

```ruby
from "ceph/rbd:latest"
```

or fully qualified image IDs.

```ruby
# sha256s are longer than this normally.
from "sha256:deadbeefcafebabeaddedbeef"
```

`from :scratch`:

```ruby
from :scratch
copy "box", "/"
entrypoint "/box"
```

## run

run runs a command provided as a string, and saves the layer.

It respects user and workdir, but not entrypoint and command. It does this
so it can respect the values provided in the plan instead of what was
intended for the final image.

Options:

* `output`: supply `false` to omit output from the plan run.

Cache keys are generated based on the command name, so to be certain your
command is run in the event of it hitting cache, run box with NO_CACHE=1.

Examples:

Create a file called `/bar` inside the container, then chown it to nobody. Run
commands don't need a lot of `&&` because you can trivially flatten the layers.

Run does not accept the exec-form from docker's RUN equivalent. Everything RUN
processes goes through `/bin/sh -c`.

```ruby
from "debian"
run "echo foo >/bar"
run "chown nobody:nogroup /bar"
```

Run in the context of a specific user or workdir. This allows us to finely
control our run invocations and further processing after the container image
has been run.

```ruby
from "debian"

with_user "nobody" do # just the commands inside this block will run as `nobody`
  run "echo foo >/tmp/bar"
end

# notice how we are still root
run "useradd -s /bin/sh -m -d /home/erikh erikh"

# all commands from here on will run as `erikh`, overridden only by `with_user`
# and other `user` calls.
user "erikh"
run "echo foo >/tmp/erikh-file"

# set the workdir temporarily for the commands within the block.
# this will create /tmp/another-file-in-tmp.
inside "/tmp" do
  run "echo foo >another-file-in-tmp"
end

# this behaves exactly like user, just setting the default cwd instead:
# creates /tmp/yet-another-file
workdir "/tmp"
run "echo foo >yet-another-file"

# will not display anything
run "ls -l /", output: false
```

## with\_user

`with_user`, when provided with a string username and block invokes commands
within the user's login context. Unfortunately, copy does not respect this
yet. It does not affect the final image.

Example:

```ruby
from "debian"

with_user "nobody" do
  run "whoami" # i am nobody!
end
```

## inside

inside, when provided with a directory name string and block, invokes
commands within the context of the working directory being set to the
string.

It will affect the final image if a file-modification event occurs inside a
directory which has been specified but not created manually yet. This is a side
effect of the docker engine's relationship to how we use `workdir` directives
within docker itself. **Docker will create any workdir that does not exist when
a build container is started.**

Example:

```ruby
from "debian"

inside "/dev" do
  run "mknod webscale c 1 3"
end
```

When given a relative path, it assumes the workdir as well as any other
additional inside statements. For example:

```ruby
workdir "/etc"
inside "apt" do # will travel into /etc/apt/
  run "rm sources.list"
end
```

## env

env, when provided with a hash of string => string key/value combinations,
will set the environment in the image and future run invocations.

Example:

```ruby
from "debian"

env "GOPATH" => "/go", "PATH" => "/usr/bin:/bin"
env GOPATH: "/go", PATH: "/usr/bin:/bin" # equivalent if you prefer this syntax
```

## cmd

cmd, when provided with a string will set the docker image's Cmd property,
which are the arguments that follow the entrypoint (and are overridden when
you provide a command to `docker run`). It does not affect run invocations.

Example:

```ruby
from "debian"
# if you pass nil or an empty array, it will clear any inherited cmd from the debian image.
cmd nil
# You can also use arrays.
cmd %w[ls -la]
# This image will run `ls` in the workdir by default.
cmd "ls"
```

## copy

copy copies files from the host to the container. It only works relative to
the current directory. The build cache is calculated by summing the tar
result of edited files. Since mtime is also considered, changes to that will
also bust the cache.

copy accepts globbing on the local side (LHS of arguments) according to
[these rules](https://golang.org/pkg/path/filepath/#Match). For example, it
supports `*` but not the zsh extended `**` syntax.

Parameters may be specified after the target directory; the following options
are supported:

* `ignore_list`: the provided array of file patterns will be ignored from the
  copied product.
* `ignore_file`: similar to `ignore_list`, it will reap the values from the
  filename specified.

NOTE: copy will not overwrite directories with files, this will abort the run.
If you are trying to copy a file into a named directory, suffix it with `/`
which will instruct it to put it into that directory instead of trying to
replace it with the file you're copying.

NOTE: copy does not respect user permissions when the `user` or `with_user`
modifiers are applied. This will be fixed eventually.

Example:

```ruby
from "debian"

# recursively copies everything the cwd to test, which is relative to the
# workdir inside the container (`/` by default).
workdir "/tmp", do
  copy ".", "/test"
end

copy "a_file", "/tmp/" # example of not overwriting directories with files
copy "files*", "/var/lib" # example of globbing

# copy all files named `files*`, but ignore the ones that start with `files1*`.
copy "files*", "/var/lib", ignore_list: ["files1*"] 
```
