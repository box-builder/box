terst
### v0.4.1

* Support .dockerignore files and per-copy-statement exclude/ignore statements[
* Support `from :scratch` and `from ""` as viable methods of using an empty container
* Compatibility fix around certain builder instructions setting the
  user/cmd/entrypoint/workdir incorrectly
* You can now suppress `run` statement output per-statement.
* New tarring routines capture special files, and other improvements in this area.
* New `after` verb which takes a proc of methods to be run after image composition.
* Fix for a bug where duplicate insertions of environment keys would cause the
  N++'d items to not be registered.
* Fix copying into volumes (by removing them all from the image)

### v0.4

* multi-build mode! Now build whole projects full of multiple images at once!
* many stability enhancements
  * in particular, you should not get random unpreventable panics when invoking
    box anymore. 

### v0.3.3

* Summing performance has improved across the board, which should drastically
	affect copy, flatten, and skip operations.
* Globbing on the left-hand-side of copy statements is now supported. Consult
	the documentation for more.
* The REPL/Shell now handles multi-line input more appropriately.

### v0.3.2

* Fix TTY handling in debug modes
* Improve signal handling in a few edge case scenarios in run statements
* USER, WORKDIR, CMD and ENTRYPOINT inheritance is much better now. It should be less
  surprising when issusing run statements the last layer in a series.
* Box no longer takes a final step to commit the image after the run has
  completed.
* New progress meters for all copy/tar/summing operations. 
* Tarring routines (copy, flatten etc) no longer attempt to tar special files
  such as unix sockets.
* Many fixes around copy, path handling and workdir. Note that now if you want
  to copy files into a target that is a directory, it will fail. If you do wish
  to copy them into the directory instead of over it, suffix the directory name
  with a `/`.

### v0.3.1

* Release version is reflected correctly in the binary

### v0.3
* New REPL/shell! You can now interactively build container images with box.
* New skip verb: skip layers that you don't want in the final image.
* Improved signal handling; canceling builds now leaves no temporary files or
  containers within the system.
* A new command-line flag, `box -f`, omits the automatic final commit. It is
  typically used with the `tag` verb to avoid making two images.
* The readability of progress meters was improved. 


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
