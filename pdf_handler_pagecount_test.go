// nolint
package lazypdf

/*
import (
    "bytes"
    "context"

    "fmt"
    "io"
    "log/slog"
    "os"
    "testing"

    "github.com/stretchr/testify/require"
)

func TestPageCount(t *testing.T) {
    t.Parallel()

    file, err := os.Open("testdata/pdf_handler_sample.pdf")
    require.NoError(t, err)
    defer func() { require.NoError(t, file.Close()) }()

    handler := setupPdfHandler(t)

    document, err := handler.OpenPDF(file)
    if err != nil {
        t.Fatalf("OpenPDF: %v", err)
    }
    defer func() { require.NoError(t, handler.ClosePDF(document)) }()

    count, err := handler.PageCount(document)
    require.NoError(t, err)
    require.Equal(t, 1, count)
}
*/
