// Package lazypdf provides a MuPDF-based document page rasterizer. It is managed
// via the Rasterizer struct.
package lazypdf

import (
	"context"
	"errors"
	"fmt"
	"image"
	"time"
	"unsafe"

	log "github.com/sirupsen/logrus"
)

// #include <faster_raster.h>
import "C"

const (
	// We'll wait up to 10 seconds for a single page to Rasterize.
	RasterTimeout = 10 * time.Second

	LandscapeScale = 1.0
	PortraitScale  = 1.5
)

type rasterType int

const (
	RasterImage rasterType = iota
	RasterSVG
)

var (
	// A page was requested with an out of bounds page number.
	ErrBadPage = errors.New("invalid page number")
	// We tried to rasterize the page, but we gave up waiting.
	ErrRasterTimeout = errors.New("rasterizer timed out!")

	defaultExtension = C.CString(".pdf")
)

// IsBadPage validates that the type of error was an ErrBadPage.
func IsBadPage(err error) bool {
	return err == ErrBadPage
}

// IsRasterTimeout validates that the type of error was an ErrRasterTimeout.
func IsRasterTimeout(err error) bool {
	return err == ErrRasterTimeout
}

type ReplyWrapper interface {
	Error() error
}

type RasterRequest struct {
	ctx        context.Context
	PageNumber int
	Width      int
	Scale      float64
	RasterType rasterType
	ReplyChan  chan ReplyWrapper
}

type RasterReply struct {
	err error
}

func (r *RasterReply) Error() error {
	return r.err
}

type RasterImageReply struct {
	RasterReply
	Image image.Image
}

// Rasterizer is an actor that runs on an event loop processing a request channel.
// Replies are asynchronous from the standpoint of the internals of the library
// but the exported interface (via GeneratePage) is synchronous, using channels.
//
// If you need to use this asynchronously, you can directly insert entries into
// the RequestChan.
//
// Lifecycle:
//  * The event loop is started up by calling the Run() function, which will
//    allocate some resources and then start up a background Goroutine.
//  * You need to stop the event loop to remove the Goroutine and to free up
//    any resources that have been allocated in the Run() function.
type Rasterizer struct {
	Filename      string
	RequestChan   chan *RasterRequest
	Document      C.document_handle
	hasRun        bool
	scaleFactor   float64
	docPageCount  int
	quitChan      chan struct{}
	stopCompleted chan struct{}
}

// NewRasterizer returns a configured Rasterizer for a given filename,
// which uses a buffered channel of rasterBufferSize to queue requests
// for the rasterizer.
func NewRasterizer(filename string, rasterBufferSize int) *Rasterizer {
	return &Rasterizer{
		Filename:    filename,
		RequestChan: make(chan *RasterRequest, rasterBufferSize),
		quitChan:    make(chan struct{}),
	}
}

// generatePage is a synchronous interface to the processing engine. Asynchronous
// requests can be put directly into the RequestChan if needed rather than
// calling this function.
func (r *Rasterizer) generatePage(ctx context.Context, pageNumber int, width int, scale float64, rasterType rasterType) (ReplyWrapper, error) {
	if !r.hasRun {
		return nil, errors.New("Rasterizer has not been started!")
	}

	if r.quitChan == nil {
		return nil, errors.New("Rasterizer has been stopped!")
	}

	// Sanity check
	if r.Document == nil {
		return nil, errors.New("Rasterizer has been cleaned up! Cannot re-use")
	}

	// This channel must be buffered, or there is a race on the reply. If we
	// don't start listening on the channel yet by the time the reply comes, then
	// we will wait until the RasterTimeout and miss the returned response.
	replyChan := make(chan ReplyWrapper, 1)

	ctx, cancelFunc := context.WithTimeout(ctx, RasterTimeout)
	defer cancelFunc()

	// Pass the request to the rendering function via the channel
	req := RasterRequest{
		ctx:        ctx,
		PageNumber: pageNumber,
		Width:      width,
		Scale:      scale,
		RasterType: rasterType,
		ReplyChan:  replyChan,
	}
	select {
	// Try to send the request to the event loop
	case r.RequestChan <- &req:
		// Proceed to wait for the response
	case <-ctx.Done():
		// Bail out early if the processing pipeline is still full or the caller gave up
		return nil, ErrRasterTimeout
	}

	// Wait for a reply or a timeout, whichever occurs first
	select {
	case response := <-replyChan:
		close(replyChan)

		err := response.Error()
		if err != nil {
			return nil, err
		}
		return response, nil

	case <-ctx.Done():
		// We waited enough. Discard whatever we eventually render and bail out
		return nil, ErrRasterTimeout
	}
}

