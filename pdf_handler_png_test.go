package lazypdf

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

func TestPdfHandler_SaveToPNGOK(t *testing.T) {
	t.Parallel()

	for i := uint16(0); i < 13; i++ {
		t.Run(fmt.Sprintf("page_%d", i), func(t *testing.T) {

			logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
			handler := NewPdfHandler(context.Background(), logger)
			file, err := os.Open("testdata/sample.pdf")
			require.NoError(t, err)
			defer func() { require.NoError(t, file.Close()) }()

			document, err := handler.OpenPDF(file)
			require.NoError(t, err)
			defer func() { require.NoError(t, handler.ClosePDF(document)) }()

			buf := bytes.NewBuffer([]byte{})
			err = handler.SaveToPNG(document, i, 0, 0, 0, buf)
			require.NoError(t, err)

			resultPage, err := io.ReadAll(buf)
			require.NoError(t, err)
			expectedPage, err := os.ReadFile(fmt.Sprintf("testdata/sample_page%d.png", i))
			require.NoError(t, err)
			require.Equal(t, expectedPage, resultPage)
		})
	}
}

func BenchmarkPdfHandler_SaveToPNGPage0(b *testing.B)  { benchmarkPdfHandlerSaveToPNGRunner(0, b) }
func BenchmarkPdfHandler_SaveToPNGPage1(b *testing.B)  { benchmarkPdfHandlerSaveToPNGRunner(1, b) }
func BenchmarkPdfHandler_SaveToPNGPage2(b *testing.B)  { benchmarkPdfHandlerSaveToPNGRunner(2, b) }
func BenchmarkPdfHandler_SaveToPNGPage3(b *testing.B)  { benchmarkPdfHandlerSaveToPNGRunner(3, b) }
func BenchmarkPdfHandler_SaveToPNGPage4(b *testing.B)  { benchmarkPdfHandlerSaveToPNGRunner(4, b) }
func BenchmarkPdfHandler_SaveToPNGPage5(b *testing.B)  { benchmarkPdfHandlerSaveToPNGRunner(5, b) }
func BenchmarkPdfHandler_SaveToPNGPage6(b *testing.B)  { benchmarkPdfHandlerSaveToPNGRunner(6, b) }
func BenchmarkPdfHandler_SaveToPNGPage7(b *testing.B)  { benchmarkPdfHandlerSaveToPNGRunner(7, b) }
func BenchmarkPdfHandler_SaveToPNGPage8(b *testing.B)  { benchmarkPdfHandlerSaveToPNGRunner(8, b) }
func BenchmarkPdfHandler_SaveToPNGPage9(b *testing.B)  { benchmarkPdfHandlerSaveToPNGRunner(9, b) }
func BenchmarkPdfHandler_SaveToPNGPage10(b *testing.B) { benchmarkPdfHandlerSaveToPNGRunner(10, b) }
func BenchmarkPdfHandler_SaveToPNGPage11(b *testing.B) { benchmarkPdfHandlerSaveToPNGRunner(11, b) }
func BenchmarkPdfHandler_SaveToPNGPage12(b *testing.B) { benchmarkPdfHandlerSaveToPNGRunner(12, b) }

func benchmarkPdfHandlerSaveToPNGRunner(page uint16, b *testing.B) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := NewPdfHandler(context.Background(), logger)

	buf, err := os.ReadFile("testdata/sample.pdf")
	require.NoError(b, err)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		input := bytes.NewBuffer(buf)
		document, err := handler.OpenPDF(input)
		require.NoError(b, err)

		output := bytes.NewBuffer([]byte{})
		err = handler.SaveToPNG(document, page, 0, 0, 0, output)
		require.NoError(b, err)

		err = handler.ClosePDF(document)
		require.NoError(b, err)
	}
}
