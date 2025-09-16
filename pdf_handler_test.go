// nolint
package lazypdf

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPdfHandler_OpenPDF(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := PdfHandler{Logger: logger}

	file, err := os.Open("testdata/pdf_handler_sample.pdf")
	require.NoError(t, err)
	defer func() { require.NoError(t, file.Close()) }()

	document, err := handler.OpenPDF(file)
	if err != nil {
		t.Fatalf("OpenPDF: %v", err)
	}
	if document.handle == 0 {
		t.Fatalf("handle is null:")
	}
	defer func() { require.NoError(t, handler.ClosePDF(document)) }()
}

func TestPdfHandler_OpenInvalidFile(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := PdfHandler{Logger: logger}

	file, err := os.Open("testdata/sample-invalid.pdf")
	require.NoError(t, err)
	defer func() { require.NoError(t, file.Close()) }()

	_, err = handler.OpenPDF(file)
	require.Error(t, err)
	require.Equal(t, "failure at the C/MuPDF open_pdf function: no objects found", err.Error())
}

func TestPdfHandler_OpenNil(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := PdfHandler{Logger: logger}

	_, err := handler.OpenPDF(nil)
	require.Error(t, err)
	require.Equal(t, "payload can't be nil", err.Error())
}

func TestPdfHandler_TestClosePDF(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := PdfHandler{Logger: logger}

	file, err := os.Open("testdata/pdf_handler_sample.pdf")
	require.NoError(t, err)
	defer func() { require.NoError(t, file.Close()) }()

	document, err := handler.OpenPDF(file)
	if err != nil {
		t.Fatalf("OpenPDF: %v", err)
	}

	if err := handler.ClosePDF(document); err != nil {
		t.Fatalf("ClosePDF: %v", err)
	}
}
func TestPdfHandler_ConvertTopLeftToBottomLeft_InvalidPage(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := PdfHandler{Logger: logger}

	file, err := os.Open("testdata/pdf_handler_sample.pdf")
	require.NoError(t, err)
	defer func() { require.NoError(t, file.Close()) }()

	document, err := handler.OpenPDF(file)
	if err != nil {
		t.Fatalf("OpenPDF: %v", err)
	}
	defer func() { require.NoError(t, handler.ClosePDF(document)) }()

	_, _, err = handler.ConvertTopLeftToBottomLeft(
		context.Background(),
		document,
		2,
		0,
		0,
		0,
	)

	require.Error(t, err)
	require.Equal(t, "failed to get page size: failure at the C/MuPDF get_page_size function: invalid page number: 3", err.Error())
}

func TestPdfHandler_ConvertTopLeftToBottomLeft(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		path      string
		x         float64
		y         float64
		height    float64
		expectedX float64
		expectedY float64
	}{
		{
			"top-left corner with zero height",
			"testdata/pdf_handler_sample.pdf",
			0, 0, 0,
			0, 792.0, // For 612x792 page, top-left (0,0) with height 0 becomes (0, 792)
		},
		{
			"top-left corner with some height",
			"testdata/pdf_handler_sample.pdf",
			0, 0, 50,
			0, 742.0, // For 612x792 page, top-left (0,0) with height 50 becomes (0, 792-0-50=742)
		},
		{
			"center of page",
			"testdata/pdf_handler_sample.pdf",
			306, 396, 100,
			306, 296, // For 612x792 page, center (306,396) with height 100 becomes (306, 792-396-100=296)
		},
		{
			"bottom-right corner",
			"testdata/pdf_handler_sample.pdf",
			612, 792, 0,
			612, 0, // For 612x792 page, bottom-right (612,792) with height 0 becomes (612, 792-792-0=0)
		},
		{
			"rotated page top-left",
			"testdata/sample_rotate_90.pdf",
			0, 0, 0,
			0, 612.0, // For 792x612 rotated page, top-left (0,0) with height 0 becomes (0, 612)
		},
		{
			"rotated page center",
			"testdata/sample_rotate_90.pdf",
			396, 306, 50,
			396, 256, // For 792x612 rotated page, center (396,306) with height 50 becomes (396, 612-306-50=256)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := setupPdfHandler(t)
			handle := openTestPDF(t, tt.path)

			x, y, err := handler.ConvertTopLeftToBottomLeft(
				context.Background(),
				handle,
				0,
				tt.x,
				tt.y,
				tt.height,
			)
			require.NoError(t, err, "Failed to convert top-left to bottom-left coordinates for file: %s", tt.path)

			require.InDelta(t, tt.expectedX, x, 0.1, "Unexpected x coordinate for file: %s", tt.path)
			require.InDelta(t, tt.expectedY, y, 0.1, "Unexpected y coordinate for file: %s", tt.path)
		})
	}
}
func TestPdfHandler_GetPageSize_InvalidPage(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := PdfHandler{Logger: logger}

	file, err := os.Open("testdata/pdf_handler_sample.pdf")
	require.NoError(t, err)
	defer func() { require.NoError(t, file.Close()) }()

	document, err := handler.OpenPDF(file)
	if err != nil {
		t.Fatalf("OpenPDF: %v", err)
	}
	defer func() { require.NoError(t, handler.ClosePDF(document)) }()

	_, err = handler.GetPageSize(document, 2)
	require.Error(t, err)
	require.Equal(t, "failure at the C/MuPDF get_page_size function: invalid page number: 3", err.Error())
}

