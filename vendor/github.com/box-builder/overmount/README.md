## overmount - mount tars in an overlay filesystem

[![GoDoc](https://godoc.org/github.com/box-builder/overmount?status.svg)](https://godoc.org/github.com/box-builder/overmount)
[![Build Status](http://jenkins.hollensbe.org:8080/job/overmount-master/badge/icon)](http://jenkins.hollensbe.org:8080/job/overmount-master/)

overmount is intended to mount docker images, or work with similar
functionality to achieve a series of layered filesystems which can be composed
into an image.

See the [examples](https://github.com/box-builder/overmount/tree/master/examples)
directory for examples of how to use the API.

## installation

It is strongly recommended to use a vendoring tool to pin at a specific commit.
We will tag releases on an as-needed basis, but there is no notion of backwards
compat between versions.

## author

Erik Hollensbe <github@hollensbe.org>
