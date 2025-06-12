package lazypdf

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"unsafe"
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

func (p PdfHandler) GetPageSize(document PdfDocument, Page int) (PageSize, error) {
	pdf := C.pdfDocument{
		handle: document.handle,
		error:  nil,
	}
	output := C.get_page_size(pdf, C.int(Page))
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
	candidates := make([]string, 0, len(unique))
	for key := range unique {
		candidates = append(candidates, key)
	}
	return candidates
}

func IsStandardFont(name string) bool {
	standardFonts := map[string]struct{}{
		"Courier":               {},
		"Courier-Oblique":       {},
		"Courier-Bold":          {},
		"Courier-BoldOblique":   {},
		"Helvetica":             {},
		"Helvetica-Oblique":     {},
		"Helvetica-Bold":        {},
		"Helvetica-BoldOblique": {},
		"Times-Roman":           {},
		"Times-Italic":          {},
		"Times-Bold":            {},
		"Times-BoldItalic":      {},
		"Symbol":                {},
		"ZapfDingbats":          {},
	}

	_, exists := standardFonts[name]
	return exists
}

func (p PdfHandler) GetFontPath(font string) (string, error) {
	if IsStandardFont(font) {
		return "", nil // Standard fonts do not need a file path
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
			return "", err
		}
		if path != "" {
			return path, nil
		}
	}
	return "", fmt.Errorf("font %q not found", font)
}

func (p PdfHandler) AddTextToPage(document PdfDocument, params TextParams) error {
	fontPath, err := p.GetFontPath(params.Font.Family)
	if err != nil {
		return fmt.Errorf("failure at PdfHandler AddTextToPage function: failed to find font path for %q", params.Font.Family)
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
		return fmt.Errorf("failure at the AddTextToPage function: %s", err)
	}

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
		defer C.free(unsafe.Pointer(output.error))
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
