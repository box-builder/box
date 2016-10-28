Functions in Box provide a data-passing mechanism for build instructions. For
example, you may wish to read the contents of a file from the container into
your build for further processing; the `read` function allows that.

These are the functions supported by Box.

## import

import loads a ruby file, and then executes it as if it were a box plan. This
is prinicipally used to modularize build instructions between multiple builds.

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

```ruby
# If you set IMAGE=ceph/rbd:latest in your environment, that would be pulled
# via the `from` statement.
from getenv("IMAGE")
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
run "useradd -m -d /home/erikh -s /bin/sh erikh"
run "id #{getuid("erikh")}"
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