func TestPdfHandler_TestGetPageSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path           string
		expectedWidth  float64
		expectedHeight float64
	}{
		{"testdata/pdf_handler_sample.pdf", 612.0, 792.0},
		{"testdata/sample_rotate_90.pdf", 792.0, 612.0},
		{"testdata/sample_rotate_180.pdf", 612.0, 792.0},
		{"testdata/sample_rotate_270.pdf", 792.0, 612.0},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()

			handler := setupPdfHandler(t)
			document := openTestPDF(t, tt.path)

			size, err := handler.GetPageSize(document, 0)
			require.NoError(t, err, "Failed to get page size for file: %s", tt.path)

			require.InDelta(t, tt.expectedWidth, size.Width, 0.1, "Unexpected width for file: %s", tt.path)
			require.InDelta(t, tt.expectedHeight, size.Height, 0.1, "Unexpected height for file: %s", tt.path)
		})
	}

}

func TestPdfHandler_AddImageToPage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		pdfPath     string
		outputPath  string
		imageParams ImageParams
	}{
		{
			name:       "Valid Image - A4 - Portrait",
			pdfPath:    "testdata/pdf_handler_sample.pdf",
			outputPath: "tmp/output_rotate_0_add_image_to_page_valid_image.pdf",
			imageParams: ImageParams{
				Page: 0,
				Location: Location{
					X: 0,
					Y: 677,
				},
				Size: Size{
					Width:  443,
					Height: 115,
				},
				ImagePath: "testdata/test_signature.png",
			},
		},
		{
			name:       "Valid Image - A4 - Landscape",
			pdfPath:    "testdata/sample_rotate_90.pdf",
			outputPath: "tmp/output_rotate_90_add_image_to_page_valid_image.pdf",
			imageParams: ImageParams{
				Page: 0,
				Location: Location{
					X: 0,
					Y: 523,
				},
				Size: Size{
					Width:  573,
					Height: 88,
				},
				ImagePath: "testdata/test_signature.png",
			},
		},
		{
			name:       "Valid Image - A4 - Portrait - 1/4 size - top right",
			pdfPath:    "testdata/sample_rotate_180.pdf",
			outputPath: "tmp/output_rotate_180_add_image_to_page_valid_image_top_right.pdf",
			imageParams: ImageParams{
				Page: 0,
				Location: Location{
					X: 501,
					Y: 0,
				},
				Size: Size{
					Width:  110,
					Height: 28,
				},
				ImagePath: "testdata/test_signature.png",
			},
		},
		{
			name:       "Valid Image - A4 - Landscape",
			pdfPath:    "testdata/sample_rotate_270.pdf",
			outputPath: "tmp/output_rotate_270_add_image_to_page_valid_image.pdf",
			imageParams: ImageParams{
				Page: 0,
				Location: Location{
					X: 349,
					Y: 497,
				},
				Size: Size{
					Width:  443,
					Height: 114,
				},
				ImagePath: "testdata/test_signature.png",
			},
		},
		{
			name:       "Valid Image - unreliable content stream",
			pdfPath:    "testdata/sample_content.pdf",
			outputPath: "tmp/output_add_image_to_unreliable_content_stream.pdf",
			imageParams: ImageParams{
				Page: 0,
				Location: Location{
					X: 0,
					Y: 0,
				},
				Size: Size{
					Width:  443,
					Height: 115,
				},
				ImagePath: "testdata/test_signature.png",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := setupPdfHandler(t)
			document := openTestPDF(t, tt.pdfPath)

			addImageAndSave(t, handler, document, tt.imageParams, tt.outputPath)
		})
	}
}

