// nolint
package lazypdf

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
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
}

type PdfDocument struct {
	handle C.uintptr_t
	file   string
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

func savePayloadToTempFile(r io.Reader) (filename string, err error) {
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
func (p PdfHandler) LocationSizeToPdfPoints(document PdfDocument, page int, x, y, width, height float64) (float64, float64, float64, float64, error) {
	pageSize, err := p.GetPageSize(document, page)
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

func (p PdfHandler) OpenPDF(rawPayload io.Reader) (PdfDocument, error) {
	filename, err := savePayloadToTempFile(rawPayload)
	if err != nil {
		return PdfDocument{}, err
	}

	cFilename := C.CString(filename)
	defer C.free(unsafe.Pointer(cFilename))

	input := C.openPDFInput{
		filename: cFilename,
	}
	output := C.open_pdf(input)
	if output.error != nil {
		defer C.je_free(unsafe.Pointer(output.error))
		return PdfDocument{}, fmt.Errorf("failure at the C/MuPDF open_pdf function: %s", C.GoString(output.error))
	}

	pdf := PdfDocument{
		handle: output.handle,
		file:   filename,
	}
	return pdf, nil
}

func (p PdfHandler) ClosePDF(document PdfDocument) error {
	pdf := C.pdfDocument{
		handle: document.handle,
		error:  nil,
	}
	output := C.close_pdf(pdf)
	removeErr := os.Remove(document.file)

	var errs []error
	if output.error != nil {
		defer C.je_free(unsafe.Pointer(output.error))
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

func (p PdfHandler) GetPageSize(document PdfDocument, page int) (PageSize, error) {
	pdf := C.pdfDocument{
		handle: document.handle,
		error:  nil,
	}
	output := C.get_page_size(pdf, C.int(page))
	if output.error != nil {
		defer C.je_free(unsafe.Pointer(output.error))
		return PageSize{}, fmt.Errorf("failure at the C/MuPDF get_page_size function: %s", C.GoString(output.error))
	}
	pageSize := PageSize{
		Width:  float64(output.width),
		Height: float64(output.height),
	}
	return pageSize, nil
}

func (p PdfHandler) AddImageToPage(document PdfDocument, params ImageParams) error {
	cImagePath := C.CString(params.ImagePath)
	defer C.free(unsafe.Pointer(cImagePath))

	pdf := C.pdfDocument{
		handle: document.handle,
		error:  nil,
	}

	x, y, width, height, err := p.LocationSizeToPdfPoints(
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

	output := C.add_image_to_page(pdf, input)
	if output.error != nil {
		defer C.je_free(unsafe.Pointer(output.error))
		return fmt.Errorf("failure at the C/MuPDF add_image_to_page function: %s", C.GoString(output.error))
	}

	return nil
}

var (
	fontMetricsCache = make(map[string]*sfnt.Font)
	fontCacheMu      sync.RWMutex
)

func GetDescenderToBaselineFromTTF(ttfPath string, fontSize float64) (float64, error) {
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

func (p PdfHandler) generateFontCandidates(font string) []string {
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

func (p PdfHandler) getFontAttributes(font string, fontSize float64) (fontPath string, descender float64, err error) {
	for _, f := range standardFontList {
		if f.Name == font {
			return "", math.Abs(f.Descender / 1000.0 * fontSize), nil // Standard fonts do not need a file path
		}
	}

	candidates := p.generateFontCandidates(font)
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
			descender, err := GetDescenderToBaselineFromTTF(path, fontSize)
			if err != nil {
				return "", 0, fmt.Errorf("failed to compute descender: %w", err)
			}
			return path, descender, nil
		}
	}
	return "", 0, fmt.Errorf("font %q not found", font)
}

func (p PdfHandler) AddTextBoxToPage(document PdfDocument, params TextParams) error {
	fontPath, descender, err := p.getFontAttributes(params.Font.Family, params.Font.Size)
	if err != nil {
		return fmt.Errorf("failure at PdfHandler AddTextBoxToPage function: failed to find font path for %q", params.Font.Family)
	}

	pdf := C.pdfDocument{
		handle: document.handle,
		error:  nil,
	}

	x, y, _, _, err := p.LocationSizeToPdfPoints(
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

	output := C.add_text_to_page(pdf, input)
	if output.error != nil {
		defer C.je_free(unsafe.Pointer(output.error))
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

func (p PdfHandler) AddCheckboxToPage(document PdfDocument, params CheckboxParams) error {
	pdf := C.pdfDocument{
		handle: document.handle,
		error:  nil,
	}

	x, y, width, height, err := p.LocationSizeToPdfPoints(
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

	output := C.add_checkbox_to_page(pdf, input)
	if output.error != nil {
		defer C.je_free(unsafe.Pointer(output.error))
		return fmt.Errorf("failure at the C/MuPDF add_checkbox_to_page function: %s", C.GoString(output.error))
	}

	return nil
}

func (p PdfHandler) SavePDF(document PdfDocument, filePath string) error {
	cFilePath := C.CString(filePath)
	defer C.free(unsafe.Pointer(cFilePath))

	pdf := C.pdfDocument{
		handle: document.handle,
		error:  nil,
	}

	output := C.save_pdf(pdf, cFilePath)
	if output.error != nil {
		defer C.je_free(unsafe.Pointer(output.error))
		return fmt.Errorf("failure at the C/MuPDF save_pdf function: %s", C.GoString(output.error))
	}

	return nil
}
