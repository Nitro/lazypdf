# LazyPDF
This is a rasterizing engine for PDF documents built around [MuPDF][mupdf] and [jemalloc][jemalloc]. Works on Linux and macOS.
<a target="_blank" href="https://icons8.com/icon/43610/copy">
  <img src="misc/assets/icon.png" align="right" height="96px" width="96px" />
</a>

## Using
Run the command `go get github.com/nitro/lazypdf/v2` to add the dependency to your project. The documentation can be found [here](https://pkg.go.dev/github.com/nitro/lazypdf/v2).

## Building
```golang
go build
```

## Testing
```golang
go test -race
```

## Updating the native libraries
To update MuPDF or jemalloc simply change its version at `misc/{library}/version` and submit the change at a pull request. GitHub Actions will then trigger the process of updating the library and headers through a series of commits at the pull request branch.

[mupdf]: https://mupdf.com
[jemalloc]: https://github.com/jemalloc/jemalloc