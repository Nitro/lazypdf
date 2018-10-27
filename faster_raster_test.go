package lazypdf

import (
	"context"
	"image"
	"sync"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func Test_NewRasterizer(t *testing.T) {
	Convey("NewRasterizer()", t, func() {
		Convey("returns a properly configured struct", func() {
			raster := NewRasterizer("foo", 1)

			So(raster.Filename, ShouldEqual, "foo")
			So(raster.RequestChan, ShouldNotBeNil)
			So(raster.quitChan, ShouldNotBeNil)
			So(raster.hasRun, ShouldBeFalse)
		})
	})
}

func Test_Run(t *testing.T) {
	Convey("When Running and Stopping", t, func() {
		Convey("rasterizer starts without error", func() {
			raster := NewRasterizer("fixtures/sample.pdf", 1)
			err := raster.Run()

			So(err, ShouldBeNil)
			raster.Stop()
		})

		Convey("rasterizer stops", func() {
			raster := NewRasterizer("fixtures/sample.pdf", 1)
			err := raster.Run()
			So(err, ShouldBeNil)

			// We have to give the background goroutine a little time to start :(
			time.Sleep(5 * time.Millisecond)

			raster.stopCompleted = make(chan struct{}) // Get notified when it's all stopped
			raster.Stop()

			<-raster.stopCompleted
			So(raster.RequestChan, ShouldBeNil)
			So(raster.quitChan, ShouldBeNil)
			So(raster.locks, ShouldBeNil)
			So(raster.hasRun, ShouldBeTrue)
		})
	})
}

func Test_getRotation(t *testing.T) {
	Convey("Identifies the page rotation", t, func() {
		raster := NewRasterizer("fixtures/rotated-sample.pdf", 1)
		err := raster.Run()
		So(err, ShouldBeNil)

		_, err = raster.getRotation(0)
		So(err, ShouldEqual, ErrBadPage)

		rot, err := raster.getRotation(1)
		So(err, ShouldBeNil)
		So(rot, ShouldEqual, 180)

		rot, err = raster.getRotation(2)
		So(err, ShouldBeNil)
		So(rot, ShouldEqual, 0)

		raster.Stop()
	})
}

func Test_scalePage(t *testing.T) {
	Convey("Doing silly calculations on page rotation and scaling", t, func() {
		Convey("handles landscape pages", func() {
			Convey("as LandscapeScale when pages really are", func() {
				raster := NewRasterizer("fixtures/landscape-sample.pdf", 1)
				err := raster.Run()
				So(err, ShouldBeNil)

				img, err := raster.GeneratePageImage(context.Background(), 1, 0, 0)

				So(err, ShouldBeNil)
				So(img.Bounds().Max.X, ShouldEqual, 842)

				raster.Stop()
			})

			Convey("as PortraitScale when pages were rotated", func() {
				raster := NewRasterizer("fixtures/rotated-sample.pdf", 1)
				err := raster.Run()
				So(err, ShouldBeNil)
				img, err := raster.GeneratePageImage(context.Background(), 1, 0, 0)

				So(err, ShouldBeNil)
				So(img.Bounds().Max.X, ShouldEqual, 842)

				raster.Stop()
			})
		})

		Convey("handles portrait pages as PortraitScale", func() {
			raster := NewRasterizer("fixtures/sample.pdf", 1)
			err := raster.Run()
			So(err, ShouldBeNil)
			img, err := raster.GeneratePageImage(context.Background(), 1, 0, 0)

			So(err, ShouldBeNil)
			So(img.Bounds().Max.X, ShouldEqual, 893)

			raster.Stop()
		})

		Convey("uses LandscapeScale if any page is landscape", func() {
			raster := NewRasterizer("fixtures/mixed-sample.pdf", 1)
			err := raster.Run()
			So(err, ShouldBeNil)
			img, err := raster.GeneratePageImage(context.Background(), 2, 0, 0)

			So(err, ShouldBeNil)
			So(img.Bounds().Max.X, ShouldEqual, 612)

			raster.Stop()
		})

		Convey("uses specified scale if there is one", func() {
			raster := NewRasterizer("fixtures/mixed-sample.pdf", 1)
			err := raster.Run()
			So(err, ShouldBeNil)
			img, err := raster.GeneratePageImage(context.Background(), 2, 0, 0.5)

			So(err, ShouldBeNil)
			So(img.Bounds().Max.X, ShouldEqual, 306)

			raster.Stop()
		})
	})
}

func Test_WithoutFileExtensions(t *testing.T) {
	Convey("When the file has no file extension", t, func() {
		Convey("but it is a PDF file, so it should work", func() {
			raster := NewRasterizer("fixtures/sample_no_extension", 1)
			err := raster.Run()
			So(err, ShouldBeNil)

			_, err = raster.GeneratePageImage(context.Background(), 1, 1024, 0)
			So(err, ShouldBeNil)

			raster.Stop()
		})

		Convey("but it is hot garbage, so it should fail", func() {
			raster := NewRasterizer("fixtures/bad_data", 1)
			err := raster.Run()
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "Unable to open document")

			_, err = raster.GeneratePageImage(context.Background(), 1, 1024, 0)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "has been cleaned up")

			raster.Stop()
		})
	})
}

