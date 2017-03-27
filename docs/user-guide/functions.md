Functions in Box provide a data-passing mechanism for build instructions. For
example, you may wish to read the contents of a file from the container into
your build for further processing; the `read` function allows that.

These are the functions supported by Box.

## save

`save` saves an image with parameters:

* `tag`: tag the image in the docker image store.
* `file`: save the image to a file. The resulting file will be a bare tarball
  with the image contents, suitable for `docker load`.
* `type`: Two options: `docker`, and `oci`.

Example:

Tag an image with the name "foo":

```ruby
from "ubuntu"
save tag: "foo"
```

Save the ubuntu image to file after updating it:

```ruby
from "ubuntu"
run "apt-get update -qq && apt-get dist-upgrade -y"
save file: "ubuntu-with-update.tar"
```

## skip

skip skips all layers within its block in the final produced image, which may
be tagged with the `-t` commandline argument or modified in an `after` clause.

Note that any other tagging or references to images built will still be
available with full image contents, this only affects the final output image.

**WARNING**: This command can cause extreme latency over TCP connections as it
rebuilds images locally, which requires it to pull down and re-push any images.
It is strongly recommended you build on the host you wish to push from or use.

Example:

This will import the `debian` image, and run commands to install software
via `apt-get`. Then it will remove the update process's layer's contents from
the image, removing caches and other dirt from the final image.

```ruby
from "debian"

skip do
  run "apt-get update"
end

run "apt-get install tmux"
```

## import

import loads a ruby file, and then executes it as if it were a box plan. This
is principally used to modularize build instructions between multiple builds.

Note that this will load ruby files specified anywhere on the filesystem. Use
at your own risk. You can provide the `-o import` option to omit this function
from use.

Example:

File A:

```ruby
from "debian"
```

File B imports File A and builds on it:

```ruby
import "file-a.rb"
run "ls"
```

## getenv

getenv retrieves a value from the building environment (passed in as string)
and returns a string with the value. If no value exists, an empty string is
returned.

Example:

```ruby
# If you set IMAGE=ceph/rbd:latest in your environment, that would be pulled
# via the `from` statement.
from getenv("IMAGE")
```

## read

read takes a filename as string, reads it from the latest image in the
evaluation and returns its data. Yields an error if the file does not exist
or from has not been called.

read returns a string which may then be manipulated with normal string
manipulations in ruby.

Example:

```ruby
from "debian"
# this gets the first username in your passwd file inside the debian image
run "echo #{read("/etc/passwd").split("\n").first.split(":")[0]}"
```

## getuid

getuid, given a string username provides an integer response with the UID of
the user. This works by reading the /etc/passwd file in the image.

Yields an error if it cannot find the user or from has not been called.

Example:

```ruby
from "debian"
run "useradd -m -d /home/box-builder -s /bin/sh box-builder"
run "id #{getuid("box-builder")}"
```

## getgid

getgid, given a string group name provides an integer response with the GID
of the group. This works by reading the /etc/group file in the image.

Yields an error if it cannot find the group or from has not been called.

Example:

```ruby
from "debian"
run "groupadd cabal"
run "getent group #{getgid("cabal")}"
```
