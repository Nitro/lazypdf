LazyPDF
=======

This is a rasterizing engine for PDF documents, built around MuPDF. It exports
a Go interface that allows the creation of a `Rasterizer` for each document you
wish to be able to rasterize to images.  It generated Go `image.Image`s and you
can then render these as PNG/JPEG/etc by using the Go stdlib functions for
doing that (`image/jpeg`, `image/png`).

Installing
----------

You cannot simply `go get` this library. It pulls in and links against C code
which is external to this repository for licensing reasons (the MuPDF library
is AGPL). Thus, in order to build this library, you need to run the `build`
shell script to pull down and compile MuPDF first.

Anything that you then want
to link against this library will need to use the correct Cgo configuration.
That should look like:

```go
// #cgo CFLAGS: -I<relative path> -I<relative path>/mupdf-1.11-source/include -I<relative path>/mupdf-1.11-source/include/mupdf -I<relative path>/mupdf-1.11-source/thirdparty/openjpeg -I<relative path>/mupdf-1.11-source/thirdparty/jbig2dec -I<relative path>/mupdf-1.11-source/thirdparty/zlib -I<relative path>/mupdf-1.11-source/thirdparty/jpeg -I<relative path>/mupdf-1.11-source/thirdparty/freetype
// #cgo LDFLAGS: -L<relative path>/mupdf-1.11-source/build/release -L<relative path>/mupdf-1.11-source/thirdparty/openjpeg/bin -lmupdf -lmupdfthird -lmutools -lmupdf-js-none -lm -ljbig2dec -lz -lfreetype -ljpeg -lcrypto -lpthread
// #include <faster_raster.h>
import "C"
```

`<relative path>` in this case should be the relative location of this library
to your project. If you are building a Go project under the `github.com/Nitro`
folder, this would usually be `../lazypdf`.

License
-------

This is Copyright 2017 Nitro Software. All Rights Reserved.

This is private and confidential, not for public release.
