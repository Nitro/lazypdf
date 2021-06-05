# LazyPDF
This is a rasterizing engine for PDF documents built around [MuPDF][mupdf]. Works on Linux and macOS.
<a target="_blank" href="https://icons8.com/icon/43610/copy">
  <img src="misc/assets/icon.png" align="right" height="96px" width="96px" />
</a>

## Required tool
`mutool` is required to execute this program.

For macOs: `brew install mutool`

## Building
```golang
go build
```

## Testing
```golang
go test -race
```
