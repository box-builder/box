Verbs take action on a container and usually create a layer. Some commands can
be used to move data into and out of containers, or set properties and run
commands.


## set\_exec
`set_exec` sets both the entrypoint and cmd at the same time, allowing for no
race between the operations.

`set_exec` takes a dictionary consisting of two known elements as symbols:
entrypoint and cmd. They each take a string array which is then propagated
to the respective properties in the container's configuration.

## workdir

workdir sets the WorkingDir in the docker environment. It sets this
throughout the image creation; all run/copy statements will respect this
value. If you wish to break out or work within it further, look at the
`inside` call.

## user

user sets the username this container will use by default. It also affects
following run statements (but not copy, which always copies as root
currently). If you wish to switch to a user temporarily, consider using
`with_user`.

## flatten

flatten requires no argumemnts and flattens all layers and commits a new
layer. This is useful for reducing the size of images or making them easier
to distribute.

NOTE: flattening will always bust the build cache.

NOTE: flattening requires downloading the image and re-uploading it. This
can take a lot of time over remote connections and is not advised.

## tag

tag tags an image within the docker daemon, named after the string provided.
It must be a valid tag name.

## entrypoint

entrypoint sets the entrypoint for the image at runtime. It will not be
used for run invocations. Note that setting this clears any previously set cmd.

## from

from sets the initial image and if necessary, pulls it from the registry. It
also sets the initial layer and must be called before several operations.

## run

run runs a command and saves the layer.

It respects user and workdir, but not entrypoint and command. It does this
so it can respect the values provided in the script instead of what was
intended for the final image.

Cache keys are generated based on the command name, so to be certain your
command is run in the event of it hitting cache, run box with NO_CACHE=1.

## with\_user

`with_user`, when provided with a string username and block invokes commands
within the user's login context. Unfortunately, copy does not respect this
yet. It does not affect the final image.

Example:

```ruby
with_user "erikh" do
  run "vim +PluginInstall +qall"
end
```

## inside

inside, when provided with a directory name string and block, invokes
commands within the context of the working directory being set to the
string. It does not affect the final image.

Example:

```ruby
inside "/dev" do
  run "mknod webscale c 1 3"
end
```

## env

env, when provided with a hash of string => string key/value combinations,
will set the environment in the image and future run invocations.

Example:

```ruby
env "GOPATH" => "/go", "PATH" => "/usr/bin:/bin"
env GOPATH: "/go", PATH: "/usr/bin:/bin" # equivalent if you prefer this syntax
```

## cmd

cmd, when provided with a string will set the docker image's Cmd property,
which are the arguments that follow the entrypoint (and are overridden when
you provide a command to `docker run`). It does not affect run invocations.

Note that if you set this before entrypoint, it will be cleared.

## copy

copy copies files from the host to the container. It only works relative to
the current directory. The build cache is calculated by summing the tar
result of edited files. Since mtime is also considered, changes to that will
also bust the cache.

NOTE: copy does not respect inside or workdir right now, this is a bug.

NOTE: copy does not respect user permissions when the `user` or `with_user`
modifiers are applied. This is also a bug, but a much harder to fix one.

Example:

```ruby
copy ".", "test"
```
