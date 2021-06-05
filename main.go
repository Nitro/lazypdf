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
	"unsafe"

	"github.com/google/uuid"
	ddTracer "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

// Global state to interact with C.
// nolint: gochecknoglobals
var (
	saveToPNGState map[string]saveToPNGOutput
	saveToPNGMutex sync.Mutex
	pageCountState map[string]pageCountOutput
	pageCountMutex sync.Mutex
)

type saveToPNGOutput struct {
	chanResult chan []byte
	chanError  chan error
}

type pageCountOutput struct {
	chanResult chan int
	chanError  chan error
}

// nolint: gochecknoinits
func init() {
	saveToPNGState = make(map[string]saveToPNGOutput)
	pageCountState = make(map[string]pageCountOutput)
}

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

	payload, err := io.ReadAll(rawPayload)
	if err != nil {
		return fmt.Errorf("fail to read the payload: %w", err)
	}
	payloadPointer := C.CBytes(payload)
	defer C.free(payloadPointer)

	id := uuid.New().String()
	cgoID := C.CString(id)
	defer C.free(unsafe.Pointer(cgoID))

	input := C.SaveToPNGInput{
		id:             cgoID,
		page:           C.int(page),
		width:          C.int(width),
		scale:          C.float(scale),
		payload:        (*C.uchar)(payloadPointer),
		payload_length: C.size_t(len(payload)),
	}

	saveToPNGMutex.Lock()
	out := saveToPNGOutput{chanResult: make(chan []byte), chanError: make(chan error)}
	saveToPNGState[id] = out
	defer cleanSaveToPNGState(id)
	saveToPNGMutex.Unlock()
	C.save_to_png(&input) // nolint: gocritic

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-out.chanError:
		return err
	case payload := <-out.chanResult:
		if _, err = output.Write(payload); err != nil {
			return fmt.Errorf("fail to write the result at the output: %w", err)
		}
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

	payload, err := io.ReadAll(rawPayload)
	if err != nil {
		return 0, fmt.Errorf("fail to read the payload: %w", err)
	}
	payloadPointer := C.CBytes(payload)
	defer C.free(payloadPointer)

	id := uuid.New().String()
	cgoID := C.CString(id)
	defer C.free(unsafe.Pointer(cgoID))

	input := C.PageCountInput{
		id:             cgoID,
		payload:        (*C.uchar)(payloadPointer),
		payload_length: C.size_t(len(payload)),
	}

	pageCountMutex.Lock()
	out := pageCountOutput{chanResult: make(chan int), chanError: make(chan error)}
	pageCountState[id] = out
	defer cleanPageCountState(id)
	pageCountMutex.Unlock()
	C.page_count(&input) // nolint: gocritic

	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case err := <-out.chanError:
		return 0, err
	case count := <-out.chanResult:
		return count, nil
	}
}

//export callbackSaveToPNGOutput
func callbackSaveToPNGOutput(output *C.SaveToPNGOutput) {
	saveToPNGMutex.Lock()
	defer saveToPNGMutex.Unlock()

	id := C.GoString(output.id)
	if output.error != nil {
		saveToPNGState[id].chanError <- errors.New(C.GoString(output.error))
		return
	}
	saveToPNGState[id].chanResult <- []byte(C.GoStringN(output.data, C.int(output.len)))
}

func cleanSaveToPNGState(id string) {
	saveToPNGMutex.Lock()
	delete(saveToPNGState, id)
	saveToPNGMutex.Unlock()
}

//export callbackPageCountOutput
func callbackPageCountOutput(output *C.PageCountOutput) {
	pageCountMutex.Lock()
	defer pageCountMutex.Unlock()

	id := C.GoString(output.id)
	if output.error != nil {
		pageCountState[id].chanError <- errors.New(C.GoString(output.error))
		return
	}
	pageCountState[id].chanResult <- int(output.count)
}

func cleanPageCountState(id string) {
	pageCountMutex.Lock()
	delete(pageCountState, id)
	pageCountMutex.Unlock()
}
