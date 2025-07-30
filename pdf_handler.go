// nolint
package lazypdf

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	ddTracer "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

/*
#cgo CFLAGS: -I ${SRCDIR}/misc/mupdf/include -I ${SRCDIR}/misc/mupdf/include/mupdf -I ${SRCDIR}/misc/jemalloc/include -I ${SRCDIR}/misc/jemalloc/include/jemalloc
#cgo linux,amd64 LDFLAGS: -L ${SRCDIR}/misc/mupdf/lib/x86-64-linux -lmupdf -lmupdf-third -L ${SRCDIR}/misc/jemalloc/lib/x86-64-linux -ljemalloc -lm -lpthread -ldl
#cgo darwin,arm64 LDFLAGS: -L ${SRCDIR}/misc/mupdf/lib/arm64-macos -lmupdf -lmupdf-third -L ${SRCDIR}/misc/jemalloc/lib/arm64-macos -ljemalloc -lm -lpthread -ldl
#include <jemalloc/jemalloc.h>
#include "pdf_handler.h"
*/
import "C"

type PdfHandler struct {
	Logger *slog.Logger
	ctx    context.Context
}

// NewPdfHandler creates a new PdfHandler with the given context and logger
func NewPdfHandler(ctx context.Context, logger *slog.Logger) *PdfHandler {
	return &PdfHandler{
		Logger: logger,
		ctx:    ctx,
	}
}

// NewPdfHandlerWithLogger creates a new PdfHandler with background context and the given logger
// Deprecated: Use NewPdfHandler instead
func NewPdfHandlerWithLogger(logger *slog.Logger) *PdfHandler {
	return NewPdfHandler(context.Background(), logger)
}

type PdfDocument struct {
	handle       C.uintptr_t
	file         string
	wrappedPages map[int]bool
	mu           sync.RWMutex
}

type Location struct {
	X float64
	Y float64
}

type Size struct {
	Width  float64
	Height float64
}

type ImageParams struct {
	Page int
	// Specify location as percentages relative to page dimensions:
	//   (0,0) represents the upper-left corner.
	//   (1,1) represents the bottom-right corner.
	Location Location
	// Specify size as a percentage of page dimensions:
	//   0 represents zero size.
	//   1 represents the full page width or height
	Size      Size
	ImagePath string
}

type TextParams struct {
	Value string
	Page  int
	// Specify location as percentages relative to page dimensions:
	//   (0,0) represents the upper-left corner.
	//   (1,1) represents the bottom-right corner.
	Location Location
	// Set the text bounding box size as a percentage of the page size:
	//   0 represents zero size.
	//   1 represents the full page width or height
	Size Size
	Font struct {
		Family string
		Size   float64 // In "Point" where 1 point = 1/72 inch
	}
}

type CheckboxParams struct {
	Value bool
	Page  int
	// Specify location as percentages relative to page dimensions:
	//   (0,0) represents the upper-left corner.
	//   (1,1) represents the bottom-right corner.
	Location Location
	// Specify size as a percentage of page dimensions:
	//   0 represents zero size.
	//   1 represents the full page width or height
	Size Size
}

type PageSize struct {
	Width  float64 // In "Point" where 1 point = 1/72 inch
	Height float64 // In "Point" where 1 point = 1/72 inch
}

var standardFontList = []struct {
	Name      string
	Descender float64
}{
	{"Courier", -194},
	{"Courier-Oblique", -194},
	{"Courier-Bold", -194},
	{"Courier-BoldOblique", -194},
	{"Helvetica", -207},
	{"Helvetica-Oblique", -207},
	{"Helvetica-Bold", -207},
	{"Helvetica-BoldOblique", -207},
	{"Times-Roman", -219},
	{"Times-Italic", -217},
	{"Times-Bold", -218},
	{"Times-BoldItalic", -222},
	{"Symbol", -293},
	{"ZapfDingbats", -143},
}

