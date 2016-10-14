Functions in Box provide a data-passing mechanism for build instructions. For
example, you may wish to read the contents of a file from the container into
your build for further processing; the `read` function allows that.

These are the functions supported by Box.

## getenv

getenv retrieves a value from the building environment (passed in as string)
and returns a string with the value. If no value exists, an empty string is
returned.

## read

read takes a filename as string, reads it from the latest image in the
evaluation and returns its data. Yields an error if the file does not exist
or from has not been called.

## getuid

getuid, given a string username provides an integer response with the UID of
the user. This works by reading the /etc/passwd file in the image.

Yields an error if it cannot find the user or from has not been called.

## getgid

getgid, given a string group name provides an integer response with the GID
of the group. This works by reading the /etc/group file in the image.

Yields an error if it cannot find the group or from has not been called.
