# LazyPDF
This is a rasterizing engine for PDF documents built around [MuPDF][mupdf] and [jemalloc][jemalloc].

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

## Supported environments
- Linux amd64
- MacOS arm64

## Updating the native libraries
To update MuPDF or jemalloc simply change its version at `misc/{library}/version` and submit the change at a pull request. GitHub Actions will then trigger the process of updating the library and headers through a series of commits at the pull request branch.

[mupdf]: https://mupdf.com
[jemalloc]: https://github.com/jemalloc/jemalloc
