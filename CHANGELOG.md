### v0.3.1

* Release version is reflected correctly in the binary

### v0.2.1

* Fix colorized output bleed for certain terminals on OS X.
* Fix run statements appropriately propagating when not supplied in the build plan
* Fix flatten statement to incorporate permissions when copying.
* Move to new official docker client.
* Clean up a file descriptor leak handling ruby files themselves.

### v0.2

* TTY detection (for colorized output and terminal handling) and flags to force it on (--force-tty) and off (--no-tty).
* -t/--tag flags to tag the final image with the tag name. Does not affect the tag verb in any way.
* -o/--omit can be used to filter functions/verbs from the capabilities of the builder.
* from statements now appropriately cause the image to inherit their attributes, such as CMD and ENV.
* debug: set a breakpoint in your build plan to drop into a shell. Placing this anywhere in your code, once called, will drop you into a container. Once the container terminates, its layer will be saved and the run will continue.
* import: import another file's ruby code.
* Colorized output! This provides a clearer visual experience and is appropriately turned off when no TTY is present.

### v0.1: Initial Release

This is the initial release of box! Huzzah!
