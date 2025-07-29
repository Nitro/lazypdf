package lazypdf

/*
#cgo CFLAGS: -I ${SRCDIR}/misc/mupdf/include -I ${SRCDIR}/misc/mupdf/include/mupdf -I ${SRCDIR}/misc/jemalloc/include -I ${SRCDIR}/misc/jemalloc/include/jemalloc
#cgo linux,amd64 LDFLAGS: -L ${SRCDIR}/misc/mupdf/lib/x86-64-linux -lmupdf -lmupdf-third -L ${SRCDIR}/misc/jemalloc/lib/x86-64-linux -ljemalloc -lm -lpthread -ldl
#cgo darwin,arm64 LDFLAGS: -L ${SRCDIR}/misc/mupdf/lib/arm64-macos -lmupdf -lmupdf-third -L ${SRCDIR}/misc/jemalloc/lib/arm64-macos -ljemalloc -lm -lpthread -ldl
#include <jemalloc/jemalloc.h>
#include "main.h"
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"unsafe"

	ddTracer "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

const defaultDPI = 72

func init() {
	C.init()
}

// SaveToPNG is used to convert a page from a PDF file to PNG. Internally everything is based on the scale factor and
// this value is used to determine the actual output size based on the original size of the page.
// If none is set we'll use a default scale factor of 1.5. When using the default value, 1.5, there is a special case
// when we detect that the page is a landscape and it has a 0 or 180 degree rotation, on those cases we set the scale
// factor of 1.
// If width is set then we'll calculate the scale factor by dividing the width by the page horizontal size.
// If both width and scale are set we'll use only the scale as it takes precedence.
func SaveToPNG(
	ctx context.Context, page, width uint16, scale float32, dpi int, rawPayload io.Reader, output io.Writer,
) (err error) {
	span, _ := ddTracer.StartSpanFromContext(ctx, "lazypdf.SaveToPNG")
	defer func() { span.Finish(ddTracer.WithError(err)) }()

	if rawPayload == nil {
		return errors.New("payload can't be nil")
	}
	if output == nil {
		return errors.New("output can't be nil")
	}

	payload, err := io.ReadAll(rawPayload)
	if err != nil {
		return fmt.Errorf("fail to read the payload: %w", err)
	}

	input := C.save_to_png_input{
		params: C.save_to_png_params{
			page:   C.int(page),
			width:  C.int(width),
			scale:  C.float(scale),
			dpi:    C.int(dpi),
			cookie: &C.fz_cookie{abort: 0},
		},
		payload:        (*C.char)(unsafe.Pointer(&payload[0])),
		payload_length: C.size_t(len(payload)),
	}
	if dpi < defaultDPI {
		input.params.dpi = C.int(defaultDPI)
	}
	go func() {
		<-ctx.Done()
		input.params.cookie.abort = 1
	}()
	result := C.save_to_png(input) // nolint: gocritic
	defer C.je_free(unsafe.Pointer(result.payload))
	if result.error != nil {
		defer C.je_free(unsafe.Pointer(result.error))
		return fmt.Errorf("failure at the C/MuPDF layer: %s", C.GoString(result.error))
	}

	if _, err := output.Write([]byte(C.GoStringN(result.payload, C.int(result.payload_length)))); err != nil {
		return fmt.Errorf("fail to write to the output: %w", err)
	}
	return nil
}

func SaveToHTML(
	ctx context.Context, page, width uint16, scale float32, dpi int, rawPayload io.Reader, output io.Writer,
) (err error) {
	span, _ := ddTracer.StartSpanFromContext(ctx, "lazypdf.SaveToHTML")
	defer func() { span.Finish(ddTracer.WithError(err)) }()

	if rawPayload == nil {
		return errors.New("payload can't be nil")
	}
	if output == nil {
		return errors.New("output can't be nil")
	}

	payload, err := io.ReadAll(rawPayload)
	if err != nil {
		return fmt.Errorf("fail to read the payload: %w", err)
	}

	input := C.save_to_html_input{
		params: C.save_to_html_params{
			page:   C.int(page),
			width:  C.int(width),
			scale:  C.float(scale),
			dpi:    C.int(dpi),
			cookie: &C.fz_cookie{abort: 0},
		},
		payload:        (*C.char)(unsafe.Pointer(&payload[0])),
		payload_length: C.size_t(len(payload)),
	}
	if dpi < defaultDPI {
		input.params.dpi = C.int(defaultDPI)
	}
	go func() {
		<-ctx.Done()
		input.params.cookie.abort = 1
	}()
	result := C.save_to_html(input) // nolint: gocritic
	defer C.je_free(unsafe.Pointer(result.payload))
	if result.error != nil {
		defer C.je_free(unsafe.Pointer(result.error))
		return fmt.Errorf("failure at the C/MuPDF layer: %s", C.GoString(result.error))
	}

	if _, err := output.Write(C.GoBytes(unsafe.Pointer(result.payload), C.int(result.payload_length))); err != nil {
		return fmt.Errorf("fail to write to the output: %w", err)
	}
	return nil
}

// PageCount is used to return the page count of the document.
func PageCount(ctx context.Context, rawPayload io.Reader) (_ int, err error) {
	span, _ := ddTracer.StartSpanFromContext(ctx, "lazypdf.PageCount")
	defer func() { span.Finish(ddTracer.WithError(err)) }()

	if rawPayload == nil {
		return 0, errors.New("payload can't be nil")
	}

	payload, err := io.ReadAll(rawPayload)
	if err != nil {
		return 0, fmt.Errorf("fail to read the payload: %w", err)
	}
	input := C.page_count_input{
		payload:        (*C.char)(unsafe.Pointer(&payload[0])),
		payload_length: C.size_t(len(payload)),
	}
	output := C.page_count(input) // nolint: gocritic
	if output.error != nil {
		defer C.je_free(unsafe.Pointer(output.error))
		return 0, fmt.Errorf("failure at the C/MuPDF layer: %s", C.GoString(output.error))
	}
	return int(output.count), nil
}