func savePayloadToTempFile(ctx context.Context, r io.Reader) (filename string, err error) {
	span, _ := ddTracer.StartSpanFromContext(ctx, "savePayloadToTempFile")
	defer func() { span.Finish(ddTracer.WithError(err)) }()

	if r == nil {
		return "", errors.New("payload can't be nil")
	}

	tmpFile, err := os.CreateTemp("", "pdf_handler_*.pdf")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tmpFile.Close()

	if _, err = io.Copy(tmpFile, r); err != nil {
		removeErr := os.Remove(tmpFile.Name())
		if removeErr != nil {
			return "", fmt.Errorf("failed to write payload to temp file: %w; also failed to remove temp file: %v", err, removeErr)
		}
		return "", fmt.Errorf("failed to write payload to temp file: %w", err)
	}

	return tmpFile.Name(), nil
}

// Percentages relative to page dimensions to PDF Point
func (p *PdfHandler) LocationSizeToPdfPoints(ctx context.Context, document PdfDocument, page int, x, y, width, height float64) (float64, float64, float64, float64, error) {
	span, _ := ddTracer.StartSpanFromContext(ctx, "PdfHandler.LocationSizeToPdfPoints")
	defer span.Finish()

	pageSize, err := p.GetPageSizeWithContext(ctx, document, page)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("failed to get page size: %w", err)
	}
	if x < 0 || x > 1 || y < 0 || y > 1 || width < 0 || width > 1 || height < 0 || height > 1 {
		return 0, 0, 0, 0, fmt.Errorf("invalid input percentages: x=%f, y=%f, width=%f, height=%f", x, y, width, height)
	}
	return x * pageSize.Width,
		(1.0 - y - height) * pageSize.Height,
		width * pageSize.Width,
		height * pageSize.Height,
		nil
}

func (p *PdfHandler) OpenPDF(rawPayload io.Reader) (document PdfDocument, err error) {
	span, ctx := ddTracer.StartSpanFromContext(p.ctx, "PdfHandler.OpenPDF")
	defer func() { span.Finish(ddTracer.WithError(err)) }()

	filename, err := savePayloadToTempFile(ctx, rawPayload)
	if err != nil {
		return PdfDocument{}, err
	}

	cFilename := C.CString(filename)
	defer C.free(unsafe.Pointer(cFilename))

	input := C.openPDFInput{
		filename: cFilename,
	}

	// Measure C function call performance
	cCallStart := time.Now()
	output := C.open_pdf(input)
	cCallDuration := time.Since(cCallStart)

	// Add performance metrics to trace
	span.SetTag("c_function", "open_pdf")
	span.SetTag("c_call_duration_ms", float64(cCallDuration.Nanoseconds())/1e6)

	if output.error != nil {
		defer C.je_free(unsafe.Pointer(output.error))
		span.SetTag("c_function_error", true)
		return PdfDocument{}, fmt.Errorf("failure at the C/MuPDF open_pdf function: %s", C.GoString(output.error))
	}

	pdf := PdfDocument{
		handle:       output.handle,
		file:         filename,
		wrappedPages: make(map[int]bool),
	}
	return pdf, nil
}

