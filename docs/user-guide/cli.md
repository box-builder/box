Command-Line Options in box alter behavior of box for its runtime.  Using these
options shape your build and suit it for use in many group environments.

In the documentation below, each long option is in the heading with the short
option (if any) in parentheses.

## Multi Mode

`box multi` will initiate multi-mode, which invokes multiple builds at the same
time.

## --help (-h) and --version (-v)

Show the help and version respectively.

## --no-cache (-n)

Turn caching off, this forces a rebuild of all build plan steps. Note that
this won't re-pull any pulled images.

Example:

```bash
$ box -n plan.rb
```

## --omit (-o)

Omit a function or verb from the DSL. This removes all functionality of a
specific mruby verb or function and causes a syntax error if encountered.  This
restricts certain operations in builds for teams or unprivileged scenarios.

Example:

```bash
$ cat >plan.rb <<EOF
from 'debian'
tag 'mydebian'
EOF
# boom - missing keyword or function from ruby
$ box -o tag plan.rb
```

## --tag (-t)

Tag the last generated image with the provided value. If the tag fails, the
build won't fail, but instead be untagged. However, box still exits
non-zero to indicate the tag failed.

Example:

```bash
# starts a build with debian and retags the result as 'mydebian'
echo "from 'debian'" | box -t mydebian
```

## --no-tty

Forcibly turn all tty operation/propagation off for this run. This will cause
many programs to behave different specifically in `run` statements, and the
`pull` animations for downloading will not be provided.

No TTY mode is the default when unix pipes are involved.

## --force-tty

Force the TTY on even if it is off for some reason.

The combination of `--no-tty --force-tty` is to force the tty.
