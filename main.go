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
	"sync"
	"time"

	ddTracer "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

// Global state to interact with C.
// nolint: gochecknoglobals
var (
	baseCtx           context.Context
	baseCtxCancel     func()
	cancelationReason string
	mutex             sync.Mutex
)

// SaveToPNG is used to convert a page from a PDF file to PNG.
func SaveToPNG(ctx context.Context, page, width uint16, scale float32, rawPayload io.Reader, output io.Writer) error {
	span, ctx := ddTracer.StartSpanFromContext(ctx, "lazypdf.SaveToPNG")
	defer span.Finish()

	if rawPayload == nil {
		return errors.New("payload can't be nil")
	}
	if output == nil {
		return errors.New("output can't be nil")
	}

	t1 := time.Now()
	mutex.Lock()
	span.SetTag("LockContention", time.Since(t1))
	defer mutex.Unlock()

	payload, err := io.ReadAll(rawPayload)
	if err != nil {
		return fmt.Errorf("fail to read the payload: %w", err)
	}
	payloadPointer := C.CBytes(payload)
	defer C.free(payloadPointer)

	baseCtx, baseCtxCancel = context.WithCancel(ctx)
	result := C.save_to_png(C.int(page), C.int(width), C.float(scale), (*C.uchar)(payloadPointer), C.size_t(len(payload)))
	if baseCtx.Err() != nil {
		return errors.New(cancelationReason)
	}
	defer C.drop_result(result)

	png := C.GoStringN(result.data, C.int(result.len))
	if _, err = output.Write([]byte(png)); err != nil {
		return fmt.Errorf("fail to write the result at the output: %w", err)
	}
	return nil
}

// PageCount is used to return the page count of the document.
func PageCount(ctx context.Context, rawPayload io.Reader) (int, error) {
	span, ctx := ddTracer.StartSpanFromContext(ctx, "lazypdf.PageCount")
	defer span.Finish()

	if rawPayload == nil {
		return 0, errors.New("payload can't be nil")
	}

	t1 := time.Now()
	mutex.Lock()
	span.SetTag("LockContention", time.Since(t1))
	defer mutex.Unlock()

	payload, err := io.ReadAll(rawPayload)
	if err != nil {
		return 0, fmt.Errorf("fail to read the payload: %w", err)
	}
	payloadPointer := C.CBytes(payload)
	defer C.free(payloadPointer)

	baseCtx, baseCtxCancel = context.WithCancel(ctx)
	result := C.page_count((*C.uchar)(payloadPointer), C.size_t(len(payload)))
	if baseCtx.Err() != nil {
		return 0, errors.New(cancelationReason)
	}

	return int(result), nil
}

//export errorHandler
func errorHandler(message *C.cchar) {
	baseCtxCancel()
	cancelationReason = C.GoString(message)
}
