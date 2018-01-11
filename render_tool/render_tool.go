package main

import (
        "image/png"
        "os"
        "log"

        "gopkg.in/alecthomas/kingpin.v2"

        "github.com/Nitro/lazypdf"
)

var (
  pdf = kingpin.Arg("pdf", "PDF file").Required().String()
  page = kingpin.Flag("page", "page").Default("1").Short('p').Int()
  size = kingpin.Flag("size", "size").Default("0").Short('s').Int()
  scale = kingpin.Flag("scale", "scale").Default("1.5").Short('S').Float()
  out = kingpin.Flag("out", "out").Default("").Short('o').String()
)

func main() {
  kingpin.Version("1")
  kingpin.Parse()

  if *out == "" {
    *out = *pdf + ".png"
  }

  raster := lazypdf.NewRasterizer(*pdf)
  raster.Run()
  img, err := raster.GeneratePage(*page, *size, *scale)
  if err != nil {
    log.Fatalf("render error: %s", err)
  }
  f, err := os.Create(*out)
   if err != nil {
    log.Fatalf("io error: %s", err)
  }
  defer f.Close()
  png.Encode(f, img)
}
