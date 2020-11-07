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

The only requirement is to build first MuPDF with the `./build` script.