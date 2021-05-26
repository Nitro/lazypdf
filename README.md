# LazyPDF
This is a rasterizing engine for PDF documents built around [MuPDF][mupdf]. Works on Linux and macOS.
<a target="_blank" href="https://icons8.com/icon/43610/copy">
  <img src="misc/assets/icon.png" align="right" height="96px" width="96px" />
</a>

## Building
```golang
go build
```

## Testing
```golang
go test -race
```

## Updating MuPDF library
To update MuPDF library simply change its version at [misc/mupdf/version](misc/mupdf/version) and submit the change at a pull request. GitHub Actions will then trigger the process of updating the library and headers through a series of commits at the pull request branch.

[mupdf]: https://mupdf.com
