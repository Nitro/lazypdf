package lazypdf

/*
#cgo CFLAGS: -I ${SRCDIR}/misc/mupdf/include -I ${SRCDIR}/misc/mupdf/include/mupdf
#cgo darwin,amd64 LDFLAGS: -L ${SRCDIR}/misc/mupdf/lib/x86-64-macos -lmupdf -lmupdf-third
#cgo linux,amd64 LDFLAGS: -L ${SRCDIR}/misc/mupdf/lib/x86-64-linux -lmupdf -lmupdf-third -lm
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

// SaveToPNG is used to convert a page from a PDF file to PNG.
func SaveToPNG(ctx context.Context, page, width uint16, scale float32, rawPayload io.Reader, output io.Writer) error {
	span, _ := ddTracer.StartSpanFromContext(ctx, "lazypdf.SaveToPNG")
	defer span.Finish()

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
	payloadPointer := C.CBytes(payload)
	defer C.free(payloadPointer)

	input := C.save_to_png_input{
		page:           C.int(page),
		width:          C.int(width),
		scale:          C.float(scale),
		payload:        (*C.uchar)(payloadPointer),
		payload_length: C.size_t(len(payload)),
	}
	result := C.save_to_png(&input) // nolint: gocritic
	defer func() {
		C.fz_drop_buffer(result.ctx, result.buffer)
		C.fz_drop_context(result.ctx)
		C.free(unsafe.Pointer(result))
	}()
	if result.error != nil {
		return errors.New(C.GoString(result.error))
	}

	if _, err := output.Write([]byte(C.GoStringN(result.data, C.int(result.len)))); err != nil {
		return fmt.Errorf("fail to write to the output: %w", err)
	}
	return nil
}

// PageCount is used to return the page count of the document.
func PageCount(ctx context.Context, rawPayload io.Reader) (int, error) {
	span, _ := ddTracer.StartSpanFromContext(ctx, "lazypdf.PageCount")
	defer span.Finish()

	if rawPayload == nil {
		return 0, errors.New("payload can't be nil")
	}

	payload, err := io.ReadAll(rawPayload)
	if err != nil {
		return 0, fmt.Errorf("fail to read the payload: %w", err)
	}
	payloadPointer := C.CBytes(payload)
	defer C.free(payloadPointer)

	input := C.page_count_input{
		payload:        (*C.uchar)(payloadPointer),
		payload_length: C.size_t(len(payload)),
	}
	output := C.page_count(&input) // nolint: gocritic
	defer C.free(unsafe.Pointer(output))
	if output.error != nil {
		return 0, errors.New(C.GoString(output.error))
	}
	return int(output.count), nil
}
