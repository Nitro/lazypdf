package lazypdf

/*
#cgo CFLAGS: -I ${SRCDIR}/misc/mupdf/include -I ${SRCDIR}/misc/mupdf/include/mupdf -I ${SRCDIR}/misc/jemalloc/include -I ${SRCDIR}/misc/jemalloc/include/jemalloc
#cgo darwin,amd64 LDFLAGS: -L ${SRCDIR}/misc/mupdf/lib/x86-64-macos -lmupdf -lmupdf-third -L ${SRCDIR}/misc/jemalloc/lib/x86-64-macos -ljemalloc
#cgo linux,amd64 LDFLAGS: -L ${SRCDIR}/misc/mupdf/lib/x86-64-linux -lmupdf -lmupdf-third -L ${SRCDIR}/misc/jemalloc/lib/x86-64-linux -ljemalloc -lm -lpthread -ldl
#include "main.h"
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"io"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/ext"
	ddTracer "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

func init() {
	C.init()
}

// SaveToPNG is used to convert a page from a PDF file to PNG.
func SaveToPNG(ctx context.Context, page, width uint16, scale float32, rawPayload io.Reader, output io.Writer) (err error) {
	span, _ := ddTracer.StartSpanFromContext(ctx, "lazypdf.SaveToPNG")
	defer func() {
		if err != nil {
			span.SetTag(ext.Error, err.Error())
		}
		span.Finish()
	}()

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
	defer C.drop_save_to_png_output(result)
	if result.error != nil {
		return errors.New(C.GoString(result.error))
	}

	if _, err := output.Write([]byte(C.GoStringN(result.data, C.int(result.len)))); err != nil {
		return fmt.Errorf("fail to write to the output: %w", err)
	}
	return nil
}

// PageCount is used to return the page count of the document.
func PageCount(ctx context.Context, rawPayload io.Reader) (_ int, err error) {
	span, _ := ddTracer.StartSpanFromContext(ctx, "lazypdf.PageCount")
	defer func() {
		if err != nil {
			span.SetTag(ext.Error, err.Error())
		}
		span.Finish()
	}()

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
	defer C.drop_page_count_output(output)
	if output.error != nil {
		return 0, errors.New(C.GoString(output.error))
	}
	return int(output.count), nil
}
