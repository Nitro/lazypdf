// Package lazypdf provides a MuPDF-based document page rasterizer. It is managed
// via the Rasterizer struct.
package lazypdf

import (
	"context"
	"errors"
	"fmt"
	"image"
	"sync"
	"time"
	"unsafe"

	log "github.com/sirupsen/logrus"
)

// #cgo CFLAGS: -I/opt/mupdf/include
// #cgo CFLAGS: -I/opt/mupdf/include/mupdf
// #cgo CFLAGS: -I/opt/mupdf/thirdparty/openjpeg
// #cgo CFLAGS: -I/opt/mupdf/thirdparty/jbig2dec
// #cgo CFLAGS: -I/opt/mupdf/thirdparty/zlib
// #cgo CFLAGS: -I/opt/mupdf/thirdparty/jpeg
// #cgo CFLAGS: -I/opt/mupdf/thirdparty/freetype
//
// #cgo LDFLAGS: -L/opt/mupdf/build/release -lmupdf -lmupdf-third -lm -lcrypto -lpthread
// #cgo darwin LDFLAGS: -L/usr/local/opt/openssl/lib
//
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

type RasterSVGReply struct {
	RasterReply
	SVG []byte
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
	Filename           string
	RequestChan        chan *RasterRequest
	Ctx                *C.struct_fz_context_s
	Document           *C.struct_fz_document_s
	hasRun             bool
	locks              *C.fz_locks_context
	docPageCount       int
	quitChan           chan struct{}
	backgroundRenderWg sync.WaitGroup
	stopCompleted      chan struct{}
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
	if r.Ctx == nil || r.Document == nil {
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
	response, err := r.generatePage(ctx, pageNumber, width, scale, RasterSVG)
	if err != nil {
		return nil, err
	}

	return response.(*RasterSVGReply).SVG, nil
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

	// Set up the locks for the context. We need these to get it to let us do
	// multi-threaded processing. These are pthread_mutex_t in C. There are
	// FZ_LOCK_MAX of them (usually 4).
	r.locks = C.new_locks()

	if r.locks == nil {
		return errors.New("Unable to allocate locks!")
	}

	// Allocate a new context and free it later
	r.Ctx = C.cgo_fz_new_context(nil, r.locks, C.FZ_STORE_DEFAULT)

	// Register the default document type handlers
	C.fz_register_document_handlers(r.Ctx)

	// Allocate a C strings, from the Go filename. Free later
	cfilename := C.CString(r.Filename)
	defer C.free(unsafe.Pointer(cfilename))

	// Allocate/open a document in C and set it up to free later on
	r.Document = C.cgo_open_document(r.Ctx, cfilename, defaultExtension)

	if r.Document == nil {
		return errors.New("Unable to open document: " + r.Filename + "!")
	}

	r.docPageCount = int(C.fz_count_pages(r.Ctx, r.Document))

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
	// Wait for every backgroundRender operation to complete
	r.backgroundRenderWg.Wait()

	// Some final resource cleanup in C memory space
	if r.Document != nil {
		C.cgo_drop_document(r.Ctx, r.Document)
		r.Document = nil
	}

	if r.Ctx != nil {
		C.fz_drop_context(r.Ctx)
		r.Ctx = nil
	}

	// It's now safe to close these
	if r.RequestChan != nil {
		close(r.RequestChan)
		r.RequestChan = nil
	}

	C.free_locks(&r.locks)

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
func (r *Rasterizer) getRotation(pageNum int) (int, error) {
	page := C.load_page(r.Ctx, r.Document, C.int(pageNum-1))
	if page == nil {
		return 0, ErrBadPage
	}
	defer C.fz_drop_page(r.Ctx, page)

	rotation := C.get_rotation(r.Ctx, page)
	return int(rotation), nil
}

// scalePage figures out how we're going to scale the page when rasterizing. If
// with width is set, we just do that. Otherwise if the scale is set we do that.
// Next we check the bounding box to find lanscape pages and scale them less.
// Finally we look at page rotation to see if it was rotated +/- 90 degrees. If
// it was rotated, we leave it PortraitScale.
func (r *Rasterizer) scalePage(page *C.fz_page, bounds *C.fz_rect, req *RasterRequest) float64 {
	// It's nil when called from calculateScaleForDocument
	if req != nil {
		// If width is set, override any previous scale factor and use that explicitly
		if req.Width != 0 {
			return float64(C.float(req.Width) / bounds.x1)
		}

		// If the scale was requested, use that
		if req.Scale != 0 {
			return req.Scale
		}
	}

	// Figure out if it's landscape format, and scale by 1.0
	if (bounds.y1 - bounds.y0) < (bounds.x1 - bounds.x0) {
		// This purposely calls the C function not getRotation, which is only for tests
		rotation := C.get_rotation(r.Ctx, page)
		// Was it a rotated portrait page? If so, scale it PortraitScale
		if rotation != 0 && rotation != 180 { // Ignore weird rotations
			return PortraitScale
		}

		return LandscapeScale
	}

	return PortraitScale
}

func (req *RasterRequest) sendErrorReply(filename string, err error) {
	select {
	case req.ReplyChan <- &RasterReply{err: err}:
		// nothing
	default:
		log.Warnf("Failed to send reply for %q page %d", filename, req.PageNumber)
	}
}

// runCancellableOperation sets cookie.abort to 1 if the caller bailed out early to
// instruct MuPDF to terminate the pending operation as soon as possible
func (req *RasterRequest) runCancellableOperation(filename string, fn func(*C.fz_cookie)) error {
	if req.ctx == nil {
		// In this case, the caller doesn't want to bail out early
		fn(nil)

		return nil
	}

	var cookie C.fz_cookie
	done := make(chan struct{})
	go func() {
		select {
		case <-req.ctx.Done():
			cookie.abort = 1
		case <-done:
			// Processing finished
		}
	}()

	// Run our MuPDF function
	fn(&cookie)

	// Instruct the above goroutine to exit
	close(done)

	if err := req.ctx.Err(); err != nil || cookie.errors > 0 {
		log.Infof("Operation cancelled upstream: %s", err)
		req.sendErrorReply(filename, ErrBadPage)
		return ErrBadPage
	}

	return nil
}

//  processOne does all the work of actually rendering a page and is run in a loop
//  from Run(). In rendering you can supply either the fixed output width, or a
//  scale factor. If not supplied, scale factor will default to 1.5. If supplied it
//  will be used. Width overrides any scale factor and will be rendered to as close
//  to that exact dimension as possible, if it's supplied.
func (r *Rasterizer) processOne(req *RasterRequest) {
	if r.quitChan == nil || r.Ctx == nil || r.Document == nil {
		req.sendErrorReply(r.Filename, fmt.Errorf("Tried to process a page from a closed document %q", r.Filename))
		return
	}

	if req.PageNumber < 1 || req.PageNumber > r.docPageCount {
		log.Warnf("Requested invalid page %d out of total page count %d from file %q", req.PageNumber, r.docPageCount, r.Filename)
		req.sendErrorReply(r.Filename, ErrBadPage)
		return
	}

	// Create a clone of r.Ctx for objects that are allocated for rendering the current page
	// This needs to be disposed on all return paths below!
	ctx := C.fz_clone_context(r.Ctx)
	if ctx == nil {
		req.sendErrorReply(r.Filename, errors.New("failed to clone context"))
		return
	}

	// Load the page and allocate C structure, freed later
	// This needs to be disposed on all return paths below!
	page := C.load_page(ctx, r.Document, C.int(req.PageNumber-1))
	if page == nil {
		// Free the cloned context
		C.fz_drop_context(ctx)

		req.sendErrorReply(r.Filename, ErrBadPage)
		return
	}

	bounds := C.fz_bound_page(ctx, page)

	// Do the logic to figure out how we scale this thing.
	scaleFactor := r.scalePage(page, &bounds, req)

	matrix := C.fz_scale(C.float(scaleFactor), C.float(scaleFactor))

	bounds = C.fz_transform_rect(bounds, matrix)
	bbox := C.fz_round_rect(bounds)

	// Bail out early when rendering to SVG
	if req.RasterType == RasterSVG {
		// ctx and page will be disposed inside this method
		r.renderToSVG(ctx, page, &bounds, req)
		return
	}

	// Free list in backgroundRender when we're done with it
	list := C.fz_new_display_list(ctx, bounds)

	device := C.fz_new_list_device(ctx, list)

	err := req.runCancellableOperation(r.Filename,
		func(cookie *C.fz_cookie) {
			C.fz_run_page(ctx, page, device, C.fz_identity, cookie)
		},
	)
	C.fz_close_device(ctx, device)
	C.fz_drop_device(ctx, device)
	C.fz_drop_page(ctx, page)
	if err != nil {
		C.fz_drop_display_list(ctx, list)
		// Free the cloned context
		C.fz_drop_context(ctx)

		// runCancellableOperation already replied with an error
		return
	}

	bytes := make([]byte, 4*bbox.x1*bbox.y1)
	// We take the Go buffer we made and pass a pointer into the C lib.
	// This lets Go manage the buffer lifecycle. Bytes are written
	// back-to-back as RGBA starting at x,y,a 0,0,? .
	// We'll free this C structure in a defer block in backgroundRender
	pixmap := C.fz_new_pixmap_with_bbox_and_data(
		ctx, C.fz_device_rgb(ctx), bbox, nil, 1, (*C.uchar)(unsafe.Pointer(&bytes[0])),
	)
	C.fz_clear_pixmap_with_value(ctx, pixmap, C.int(0xff))

	// The rest we can background and let the main loop return to processing
	// any additional pages that have been requested!
	r.backgroundRenderWg.Add(1)
	go r.backgroundRender(ctx, pixmap, list, bounds, &bbox, matrix, scaleFactor, bytes, req)
}

// renderToSVG renders the requested page to an SVG string
// TODO: Check if we can do some of this processing async
func (r *Rasterizer) renderToSVG(ctx *C.struct_fz_context_s, page *C.fz_page, bounds *C.struct_fz_rect_s, req *RasterRequest) {
	// Clean up earlier allocated C data structures when we've completed this
	defer func() {
		C.fz_drop_page(ctx, page)

		// Free the cloned context
		C.fz_drop_context(ctx)
	}()

	// TODO: Optimise the initial size of the buffer. For example,
	// use 1024 as the initial value and cache the actual value if bigger.
	// Once a new page is requested, pass the cached value to fz_new_buffer.
	buf := C.fz_new_buffer(ctx, 1024)
	defer C.fz_drop_buffer(ctx, buf)

	out := C.fz_new_output_with_buffer(ctx, buf)
	defer C.fz_drop_output(ctx, out)

	// Use the default values for text_format and reuse_images from fz_new_svg_writer
	device := C.fz_new_svg_device(ctx, out, bounds.x1-bounds.x0, bounds.y1-bounds.y0, C.FZ_SVG_TEXT_AS_PATH, 1)

	err := req.runCancellableOperation(r.Filename,
		func(cookie *C.fz_cookie) {
			C.fz_run_page(ctx, page, device, C.fz_identity, cookie)
		},
	)

	C.fz_close_device(ctx, device)
	C.fz_drop_device(ctx, device)
	if err != nil {
		// runCancellableOperation already replied with an error
		return
	}

	// Fetch the SVG as a byte array instead of string (by calling fz_string_from_buffer),
	// so we won't have to convert it to a byte array later when passing it to a io.Writer.
	var bufferContents *C.uchar
	length := C.fz_buffer_storage(ctx, buf, &bufferContents)
	if length == 0 {
		req.sendErrorReply(r.Filename, errors.New("failed to fetch the SVG data"))
		return
	}
	svgBytes := C.GoBytes(unsafe.Pointer(bufferContents), C.int(length))

	// Try to reply, but don't get stuck if something happened to the channel
	select {
	case req.ReplyChan <- &RasterSVGReply{SVG: svgBytes}:
		//nothing
	default:
		log.Warnf("Failed to reply for %s, page %d", r.Filename, req.PageNumber)
	}
}

// backgroundRender handles the portion of the rasterization that can be done
// without access to the main context.
func (r *Rasterizer) backgroundRender(ctx *C.struct_fz_context_s,
	pixmap *C.struct_fz_pixmap_s,
	list *C.struct_fz_display_list_s,
	bounds C.struct_fz_rect_s,
	bbox *C.struct_fz_irect_s,
	matrix C.struct_fz_matrix_s,
	scaleFactor float64,
	bytes []byte,
	req *RasterRequest,
) {
	// Clean up earlier allocated C data structures when we've completed this
	defer func() {
		C.fz_drop_pixmap(ctx, pixmap)
		C.fz_drop_display_list(ctx, list)

		// Free the cloned context
		C.fz_drop_context(ctx)

		// Allow finalCleanUp to run
		r.backgroundRenderWg.Done()
	}()

	// Set up the draw device from the cloned context
	drawDevice := C.fz_new_draw_device(ctx, matrix, pixmap)

	// Take the commands from the display list and run them on the
	// draw device
	err := req.runCancellableOperation(r.Filename,
		func(cookie *C.fz_cookie) {
			C.fz_run_display_list(ctx, list, drawDevice, C.fz_identity, bounds, cookie)
		},
	)
	C.fz_close_device(ctx, drawDevice)
	C.fz_drop_device(ctx, drawDevice)
	if err != nil {
		// runCancellableOperation already replied with an error
		return
	}

	log.Debugf("Pixmap w: %d, h: %d, scale: %.2f", pixmap.w, pixmap.h, scaleFactor)

	goBounds := image.Rect(int(bbox.x0), int(bbox.y0), int(bbox.x1), int(bbox.y1))
	rgbaImage := &image.RGBA{
		Pix: bytes, Stride: int(C.cgo_ptr_cast(pixmap.stride)), Rect: goBounds,
	}

	// Try to reply, but don't get stuck if something happened to the channel
	select {
	case req.ReplyChan <- &RasterImageReply{Image: rgbaImage}:
		//nothing
	default:
		log.Warnf("Failed to reply for %s, page %d", r.Filename, req.PageNumber)
	}
}