func (p *PdfHandler) ClosePDF(document PdfDocument) (err error) {
	span, _ := ddTracer.StartSpanFromContext(p.ctx, "PdfHandler.ClosePDF")
	defer func() { span.Finish(ddTracer.WithError(err)) }()

	pdf := C.pdfDocument{
		handle: document.handle,
		error:  nil,
	}

	// Measure C function call performance
	cCallStart := time.Now()
	output := C.close_pdf(pdf)
	cCallDuration := time.Since(cCallStart)

	// Add performance metrics to trace
	span.SetTag("c_function", "close_pdf")
	span.SetTag("c_call_duration_ms", float64(cCallDuration.Nanoseconds())/1e6)

	removeErr := os.Remove(document.file)

	var errs []error
	if output.error != nil {
		defer C.je_free(unsafe.Pointer(output.error))
		span.SetTag("c_function_error", true)
		errs = append(errs, fmt.Errorf("close_pdf failed: %s", C.GoString(output.error)))
	}
	if removeErr != nil {
		errs = append(errs, fmt.Errorf("failed to remove temp file: %w", removeErr))
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (p *PdfHandler) GetPageSize(document PdfDocument, page int) (pageSize PageSize, err error) {
	span, ctx := ddTracer.StartSpanFromContext(p.ctx, "PdfHandler.GetPageSize")
	defer func() { span.Finish(ddTracer.WithError(err)) }()

	return p.GetPageSizeWithContext(ctx, document, page)
}

func (p *PdfHandler) GetPageSizeWithContext(ctx context.Context, document PdfDocument, page int) (pageSize PageSize, err error) {
	span, _ := ddTracer.StartSpanFromContext(ctx, "PdfHandler.GetPageSize")
	defer func() { span.Finish(ddTracer.WithError(err)) }()

	pdf := C.pdfDocument{
		handle: document.handle,
		error:  nil,
	}

	// Measure C function call performance
	cCallStart := time.Now()
	output := C.get_page_size(pdf, C.int(page))
	cCallDuration := time.Since(cCallStart)

	// Add performance metrics to trace
	span.SetTag("c_function", "get_page_size")
	span.SetTag("c_call_duration_ms", float64(cCallDuration.Nanoseconds())/1e6)

	if output.error != nil {
		defer C.je_free(unsafe.Pointer(output.error))
		span.SetTag("c_function_error", true)
		return PageSize{}, fmt.Errorf("failure at the C/MuPDF get_page_size function: %s", C.GoString(output.error))
	}
	pageSize = PageSize{
		Width:  float64(output.width),
		Height: float64(output.height),
	}

	return pageSize, nil
}

func (p *PdfHandler) wrapPageContents(ctx context.Context, document *PdfDocument, pageNum int) (err error) {
	span, _ := ddTracer.StartSpanFromContext(ctx, "PdfHandler.wrapPageContents")
	defer func() { span.Finish(ddTracer.WithError(err)) }()

	// Lock the entire function
	document.mu.Lock()
	defer document.mu.Unlock()

	if document.wrappedPages[pageNum] {
		return nil // Already wrapped, no need to call C function
	}

	pdf := C.pdfDocument{
		handle: document.handle,
		error:  nil,
	}

	output := C.wrap_page_contents_for_page(pdf, C.int(pageNum))
	if output.error != nil {
		defer C.je_free(unsafe.Pointer(output.error))
		return fmt.Errorf("failure at wrap_page_contents_for_page: %s", C.GoString(output.error))
	}

	document.wrappedPages[pageNum] = true

	return nil
}

func (p *PdfHandler) AddImageToPage(document PdfDocument, params ImageParams) (err error) {
	span, ctx := ddTracer.StartSpanFromContext(p.ctx, "PdfHandler.AddImageToPage")
	defer func() { span.Finish(ddTracer.WithError(err)) }()

	// Wrap page contents before adding image
	err = p.wrapPageContents(ctx, &document, params.Page)
	if err != nil {
		return fmt.Errorf("failure at wrapPageContents in AddImageToPage: %s", err)
	}

	cImagePath := C.CString(params.ImagePath)
	defer C.free(unsafe.Pointer(cImagePath))

	pdf := C.pdfDocument{
		handle: document.handle,
		error:  nil,
	}

	x, y, width, height, err := p.LocationSizeToPdfPoints(
		ctx,
		document,
		params.Page,
		params.Location.X,
		params.Location.Y,
		params.Size.Width,
		params.Size.Height,
	)
	if err != nil {
		return fmt.Errorf("failure at the AddImageToPage function: %s", err)
	}

	input := C.addImageInput{
		page:   C.int(params.Page),
		path:   cImagePath,
		x:      C.float(x),
		y:      C.float(y),
		width:  C.float(width),
		height: C.float(height),
	}

	// Measure C function call performance
	cCallStart := time.Now()
	output := C.add_image_to_page(pdf, input)
	cCallDuration := time.Since(cCallStart)

	// Add performance metrics to trace
	span.SetTag("c_function", "add_image_to_page")
	span.SetTag("c_call_duration_ms", float64(cCallDuration.Nanoseconds())/1e6)

	if output.error != nil {
		defer C.je_free(unsafe.Pointer(output.error))
		span.SetTag("c_function_error", true)
		return fmt.Errorf("failure at the C/MuPDF add_image_to_page function: %s", C.GoString(output.error))
	}

	return nil
}

var (
	fontMetricsCache = make(map[string]*sfnt.Font)
	fontCacheMu      sync.RWMutex
)

func GetDescenderToBaselineFromTTF(ctx context.Context, ttfPath string, fontSize float64) (float64, error) {
	span, _ := ddTracer.StartSpanFromContext(ctx, "GetDescenderToBaselineFromTTF")
	defer span.Finish()

	fontCacheMu.RLock()
	cachedFont, exists := fontMetricsCache[ttfPath]
	fontCacheMu.RUnlock()

	if !exists {
		fontData, err := os.ReadFile(ttfPath)
		if err != nil {
			return 0, fmt.Errorf("failed to read font: %w", err)
		}

		parsedFont, err := sfnt.Parse(fontData)
		if err != nil {
			return 0, fmt.Errorf("failed to parse font: %w", err)
		}

		fontCacheMu.Lock()
		fontMetricsCache[ttfPath] = parsedFont
		fontCacheMu.Unlock()
		cachedFont = parsedFont
	}

	var buf sfnt.Buffer
	metrics, err := cachedFont.Metrics(&buf, fixed.Int26_6(fontSize*64), font.HintingNone)
	if err != nil {
		return 0, fmt.Errorf("failed to get metrics: %w", err)
	}

	return math.Abs(float64(metrics.Descent) / 64.0), nil
}

func (p *PdfHandler) generateFontCandidates(ctx context.Context, font string) []string {
	span, _ := ddTracer.StartSpanFromContext(ctx, "PdfHandler.generateFontCandidates")
	defer span.Finish()

	exts := []string{".ttf", ".otf"}

	unique := make(map[string]struct{})
	transforms := []func(string) string{
		func(s string) string { return strings.ReplaceAll(s, " ", "_") },
		func(s string) string { return strings.ReplaceAll(s, " ", "-") },
		func(s string) string { return strings.ReplaceAll(s, " ", "") },
	}

	for _, transform := range transforms {
		for _, ext := range exts {
			unique[transform(font)+ext] = struct{}{}
		}
	}
	var candidates []string
	for key := range unique {
		candidates = append(candidates, key)
	}
	return candidates
}

func (p *PdfHandler) getFontAttributes(ctx context.Context, font string, fontSize float64) (fontPath string, descender float64, err error) {
	span, childCtx := ddTracer.StartSpanFromContext(ctx, "PdfHandler.getFontAttributes")
	defer func() { span.Finish(ddTracer.WithError(err)) }()

	for _, f := range standardFontList {
		if f.Name == font {
			return "", math.Abs(f.Descender / 1000.0 * fontSize), nil // Standard fonts do not need a file path
		}
	}

	candidates := p.generateFontCandidates(childCtx, font)
	dirs := []string{
		"/usr/share/fonts",      // System-wide fonts (Linux)
		"~/.fonts",              // User fonts (Linux)
		"/System/Library/Fonts", // System fonts (macOS)
		"/Library/Fonts",        // Local fonts (macOS)
		"~/Library/Fonts",       // User fonts (macOS)
		"fonts",                 // Local project fonts
	}

	for _, dir := range dirs {
		if dir[:2] == "~/" {
			dir = filepath.Join(os.Getenv("HOME"), dir[2:])
		}
		var path string
		err := filepath.WalkDir(dir, func(f string, d os.DirEntry, e error) error {
			if e != nil || d.IsDir() {
				return e
			}
			for _, candidate := range candidates {
				if filepath.Base(f) == candidate {
					path = f
					return filepath.SkipDir
				}
			}
			return nil
		})
		if err != nil && err != filepath.SkipDir {
			return "", 0, err
		}
		if path != "" {
			descender, err := GetDescenderToBaselineFromTTF(childCtx, path, fontSize)
			if err != nil {
				return "", 0, fmt.Errorf("failed to compute descender: %w", err)
			}
			return path, descender, nil
		}
	}
	return "", 0, fmt.Errorf("font %q not found", font)
}

func (p *PdfHandler) AddTextBoxToPage(document PdfDocument, params TextParams) (err error) {
	span, ctx := ddTracer.StartSpanFromContext(p.ctx, "PdfHandler.AddTextBoxToPage")
	defer func() { span.Finish(ddTracer.WithError(err)) }()

	// Wrap page contents before adding text
	err = p.wrapPageContents(ctx, &document, params.Page)
	if err != nil {
		return fmt.Errorf("failure at wrapPageContents in AddTextBoxToPage: %s", err)
	}

	fontPath, descender, err := p.getFontAttributes(ctx, params.Font.Family, params.Font.Size)
	if err != nil {
		return fmt.Errorf("failure at PdfHandler AddTextBoxToPage function: failed to find font path for %q", params.Font.Family)
	}

	pdf := C.pdfDocument{
		handle: document.handle,
		error:  nil,
	}

	x, y, _, _, err := p.LocationSizeToPdfPoints(
		ctx,
		document,
		params.Page,
		params.Location.X,
		params.Location.Y,
		params.Size.Width,
		params.Size.Height,
	)
	if err != nil {
		return fmt.Errorf("failure at the AddTextBoxToPage function: %s", err)
	}
	// In PDFs, text positioning is based on the baseline
	// However, the client provides the Y position as the top-left corner of the text box, along with its height.
	// To align the text correctly, we need to adjust the Y coordinate so that the descender sits at the bottom of the provided box.
	y = y + descender

	cText := C.CString(params.Value)
	defer C.free(unsafe.Pointer(cText))

	cFontFamily := C.CString(params.Font.Family)
	defer C.free(unsafe.Pointer(cFontFamily))

	cFontPath := C.CString(fontPath)
	defer C.free(unsafe.Pointer(cFontPath))

	input := C.addTextInput{
		text:        cText,
		page:        C.int(params.Page),
		x:           C.float(x),
		y:           C.float(y),
		font_family: cFontFamily,
		font_path:   cFontPath,
		font_size:   C.float(params.Font.Size),
	}

	// Measure C function call performance
	cCallStart := time.Now()
	output := C.add_text_to_page(pdf, input)
	cCallDuration := time.Since(cCallStart)

	// Add performance metrics to trace
	span.SetTag("c_function", "add_text_to_page")
	span.SetTag("c_call_duration_ms", float64(cCallDuration.Nanoseconds())/1e6)

	if output.error != nil {
		defer C.je_free(unsafe.Pointer(output.error))
		span.SetTag("c_function_error", true)
		return fmt.Errorf("failure at the C/MuPDF add_text_to_page function: %s", C.GoString(output.error))
	}

	return nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (p *PdfHandler) AddCheckboxToPage(document PdfDocument, params CheckboxParams) (err error) {
	span, ctx := ddTracer.StartSpanFromContext(p.ctx, "PdfHandler.AddCheckboxToPage")
	defer func() { span.Finish(ddTracer.WithError(err)) }()

	// Wrap page contents before adding checkbox
	err = p.wrapPageContents(ctx, &document, params.Page)
	if err != nil {
		return fmt.Errorf("failure at wrapPageContents in AddCheckboxToPage: %s", err)
	}

	pdf := C.pdfDocument{
		handle: document.handle,
		error:  nil,
	}

	x, y, width, height, err := p.LocationSizeToPdfPoints(
		ctx,
		document,
		params.Page,
		params.Location.X,
		params.Location.Y,
		params.Size.Width,
		params.Size.Height,
	)
	if err != nil {
		return fmt.Errorf("failure at the AddCheckboxToPage function: %s", err)
	}

	input := C.addCheckboxInput{
		value:  C.int(boolToInt(params.Value)),
		page:   C.int(params.Page),
		x:      C.float(x),
		y:      C.float(y),
		width:  C.float(width),
		height: C.float(height),
	}

	// Measure C function call performance
	cCallStart := time.Now()
	output := C.add_checkbox_to_page(pdf, input)
	cCallDuration := time.Since(cCallStart)

	// Add performance metrics to trace
	span.SetTag("c_function", "add_checkbox_to_page")
	span.SetTag("c_call_duration_ms", float64(cCallDuration.Nanoseconds())/1e6)

	if output.error != nil {
		defer C.je_free(unsafe.Pointer(output.error))
		span.SetTag("c_function_error", true)
		return fmt.Errorf("failure at the C/MuPDF add_checkbox_to_page function: %s", C.GoString(output.error))
	}

	return nil
}

func (p *PdfHandler) SavePDF(document PdfDocument, filePath string) (err error) {
	span, _ := ddTracer.StartSpanFromContext(p.ctx, "PdfHandler.SavePDF")
	defer func() { span.Finish(ddTracer.WithError(err)) }()

	cFilePath := C.CString(filePath)
	defer C.free(unsafe.Pointer(cFilePath))

	pdf := C.pdfDocument{
		handle: document.handle,
		error:  nil,
	}

	// Measure C function call performance
	cCallStart := time.Now()
	output := C.save_pdf(pdf, cFilePath)
	cCallDuration := time.Since(cCallStart)

	// Add performance metrics to trace
	span.SetTag("c_function", "save_pdf")
	span.SetTag("c_call_duration_ms", float64(cCallDuration.Nanoseconds())/1e6)

	if output.error != nil {
		defer C.je_free(unsafe.Pointer(output.error))
		span.SetTag("c_function_error", true)
		return fmt.Errorf("failure at the C/MuPDF save_pdf function: %s", C.GoString(output.error))
	}

	return nil
}

func (p *PdfHandler) SaveToPNG(document PdfDocument, page, width uint16, scale float32, dpi int, output io.Writer) (err error) {
	span, _ := ddTracer.StartSpanFromContext(p.ctx, "PdfHandler.SaveToPNG")
	defer func() { span.Finish(ddTracer.WithError(err)) }()

	if output == nil {
		return errors.New("output can't be nil")
	}

	pdf := C.pdfDocument{
		handle: document.handle,
		error:  nil,
	}
	input := C.saveToPNGInput{
		page:   C.int(page),
		width:  C.int(width),
		scale:  C.float(scale),
		dpi:    C.int(dpi),
		cookie: &C.fz_cookie{abort: 0},
	}

	if dpi < defaultDPI {
		input.dpi = C.int(defaultDPI)
	}

	// Set up context cancellation handling
	go func() {
		<-p.ctx.Done()
		input.cookie.abort = 1
	}()

	// Measure C function call performance
	cCallStart := time.Now()
	result := C.save_to_png_file(pdf, input)
	cCallDuration := time.Since(cCallStart)

	// Add performance metrics to trace
	span.SetTag("c_function", "save_to_png_file")
	span.SetTag("c_call_duration_ms", float64(cCallDuration.Nanoseconds())/1e6)

	defer C.je_free(unsafe.Pointer(result.payload))
	if result.error != nil {
		defer C.je_free(unsafe.Pointer(result.error))
		span.SetTag("c_function_error", true)
		return fmt.Errorf("failure at the C/MuPDF save_to_png_file function: %s", C.GoString(result.error))
	}

	if result.payload_length > 0 {
		if _, err := output.Write(C.GoBytes(unsafe.Pointer(result.payload), C.int(result.payload_length))); err != nil {
			return fmt.Errorf("fail to write to the output: %w", err)
		}
	}

	return nil
}