func TestPdfHandler_AddImageToPage_InvalidPage(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := PdfHandler{Logger: logger}

	file, err := os.Open("testdata/pdf_handler_sample.pdf")
	require.NoError(t, err)
	defer func() { require.NoError(t, file.Close()) }()

	document, err := handler.OpenPDF(file)
	if err != nil {
		t.Fatalf("OpenPDF: %v", err)
	}
	defer func() { require.NoError(t, handler.ClosePDF(document)) }()

	params := ImageParams{
		Page: 13,
		Location: Location{
			X: 0,
			Y: 0,
		},
		Size: Size{
			Width:  61,
			Height: 79,
		},
		ImagePath: "testdata/test_signature.png",
	}

	err = handler.AddImageToPage(document, params)
	require.Error(t, err)
	require.Equal(t, "failure at the AddImageToPage function: failed to get page size: failure at the C/MuPDF get_page_size function: invalid page number: 14", err.Error())
}

func TestPdfHandler_AddImageToPage_InvalidImage(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := PdfHandler{Logger: logger}

	file, err := os.Open("testdata/pdf_handler_sample.pdf")
	require.NoError(t, err)
	defer func() { require.NoError(t, file.Close()) }()

	document, err := handler.OpenPDF(file)
	if err != nil {
		t.Fatalf("OpenPDF: %v", err)
	}
	defer func() { require.NoError(t, handler.ClosePDF(document)) }()

	params := ImageParams{
		Page: 0,
		Location: Location{
			X: 306,
			Y: 79,
		},
		Size: Size{
			Width:  91,
			Height: 158,
		},
		ImagePath: "testdata/test_signature-invalid.png",
	}

	err = handler.AddImageToPage(document, params)
	require.Error(t, err)
	require.Equal(t, "failure at the C/MuPDF add_image_to_page function: unknown image file format", err.Error())

}

func TestPdfHandler_AddCheckboxToPage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		inputFile  string
		outputFile string
		params     CheckboxParams
	}{
		{
			name:       "Checkbox - A4 - Portrait - Bottom Left",
			inputFile:  "testdata/pdf_handler_sample.pdf",
			outputFile: "tmp/output_rotate_0_add_checkbox_bottom_left.pdf",
			params: CheckboxParams{
				Value: true,
				Page:  0,
				Location: Location{
					X: 0,
					Y: 772,
				},
				Size: Size{
					Width:  20,
					Height: 20,
				},
			},
		},
		{
			name:       "Checkbox - A4 - Landscape - Bottom Left",
			inputFile:  "testdata/sample_rotate_90.pdf",
			outputFile: "tmp/output_rotate_90_add_checkbox_bottom_left.pdf",
			params: CheckboxParams{
				Value: false,
				Page:  0,
				Location: Location{
					X: 0,
					Y: 582,
				},
				Size: Size{
					Width:  30,
					Height: 30,
				},
			},
		},
		{
			name:       "Checkbox - A4 - Portrait - Bottom Right",
			inputFile:  "testdata/sample_rotate_180.pdf",
			outputFile: "tmp/output_rotate_180_add_checkbox_bottom_right.pdf",
			params: CheckboxParams{
				Value: true,
				Page:  0,
				Location: Location{
					X: 572,
					Y: 751,
				},
				Size: Size{
					Width:  40,
					Height: 40,
				},
			},
		},
		{
			name:       "Checkbox - A4 - Landscape - Top Right",
			inputFile:  "testdata/sample_rotate_270.pdf",
			outputFile: "tmp/output_rotate_270_add_checkbox_top_right.pdf",
			params: CheckboxParams{
				Value: true,
				Page:  0,
				Location: Location{
					X: 742,
					Y: 0,
				},
				Size: Size{
					Width:  50,
					Height: 50,
				},
			},
		},
		{
			name:       "Checkbox - unreliable content stream",
			inputFile:  "testdata/sample_content.pdf",
			outputFile: "tmp/output_add_checkbox_to_unreliable_content_stream.pdf",
			params: CheckboxParams{
				Value: true,
				Page:  0,
				Location: Location{
					X: 573,
					Y: 0,
				},
				Size: Size{
					Width:  40,
					Height: 40,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
			handler := PdfHandler{Logger: logger}

			file, err := os.Open(tt.inputFile)
			require.NoError(t, err)
			defer func() { require.NoError(t, file.Close()) }()

			document, err := handler.OpenPDF(file)
			require.NoError(t, err, "OpenPDF failed")
			defer func() { require.NoError(t, handler.ClosePDF(document)) }()

			err = handler.AddCheckboxToPage(document, tt.params)
			require.NoError(t, err, "failed to add checkbox")

			err = handler.SavePDF(document, tt.outputFile)
			require.NoError(t, err, "failed to save PDF")
		})
	}
}