// GeneratePageImage is a synchronous interface to the processing engine and will
// return a Go stdlib image.Image.
func (r *Rasterizer) GeneratePageImage(ctx context.Context, pageNumber int, width int, scale float64) (image.Image, error) {
	response, err := r.generatePage(ctx, pageNumber, width, scale, RasterImage)
	if err != nil {
		return nil, err
	}

	return response.(*RasterImageReply).Image, nil
}

// GeneratePageSVG is a synchronous interface to the processing engine and will
// return a Go byte array containing the SVG string.
func (r *Rasterizer) GeneratePageSVG(ctx context.Context, pageNumber int, width int, scale float64) ([]byte, error) {
	return nil, errors.New("GeneratePageSVG not implemented")
}

// Run starts the main even loop after allocating some resources. This needs to be
// called before making any requests to rasterize pages.
func (r *Rasterizer) Run() error {
	// To prevent any leaking memory and nasty free/GC interactions, let's not allow
	// this to be recycled. Just make a new one instead.
	if r.hasRun {
		return errors.New("Rasterizer has already been run and cannot be recycled!")
	}
	r.hasRun = true

	// Allocate a C strings, from the Go filename. Free later
	cfilename := C.CString(r.Filename)
	defer C.free(unsafe.Pointer(cfilename))

	// Allocate/open a document in C and set it up to free later on
	// TODO: Handle defaultExtension
	r.Document = C.open_document(cfilename)

	if r.Document == nil {
		return errors.New("Unable to open document: " + r.Filename + "!")
	}

	r.docPageCount = int(C.get_num_pages(r.Document))

	go r.mainEventLoop()

	return nil
}

// GetPageCount returns the number of pages in the document
func (r *Rasterizer) GetPageCount() int {
	return r.docPageCount
}

// This is the main event loop for the rasterizer actor. It handles processing all
// three channels and makes sure we don't have any concurrency issues on the shared
// resources.
func (r *Rasterizer) mainEventLoop() {
	// Loop over the request channel, processing each entry in turn. This runs in the
	// background until the r.quitChan is closed.
OUTER:
	for {
		select {
		case req := <-r.RequestChan:
			if req == nil {
				continue // happens on channel close
			}
			r.processOne(req)
		case <-r.quitChan:
			r.quitChan = nil
			break OUTER
		}
	}

	r.finalCleanUp()
}

// finalCleanup is called the event loop has shut down and takes care of the
// cleanup of the document, channels, etc.
func (r *Rasterizer) finalCleanUp() {
	// Some final resource cleanup in C memory space
	if r.Document != nil {
		C.close_document(r.Document)
		r.Document = nil
	}

	// It's now safe to close these
	if r.RequestChan != nil {
		close(r.RequestChan)
		r.RequestChan = nil
	}

	// Used by tests that need to know when this is fully complete.
	// stopCompleted is not normally allocated so it will be nil.
	if r.stopCompleted != nil {
		close(r.stopCompleted)
	}
}

// Stop shuts down the rasterizer and frees up some common data structures that
// were allocated in the Run() method.
func (r *Rasterizer) Stop() {
	// Send the quit signal to the mainEventLoop goroutine for this Rasterizer
	if r.quitChan != nil {
		close(r.quitChan)
	}
}

// getRotation is used by tests to test the C rotation functions since you can't
// call Cgo directly from tests.
// func (r *Rasterizer) getRotation(pageNum int) (int, error) {
// 	page := C.load_page(r.Ctx, r.Document, C.int(pageNum-1))
// 	if page == nil {
// 		return 0, ErrBadPage
// 	}
// 	defer C.fz_drop_page(r.Ctx, page)

// 	rotation := C.get_rotation(r.Ctx, page)
// 	return int(rotation), nil
// }

