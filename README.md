LazyPDF
=======

[![](https://travis-ci.org/Nitro/lazypdf.svg?branch=master)](https://travis-ci.org/Nitro/lazypdf)

This is a rasterizing engine for PDF documents, built around MuPDF. It exports
a Go interface that allows the creation of a `Rasterizer` for each document you
wish to be able to rasterize to images.  It generated Go `image.Image`s and you
can then render these as PNG/JPEG/etc by using the Go stdlib functions for
doing that (`image/jpeg`, `image/png`).

Building
--------

You can simply run `./build` and the shell script will do all the right things
on either Linux or OSX.

Installing
----------

You cannot simply `go get` this library. It pulls in and links against C code
which is external to this repository for licensing reasons (the MuPDF library
is AGPL). Thus, in order to build this library, you need to run the `build`
shell script to pull down and compile MuPDF first.