func TestPdfHandler_AddCheckboxToPage_InvalidPage(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := PdfHandler{Logger: logger}

	file, err := os.Open("testdata/pdf_handler_sample.pdf")
	require.NoError(t, err)
	defer func() { require.NoError(t, file.Close()) }()

	document, err := handler.OpenPDF(file)
	if err != nil {
		t.Fatalf("OpenPDF: %v", err)
	}
	defer func() { require.NoError(t, handler.ClosePDF(document)) }()

	params := CheckboxParams{
		Value: true,
		Page:  3,
		Location: Location{
			X: 50,
			Y: 100,
		},
		Size: Size{
			Width:  20,
			Height: 20,
		},
	}

	err = handler.AddCheckboxToPage(document, params)
	require.Error(t, err)
	require.Equal(t, "failure at the AddCheckboxToPage function: failed to get page size: failure at the C/MuPDF get_page_size function: invalid page number: 4", err.Error())
}

func TestPdfHandler_SavePDF_Valid(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := PdfHandler{Logger: logger}

	file, err := os.Open("testdata/pdf_handler_sample.pdf")
	require.NoError(t, err)
	defer func() { require.NoError(t, file.Close()) }()

	document, err := handler.OpenPDF(file)
	if err != nil {
		t.Fatalf("OpenPDF: %v", err)
	}
	defer func() { require.NoError(t, handler.ClosePDF(document)) }()

	outputPath := "tmp/output.pdf"

	err = handler.SavePDF(document, outputPath)
	if err != nil {
		t.Fatalf("failed to save PDF: %v", err)
	}
}

func setupPdfHandler(t *testing.T) PdfHandler {
	t.Helper()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	return PdfHandler{Logger: logger}
}

func openTestPDF(t *testing.T, filePath string) *PdfDocument {
	t.Helper()

	handler := setupPdfHandler(t)
	file, err := os.Open(filePath)
	require.NoError(t, err)

	document, err := handler.OpenPDF(file)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, file.Close())
		require.NoError(t, handler.ClosePDF(document))
	})

	return document
}

func addImageAndSave(t *testing.T, handler PdfHandler, document *PdfDocument, params ImageParams, outputPath string) {
	t.Helper()

	err := handler.AddImageToPage(document, params)
	require.NoError(t, err, "failed to add image")

	err = handler.SavePDF(document, outputPath)
	require.NoError(t, err, "failed to save PDF")
}

func BenchmarkPdfHandler_SoftSave(b *testing.B) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := PdfHandler{Logger: logger}

	file, err := os.Open("testdata/textboxes.pdf")
	require.NoError(b, err)
	defer func() { _ = file.Close() }()

	document, err := handler.OpenPDF(file)
	require.NoError(b, err)
	defer func() { _ = handler.ClosePDF(document) }()

	timestamp := time.Now().Format("20060102_150405")
	tmpDir := filepath.Join("tmp", timestamp)
	require.NoError(b, os.MkdirAll(tmpDir, 0o755))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		output := filepath.Join(tmpDir, fmt.Sprintf("save_%d.pdf", i))
		err := handler.SavePDF(document, output)
		require.NoError(b, err)
	}
	b.StopTimer()
}
