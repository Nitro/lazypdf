# LazyPDF
![GitHub Workflow Status](https://img.shields.io/github/workflow/status/Nitro/lazypdf/CI/master?style=for-the-badge)
![License](https://img.shields.io/badge/license-agpl-green?style=for-the-badge)

This is a rasterizing engine for PDF documents, built around [MuPDF][mupdf]. It exports a Go interface that allows the creation of a `Rasterizer` for each document you wish to be able to rasterize to images.  It generated Go `image.Image`s and you can then render these as PNG/JPEG/etc by using the Go stdlib functions for doing that (`image/jpeg`, `image/png`).

# Building
```bash
make install-mupdf # Install muppdf
go build ./...     # Build the package
```

Installing
----------
The only requirement is to build first MuPDF with the `./build` script.

[mupdf]: https://mupdf.com