func Test_Processing(t *testing.T) {
	Convey("When processing the file", t, func() {
		// Allow up to 10 parallel requests
		raster := NewRasterizer("fixtures/sample.pdf", 10)

		Convey("returns an error when the rasterizer has not started", func() {
			_, err := raster.GeneratePageImage(context.Background(), 1, 1024, 0)

			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "has not been started")
		})

		Convey("returns an error on page out of bounds", func() {
			err := raster.Run()
			So(err, ShouldBeNil)
			So(raster.docPageCount, ShouldEqual, 2)

			img, err := raster.GeneratePageImage(context.Background(), 3, 1024, 0)

			So(img, ShouldBeNil)
			So(err, ShouldEqual, ErrBadPage)

			img, err = raster.GeneratePageImage(context.Background(), 0, 1024, 0)
			So(img, ShouldBeNil)
			So(err, ShouldEqual, ErrBadPage)

			raster.Stop()
		})

		Convey("returns an image and no error when things go well", func() {
			if testing.Short() {
				return
			}

			err := raster.Run()
			So(err, ShouldBeNil)

			img, err := raster.GeneratePageImage(context.Background(), 2, 1024, 0)

			So(err, ShouldBeNil)
			So(img, ShouldNotBeNil)
			raster.Stop()
		})

		Convey("returns an SVG and no error when things go well", func() {
			if testing.Short() {
				return
			}

			err := raster.Run()
			So(err, ShouldBeNil)

			svg, err := raster.GeneratePageSVG(context.Background(), 2, 1024, 0)

			So(err, ShouldBeNil)
			So(string(svg), ShouldStartWith, `<?xml version="1.0" encoding="UTF-8" standalone="no"?>`)
			So(string(svg), ShouldContainSubstring, "</clipPath>")
			So(string(svg), ShouldEndWith, "</svg>\n")
			raster.Stop()
		})

		Convey("returns an error when the rasterizer has been stopped", func() {
			err := raster.Run()
			So(err, ShouldBeNil)

			// We have to give the background goroutine a little time to start :(
			time.Sleep(5 * time.Millisecond)

			raster.stopCompleted = make(chan struct{})
			go raster.Stop()
			<-raster.stopCompleted

			_, err = raster.GeneratePageImage(context.Background(), 1, 1024, 0)

			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "has been stopped")
		})

		Convey("returns an error when the rasterizer is started twice", func() {
			err := raster.Run()
			So(err, ShouldBeNil)

			err = raster.Run()
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "has already been run and cannot be recycled")

			raster.Stop()
		})

		Convey("returns an image and doesn't hang when stopping before the async render is complete", func() {
			if testing.Short() {
				return
			}

			err := raster.Run()
			So(err, ShouldBeNil)

			// We have to give the background goroutine a little time to start :(
			time.Sleep(5 * time.Millisecond)

			replyChan := make(chan ReplyWrapper, 1)

			// Pass the request to the rendering function via the channel
			raster.RequestChan <- &RasterRequest{
				PageNumber: 2,
				Width:      1024,
				Scale:      0,
				ReplyChan:  replyChan,
			}

			raster.stopCompleted = make(chan struct{})
			go raster.Stop()
			<-raster.stopCompleted

			// Wait for a reply or a timeout, whichever occurs first
			timeoutOccured := false
			var response ReplyWrapper
			select {
			case response = <-replyChan:
				close(replyChan)
			case <-time.After(RasterTimeout):
				timeoutOccured = true
			}
			So(timeoutOccured, ShouldBeFalse)
			So(response, ShouldNotBeNil)
			So(response.Error(), ShouldBeNil)
			So(response.(*RasterImageReply).Image, ShouldNotBeNil)
		})

		Convey("returns an image with the correct width when specified", func() {
			if testing.Short() {
				return
			}

			err := raster.Run()
			So(err, ShouldBeNil)
			img, err := raster.GeneratePageImage(context.Background(), 2, 1024, 0)

			So(err, ShouldBeNil)
			So(img, ShouldNotBeNil)

			So(img.Bounds().Max.X, ShouldEqual, 1024)
			raster.Stop()
		})

		Convey("returns an image with the correct scale factor when specified", func() {
			if testing.Short() {
				return
			}

			err := raster.Run()
			So(err, ShouldBeNil)
			img, err := raster.GeneratePageImage(context.Background(), 2, 0, 1.1)

			So(err, ShouldBeNil)
			So(img, ShouldNotBeNil)

			So(img.Bounds().Max.X, ShouldEqual, 655)
			raster.Stop()
		})

		Convey("the width takes precedence over the scale factor", func() {
			if testing.Short() {
				return
			}

			err := raster.Run()
			So(err, ShouldBeNil)
			img, err := raster.GeneratePageImage(context.Background(), 2, 1024, 1.1) // Specify BOTH

			So(err, ShouldBeNil)
			So(img, ShouldNotBeNil)

			So(img.Bounds().Max.X, ShouldEqual, 1024) // Should match -> width <-
			raster.Stop()
		})

		Convey("figures out the scale factor based on page format", func() {
			if testing.Short() {
				return
			}

			// PORTRAIT
			err := raster.Run()
			So(err, ShouldBeNil)
			img, err := raster.GeneratePageImage(context.Background(), 2, 0, 0) // Specify NEITHER scale nor width

			So(err, ShouldBeNil)
			So(img, ShouldNotBeNil)

			So(img.Bounds().Max.X, ShouldEqual, 893) // Portrait file, should be 1.5 scaling
			raster.Stop()

			// LANDSCAPE
			raster = NewRasterizer("fixtures/landscape-sample.pdf", 1)
			err = raster.Run()
			So(err, ShouldBeNil)

			img, err = raster.GeneratePageImage(context.Background(), 1, 0, 0) // Specify NEITHER scale nor width
			So(img, ShouldNotBeNil)
			So(err, ShouldBeNil)

			So(img.Bounds().Max.X, ShouldEqual, 842) // Landscape file, should be 1.0 scaling
			raster.Stop()
		})

		Convey("counts the number of pages in the document when raster starts", func() {
			err := raster.Run()
			So(err, ShouldBeNil)

			So(raster.docPageCount, ShouldEqual, 2)

			raster.Stop()
		})

		Convey("handles more than one page image at a time", func() {
			if testing.Short() {
				return
			}

			err := raster.Run()
			So(err, ShouldBeNil)

			var err1, err2, err3, err4 error
			var img1, img2, img3, img4 image.Image

			var wg sync.WaitGroup
			wg.Add(8)

			go func() {
				img1, err1 = raster.GeneratePageImage(context.Background(), 1, 1024, 0)
				wg.Done()
			}()

			go func() {
				img2, err2 = raster.GeneratePageImage(context.Background(), 1, 1024, 0)
				wg.Done()
			}()

			go func() {
				img3, err3 = raster.GeneratePageImage(context.Background(), 2, 1024, 0)
				wg.Done()
			}()

			go func() {
				img4, err4 = raster.GeneratePageImage(context.Background(), 2, 1024, 0)
				wg.Done()
			}()

			// Generate some more contention
			for i := 0; i < 4; i++ {
				page := i%2 + 1
				go func() {
					// Ignore the image and error here for now
					_, _ = raster.GeneratePageImage(context.Background(), page, 1024, 0)
					wg.Done()
				}()
			}

			wg.Wait()

			So(err1, ShouldBeNil)
			So(err2, ShouldBeNil)
			So(err3, ShouldBeNil)
			So(err4, ShouldBeNil)

			// Checking these using ShouldNotBeNil is really slow...
			So(img1 != nil, ShouldBeTrue)
			So(img2 != nil, ShouldBeTrue)
			So(img3 != nil, ShouldBeTrue)
			So(img4 != nil, ShouldBeTrue)

			raster.Stop()
		})

		Convey("handles both image and SVG rasterisation simultaneously", func() {
			if testing.Short() {
				return
			}

			err := raster.Run()
			So(err, ShouldBeNil)

			var err1, err2 error
			var img image.Image
			var svg []byte

			var wg sync.WaitGroup
			wg.Add(2)

			go func() {
				img, err1 = raster.GeneratePageImage(context.Background(), 1, 1024, 0)
				wg.Done()
			}()

			go func() {
				svg, err2 = raster.GeneratePageSVG(context.Background(), 1, 1024, 0)
				wg.Done()
			}()

			wg.Wait()

			So(err1, ShouldBeNil)
			So(err2, ShouldBeNil)

			// Checking img using ShouldNotBeNil is really slow...
			So(img != nil, ShouldBeTrue)
			So(svg, ShouldNotBeNil)

			raster.Stop()
		})
	})
}
