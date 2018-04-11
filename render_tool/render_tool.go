package main

import (
	"fmt"
	"image/png"
	"io/ioutil"
	"log"
	"os"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/Nitro/lazypdf"
)

var (
	pdf    = kingpin.Arg("pdf", "PDF file").Required().String()
	page   = kingpin.Flag("page", "page").Default("1").Short('p').Int()
	size   = kingpin.Flag("size", "size").Default("0").Short('s').Int()
	scale  = kingpin.Flag("scale", "scale").Default("1.5").Short('S').Float()
	out    = kingpin.Flag("out", "out").Default("").Short('o').String()
	format = kingpin.Flag("format", "format").Default("png").Short('f').String()
)

func main() {
	kingpin.Parse()

	if *out == "" {
		*out = *pdf + "." + *format
	}

	raster := lazypdf.NewRasterizer(*pdf)
	raster.Run()

	if *format == "png" {
		img, err := raster.GeneratePage(*page, *size, *scale)
		if err != nil {
			log.Fatalf("Render error: %s", err)
		}

		f, err := os.Create(*out)
		if err != nil {
			log.Fatalf("IO error: %s", err)
		}

		defer f.Close()
		png.Encode(f, img)

	} else if *format == "svg" {
		svg := raster.GetSVG(*page)
		err := ioutil.WriteFile(fmt.Sprintf("%s_%d.svg", *pdf, *page), svg, 0644)
		if err != nil {
			log.Fatalf("IO error: %s", err)
		}
	}
}