// scalePage figures out how we're going to scale the page when rasterizing. If
// with width is set, we just do that. Otherwise if the scale is set we do that.
// Next we check the bounding box to find lanscape pages and scale them less.
// Finally we look at page rotation to see if it was rotated +/- 90 degrees. If
// it was rotated, we leave it PortraitScale.
// func (r *Rasterizer) scalePage(page *C.fz_page, bounds *C.fz_rect, req *RasterRequest) float64 {
// 	// It's nil when called from calculateScaleForDocument
// 	if req != nil {
// 		// If width is set, override any previous scale factor and use that explicitly
// 		if req.Width != 0 {
// 			return float64(C.float(req.Width) / bounds.x1)
// 		}

// 		// If the scale was requested, use that
// 		if req.Scale != 0 {
// 			return req.Scale
// 		}
// 	}

// 	// Figure out if it's landscape format, and scale by 1.0
// 	if (bounds.y1 - bounds.y0) < (bounds.x1 - bounds.x0) {
// 		// This purposely calls the C function not getRotation, which is only for tests
// 		rotation := C.get_rotation(r.Ctx, page)
// 		// Was it a rotated portrait page? If so, scale it PortraitScale
// 		if rotation != 0 && rotation != 180 { // Ignore weird rotations
// 			return PortraitScale
// 		}

// 		return LandscapeScale
// 	}

// 	return PortraitScale
// }

// calculateScaleForDocument goes through all the bounding boxes of all the pages
// in the document and tries to figure out if any of them are landscape pages. If
// so, it will default to LandscapeScale.
// func (r *Rasterizer) calculateScaleForDocument(pageCount int) {
// 	var page *C.fz_page
// 	for i := 0; i < pageCount; i++ {
// 		page = C.load_page(r.Ctx, r.Document, C.int(i))
// 		if page == nil {
// 			continue
// 		}

// 		bounds := C.fz_bound_page(r.Ctx, page)
// 		r.scaleFactor = r.scalePage(page, &bounds, nil)
// 		C.fz_drop_page(r.Ctx, page)

// 		if r.scaleFactor == LandscapeScale {
// 			break
// 		}
// 	}
// }

func (req *RasterRequest) sendErrorReply(filename string, err error) {
	select {
	case req.ReplyChan <- &RasterReply{err: err}:
		// nothing
	default:
		log.Warnf("Failed to send reply for %q page %d", filename, req.PageNumber)
	}
}

//  processOne does all the work of actually rendering a page and is run in a loop
//  from Run(). In rendering you can supply either the fixed output width, or a
//  scale factor. If not supplied, scale factor will default to 1.5. If supplied it
//  will be used. Width overrides any scale factor and will be rendered to as close
//  to that exact dimension as possible, if it's supplied.
func (r *Rasterizer) processOne(req *RasterRequest) {
	if r.quitChan == nil || r.Document == nil {
		req.sendErrorReply(r.Filename, fmt.Errorf("Tried to process a page from a closed document %q", r.Filename))
		return
	}

	if req.PageNumber < 1 || req.PageNumber > r.docPageCount {
		log.Warnf("Requested invalid page %d out of total page count %d from file %q", req.PageNumber, r.docPageCount, r.Filename)
		req.sendErrorReply(r.Filename, ErrBadPage)
		return
	}

	var width C.uint
	var height C.uint
	ret := C.get_page_dimensions(r.Document, C.int(req.PageNumber-1), C.double(1), &width, &height)
	if int(ret) > 0 {
		log.Warnf("Failed to get dimensions for page %d with code %d", req.PageNumber, int(ret))
		req.sendErrorReply(r.Filename, ErrBadPage)
		return
	}

	bytes := make([]byte, 4*int(width)*int(height))
	copiedBytes := C.render_page(r.Document, C.int(req.PageNumber-1), C.double(1), (*C.char)(unsafe.Pointer(&bytes[0])), C.ulong(len(bytes)))
	if int(copiedBytes) == 0 {
		log.Warnf("Failed to render page %d", req.PageNumber)
		req.sendErrorReply(r.Filename, ErrBadPage)
		return
	}

	goBounds := image.Rect(0, 0, int(width), int(height))

	rgbaImage := &image.RGBA{
		Pix: bytes, Stride: 4 * int(width), Rect: goBounds,
	}

	// r.backgroundRender(ctx, pixmap, list, bounds, &bbox, matrix, r.scaleFactor, bytes, req)
	// Try to reply, but don't get stuck if something happened to the channel
	select {
	case req.ReplyChan <- &RasterImageReply{Image: rgbaImage}:
		//nothing
	default:
		log.Warnf("Failed to reply for %s, page %d", r.Filename, req.PageNumber)
	}
}
