Verbs take action on a container and usually create a layer. Some commands can
be used to move data into and out of containers, or set properties and run
commands.


## set\_exec
`set_exec` sets both the entrypoint and cmd at the same time, allowing for no
race between the operations.

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

workdir sets the WorkingDir in the docker environment. It sets this
throughout the image creation; all run/copy statements will respect this
value. If you wish to break out or work within it further, look at the
`inside` call.

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
used for run invocations. Note that setting this clears any previously set cmd.

Example:

```ruby
from "debian"
entrypoint "/bin/echo" # all `docker run` commands will be preceded by this
cmd "foo"              # this will equate to `/bin/echo foo`
```

## from

from sets the initial image and if necessary, pulls it from the registry. It
also sets the initial layer and must be called before several operations.

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

## run

run runs a command provided as a string, and saves the layer.

It respects user and workdir, but not entrypoint and command. It does this
so it can respect the values provided in the script instead of what was
intended for the final image.

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

# all commands from here on will run as `erikh`, overriden only by `with_user`
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
string. It does not affect the final image.

Example:

```ruby
from "debian"

inside "/dev" do
  run "mknod webscale c 1 3"
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

Note that if you set this before entrypoint, it will be cleared.

Example:

```ruby
from "debian"
# entrypoint is `/bin/sh -c` by default, so we will just run whatever command
# is thrown at us. This image will run `ls` in the workdir by default.
cmd "ls"
```

## copy

copy copies files from the host to the container. It only works relative to
the current directory. The build cache is calculated by summing the tar
result of edited files. Since mtime is also considered, changes to that will
also bust the cache.

NOTE: copy does not respect user permissions when the `user` or `with_user`
modifiers are applied. This will be fixed eventually.

Example:

```ruby
from "debian"

# recursively copies everything the cwd to test, which is relative to the
# workdir inside the container (`/` by default).
copy ".", "/test"
```
