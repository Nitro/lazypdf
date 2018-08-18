package main

import (
	"image/png"
	"log"
	"os"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/Nitro/lazypdf"
)

var (
	pdf   = kingpin.Arg("pdf", "PDF file").Required().String()
	page  = kingpin.Flag("page", "page").Default("1").Short('p').Int()
	size  = kingpin.Flag("size", "size").Default("0").Short('s').Int()
	scale = kingpin.Flag("scale", "scale").Default("1.5").Short('S').Float()
	out   = kingpin.Flag("out", "out").Default("").Short('o').String()
)

func main() {
	kingpin.Parse()

	if *out == "" {
		*out = *pdf + ".png"
	}

	raster := lazypdf.NewRasterizer(*pdf)
	err := raster.Run()
	if err != nil {
		log.Fatalf("Failed to initialize the renderer: %s", err)
	}

	img, err := raster.GeneratePageImage(*page, *size, *scale)
	if err != nil {
		log.Fatalf("Render error: %s", err)
	}

	f, err := os.Create(*out)
	if err != nil {
		log.Fatalf("IO error: %s", err)
	}

	defer f.Close()
	png.Encode(f, img)
}
