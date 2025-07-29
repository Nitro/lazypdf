// nolint
package lazypdf

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"strings"
	"testing"

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
func TestPdfHandler_LocationSizeToPdfPoints_InvalidPage(t *testing.T) {
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

	//nolint:dogsled // we only care about the error
	_, _, _, _, err = handler.LocationSizeToPdfPoints(
		context.Background(),
		document,
		2,
		0,
		0,
		0,
		0,
	)

	require.Error(t, err)
	require.Equal(t, "failed to get page size: failure at the C/MuPDF get_page_size function: invalid page number: 3", err.Error())
}

func TestPdfHandler_LocationSizeToPdfPoints_InvalidInputPercentages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		path                string
		x, y, width, height float64
		expectedError       string
	}{
		{
			name:          "X greater than 1",
			path:          "testdata/pdf_handler_sample.pdf",
			x:             1.1,
			y:             0.5,
			width:         0.5,
			height:        0.5,
			expectedError: "invalid input percentages: x=1.100000, y=0.500000, width=0.500000, height=0.500000",
		},
		{
			name:          "Y less than 0",
			path:          "testdata/pdf_handler_sample.pdf",
			x:             0.5,
			y:             -0.1,
			width:         0.5,
			height:        0.5,
			expectedError: "invalid input percentages: x=0.500000, y=-0.100000, width=0.500000, height=0.500000",
		},
		{
			name:          "Width less than 0",
			path:          "testdata/pdf_handler_sample.pdf",
			x:             0.5,
			y:             0.5,
			width:         -0.1,
			height:        0.5,
			expectedError: "invalid input percentages: x=0.500000, y=0.500000, width=-0.100000, height=0.500000",
		},
		{
			name:          "Height greater than 1",
			path:          "testdata/pdf_handler_sample.pdf",
			x:             0.5,
			y:             0.5,
			width:         0.5,
			height:        1.1,
			expectedError: "invalid input percentages: x=0.500000, y=0.500000, width=0.500000, height=1.100000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := setupPdfHandler(t)
			document := openTestPDF(t, tt.path)

			_, _, _, _, err := handler.LocationSizeToPdfPoints(context.Background(), document, 0, tt.x, tt.y, tt.width, tt.height)
			require.Error(t, err)
			require.EqualError(t, err, tt.expectedError)
		})
	}
}

func TestPdfHandler_TestLocationSizeToPdfPoints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		path           string
		X              float64
		Y              float64
		Width          float64
		Height         float64
		expectedX      float64
		expectedY      float64
		expectedWidth  float64
		expectedHeight float64
	}{
		{
			"upper left",
			"testdata/pdf_handler_sample.pdf",
			0, 0, 0, 0,
			0, 792.0, 0, 0,
		},
		{
			"bottom right",
			"testdata/pdf_handler_sample.pdf",
			1, 1, 1, 1,
			612.0, -792, 612.0, 792.0,
		},
		{
			"Center of the page",
			"testdata/pdf_handler_sample.pdf",
			0.5, 0.5, 0.5, 0.5,
			612.0 / 2, 0, 612.0 / 2, 792.0 / 2,
		},
		{
			"upper left rotated",
			"testdata/sample_rotate_90.pdf",
			0, 0, 0, 0,
			0, 612.0, 0, 0,
		},
		{
			"bottom right rotated",
			"testdata/sample_rotate_90.pdf",
			1, 1, 1, 1,
			792.0, -612, 792.0, 612.0,
		},
		{
			"Center of the rotated page",
			"testdata/sample_rotate_90.pdf",
			0.5, 0.5, 0.5, 0.5,
			792.0 / 2, 0, 792.0 / 2, 612.0 / 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := setupPdfHandler(t)
			handle := openTestPDF(t, tt.path)

			X, Y, Width, Height, err := handler.LocationSizeToPdfPoints(
				context.Background(),
				handle,
				0,
				tt.X,
				tt.Y,
				tt.Width,
				tt.Height,
			)
			require.NoError(t, err, "Failed to convert percentages relative to page dimensions to PDF Point for file: %s", tt.path)

			require.InDelta(t, tt.expectedX, X, 0.1, "Unexpected x for file: %s", tt.path)
			require.InDelta(t, tt.expectedY, Y, 0.1, "Unexpected y for file: %s", tt.path)
			require.InDelta(t, tt.expectedWidth, Width, 0.1, "Unexpected width for file: %s", tt.path)
			require.InDelta(t, tt.expectedHeight, Height, 0.1, "Unexpected height for file: %s", tt.path)
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

func setupPdfHandler(t *testing.T) PdfHandler {
	t.Helper()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	return PdfHandler{Logger: logger}
}

func openTestPDF(t *testing.T, filePath string) PdfDocument {
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

func addImageAndSave(t *testing.T, handler PdfHandler, document PdfDocument, params ImageParams, outputPath string) {
	t.Helper()

	err := handler.AddImageToPage(document, params)
	require.NoError(t, err, "failed to add image")

	err = handler.SavePDF(document, outputPath)
	require.NoError(t, err, "failed to save PDF")
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
				Location: struct {
					X float64
					Y float64
				}{X: 0, Y: 1 - 0.1452},
				Size: struct {
					Width  float64
					Height float64
				}{Width: 0.7239, Height: 0.1452},
				ImagePath: "testdata/test_signature.png",
			},
		},
		{
			name:       "Valid Image - A4 - Landscape",
			pdfPath:    "testdata/sample_rotate_90.pdf",
			outputPath: "tmp/output_rotate_90_add_image_to_page_valid_image.pdf",
			imageParams: ImageParams{
				Page: 0,
				Location: struct {
					X float64
					Y float64
				}{X: 0, Y: 1 - 0.1452},
				Size: struct {
					Width  float64
					Height float64
				}{Width: 0.7239, Height: 0.1452},
				ImagePath: "testdata/test_signature.png",
			},
		},
		{
			name:       "Valid Image - A4 - Portrait - 1/4 size - top right",
			pdfPath:    "testdata/sample_rotate_180.pdf",
			outputPath: "tmp/output_rotate_180_add_image_to_page_valid_image_top_right.pdf",
			imageParams: ImageParams{
				Page: 0,
				Location: struct {
					X float64
					Y float64
				}{X: 1 - 0.1810, Y: 0},
				Size: struct {
					Width  float64
					Height float64
				}{Width: 0.1810, Height: 0.0363},
				ImagePath: "testdata/test_signature.png",
			},
		},
		{
			name:       "Valid Image - A4 - Landscape",
			pdfPath:    "testdata/sample_rotate_270.pdf",
			outputPath: "tmp/output_rotate_270_add_image_to_page_valid_image.pdf",
			imageParams: ImageParams{
				Page: 0,
				Location: struct {
					X float64
					Y float64
				}{X: 1 - 0.5593, Y: 1 - 0.1879},
				Size: struct {
					Width  float64
					Height float64
				}{Width: 0.5593, Height: 0.1879},
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
		Location: struct {
			X float64
			Y float64
		}{X: 0, Y: 0},
		Size: struct {
			Width  float64
			Height float64
		}{Width: 0.1, Height: 0.1},
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
		Location: struct {
			X float64
			Y float64
		}{X: 0.5, Y: 0.1},
		Size: struct {
			Width  float64
			Height float64
		}{Width: 0.15, Height: 0.2},
		ImagePath: "testdata/test_signature-invalid.png",
	}

	err = handler.AddImageToPage(document, params)
	require.Error(t, err)
	require.Equal(t, "failure at the C/MuPDF add_image_to_page function: unknown image file format", err.Error())
}

func TestPdfHandler_TestGetFontAttributes_FontPath(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := PdfHandler{Logger: logger}

	tests := []struct {
		name, fontName string
		expectErr      bool
		isStandardFont bool
	}{
		{"Standard Font Courier", "Courier", false, true},
		{"Standard Font Courier-BoldOblique", "Courier-BoldOblique", false, true},
		{"Standard Font ZapfDingbats", "ZapfDingbats", false, true},
		{"Invalid Font", "NonExistentFont", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fontPath, _, err := handler.getFontAttributes(context.Background(), tt.fontName, 0)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)

				if tt.isStandardFont {
					require.Empty(t, fontPath, "Expected empty path for standard font %q", tt.fontName)
				} else {
					require.NotEmpty(t, fontPath, "Font path should not be empty for %q", tt.fontName)
					if _, pathErr := os.Stat(fontPath); os.IsNotExist(pathErr) {
						t.Errorf("Font path does not exist: %s", fontPath)
					} else if pathErr != nil {
						t.Errorf("Error checking font path: %v", pathErr)
					} else {
						t.Logf("Font path for %q: %s", tt.fontName, fontPath)
					}
				}
			}
		})
	}
}

func TestPdfHandler_TestGetFontAttributes_Descender(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := PdfHandler{Logger: logger}

	const epsilon = 0.05

	tests := []struct {
		name     string
		fontName string
		fontSize float64
		expected float64
	}{
		{
			name:     "Arial 12pt",
			fontName: "Arial",
			fontSize: 12.0,
			expected: 2.547,
		},
		{
			name:     "Times New Roman 10pt",
			fontName: "Times New Roman",
			fontSize: 10.0,
			expected: 2.19,
		},
		{
			name:     "Times New Roman 16pt",
			fontName: "Times New Roman",
			fontSize: 16.0,
			expected: 3.469,
		},
		{
			name:     "Courier 12pt",
			fontName: "Courier",
			fontSize: 12.0,
			expected: 2.328,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, descender, err := handler.getFontAttributes(context.Background(), tt.fontName, tt.fontSize)
			require.NoError(t, err)

			if math.Abs(descender-tt.expected) > epsilon {
				t.Errorf("got %.3f, expected %.3f ± %.2f", descender, tt.expected, epsilon)
			}
		})
	}
}

func TestPdfHandler_AddTextBoxToPage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		inputFile  string
		outputFile string
		params     TextParams
	}{
		{
			name:       "Text - A4 - Portrait - Times New Roman - 12",
			inputFile:  "testdata/pdf_handler_sample.pdf",
			outputFile: "tmp/output_rotate_0_add_text_to_page.pdf",
			params: TextParams{
				Value: "Hello, World!",
				Page:  0,
				Location: struct {
					X float64
					Y float64
				}{X: 0, Y: 0.984},
				Size: struct {
					Width  float64
					Height float64
				}{Width: 0.7239, Height: 0.015},
				Font: struct {
					Family string
					Size   float64
				}{Family: "Times New Roman", Size: 12},
			},
		},
		{
			name:       "Text - A4 - Landscape - Times New Roman Italic - 8",
			inputFile:  "testdata/sample_rotate_90.pdf",
			outputFile: "tmp/output_rotate_90_add_text_to_page.pdf",
			params: TextParams{
				Value: "Hello, World!",
				Page:  0,
				Location: struct {
					X float64
					Y float64
				}{X: 0, Y: 0.9866},
				Size: struct {
					Width  float64
					Height float64
				}{Width: 0.0617, Height: 0.0134},
				Font: struct {
					Family string
					Size   float64
				}{Family: "Times New Roman Italic", Size: 8},
			},
		},
		{
			name:       "Text - A4 - Landscape - Times New Roman Bold - 8 - top right",
			inputFile:  "testdata/sample_rotate_270.pdf",
			outputFile: "tmp/output_rotate_270_add_text_to_page_top_right.pdf",
			params: TextParams{
				Value: "Hello, World!",
				Page:  0,
				Location: struct {
					X float64
					Y float64
				}{X: 1 - 0.063, Y: 0},
				Size: struct {
					Width  float64
					Height float64
				}{Width: 0.0617, Height: 0.0134},
				Font: struct {
					Family string
					Size   float64
				}{Family: "Times New Roman Bold", Size: 8},
			},
		},
		{
			name:       "Text - A4 - Portrait - Times New Roman - 24 - top right",
			inputFile:  "testdata/sample_rotate_180.pdf",
			outputFile: "tmp/output_rotate_180_add_text_to_page_top_right_24_fontsize.pdf",
			params: TextParams{
				Value: "Hello, World!",
				Page:  0,
				Location: struct {
					X float64
					Y float64
				}{X: 1 - 0.294, Y: 0},
				Size: struct {
					Width  float64
					Height float64
				}{Width: 0.262, Height: 0.0285},
				Font: struct {
					Family string
					Size   float64
				}{Family: "Times New Roman", Size: 24},
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

			err = handler.AddTextBoxToPage(document, tt.params)
			require.NoError(t, err, "failed to add text")

			err = handler.SavePDF(document, tt.outputFile)
			require.NoError(t, err, "failed to save PDF")
		})
	}
}

func TestPdfHandler_AddTextBoxToPage_InvalidPage(t *testing.T) {
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

	params := TextParams{
		Value: "Hello, World!",
		Page:  1,
		Location: struct {
			X float64
			Y float64
		}{X: 0, Y: 1},
		Font: struct {
			Family string
			Size   float64
		}{Family: "Courier", Size: 12},
	}

	err = handler.AddTextBoxToPage(document, params)
	require.Error(t, err)
	require.Equal(t, "failure at the AddTextBoxToPage function: failed to get page size: failure at the C/MuPDF get_page_size function: invalid page number: 2", err.Error())
}

func TestPdfHandler_AddTextBoxToPage_InvalidTextLengh(t *testing.T) {
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

	params := TextParams{
		Value: strings.Repeat("a", 301),
		Page:  0,
		Location: struct {
			X float64
			Y float64
		}{X: 0, Y: 1},
		Font: struct {
			Family string
			Size   float64
		}{Family: "Courier", Size: 12},
	}

	err = handler.AddTextBoxToPage(document, params)
	require.Error(t, err)
	require.Equal(t, "failure at the C/MuPDF add_text_to_page function: Text exceeds maximum allowed size. Expected: 300, Actual: 301", err.Error())
}

func TestPdfHandler_AddTextBoxToPage_InvalidFont(t *testing.T) {
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

	params := TextParams{
		Value: "Hello, World!",
		Page:  1,
		Location: struct {
			X float64
			Y float64
		}{X: 0, Y: 0},
		Font: struct {
			Family string
			Size   float64
		}{Family: "[not existing font]", Size: 12},
	}

	err = handler.AddTextBoxToPage(document, params)
	require.Error(t, err)
	require.Equal(t, "failure at PdfHandler AddTextBoxToPage function: failed to find font path for \"[not existing font]\"", err.Error())
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
				Location: struct {
					X float64
					Y float64
				}{X: 0, Y: 1 - 0.0253},
				Size: struct {
					Width  float64
					Height float64
				}{Width: 0.0327, Height: 0.0253},
			},
		},
		{
			name:       "Checkbox - A4 - Landscape - Bottom Left",
			inputFile:  "testdata/sample_rotate_90.pdf",
			outputFile: "tmp/output_rotate_90_add_checkbox_bottom_left.pdf",
			params: CheckboxParams{
				Value: false,
				Page:  0,
				Location: struct {
					X float64
					Y float64
				}{X: 0, Y: 1 - 0.0490},
				Size: struct {
					Width  float64
					Height float64
				}{Width: 0.0379, Height: 0.0490},
			},
		},
		{
			name:       "Checkbox - A4 - Portrait - Bottom Right",
			inputFile:  "testdata/sample_rotate_180.pdf",
			outputFile: "tmp/output_rotate_180_add_checkbox_bottom_right.pdf",
			params: CheckboxParams{
				Value: true,
				Page:  0,
				Location: struct {
					X float64
					Y float64
				}{X: 1 - 0.065, Y: 1 - 0.051},
				Size: struct {
					Width  float64
					Height float64
				}{Width: 0.065, Height: 0.051},
			},
		},
		{
			name:       "Checkbox - A4 - Landscape - Top Right",
			inputFile:  "testdata/sample_rotate_270.pdf",
			outputFile: "tmp/output_rotate_270_add_checkbox_top_right.pdf",
			params: CheckboxParams{
				Value: true,
				Page:  0,
				Location: struct {
					X float64
					Y float64
				}{X: 1 - 0.063, Y: 0},
				Size: struct {
					Width  float64
					Height float64
				}{Width: 0.063, Height: 0.082},
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
		Location: struct {
			X float64
			Y float64
		}{X: 50, Y: 100},
		Size: struct {
			Width  float64
			Height float64
		}{Width: 20, Height: 20},
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

func TestPdfHandler_MultipleOperations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		inputFile  string
		outputFile string
		operations []func(handler PdfHandler, document PdfDocument) error
	}{
		{
			name:       "Multiple operations on sample.pdf",
			inputFile:  "testdata/pdf_handler_sample.pdf",
			outputFile: "tmp/output_multiple_operations.pdf",
			operations: []func(handler PdfHandler, document PdfDocument) error{
				func(handler PdfHandler, document PdfDocument) error {
					params := TextParams{
						Value: "The quick brown fox jumps over the lazy dog!",
						Page:  0,
						Location: struct {
							X float64
							Y float64
						}{X: 0.0817, Y: 0.0530},
						Font: struct {
							Family string
							Size   float64
						}{Family: "Courier", Size: 12},
					}
					return handler.AddTextBoxToPage(document, params)
				},
				func(handler PdfHandler, document PdfDocument) error {
					params := TextParams{
						Value: "The quick brown fox jumps over the lazy dog!",
						Page:  0,
						Location: struct {
							X float64
							Y float64
						}{X: 0.082, Y: 0.091},
						Font: struct {
							Family string
							Size   float64
						}{Family: "Courier", Size: 14},
					}
					return handler.AddTextBoxToPage(document, params)
				},
				func(handler PdfHandler, document PdfDocument) error {
					params := ImageParams{
						Page: 0,
						Location: struct {
							X float64
							Y float64
						}{X: 0.163, Y: 0.179},
						Size: struct {
							Width  float64
							Height float64
						}{Width: 0.163, Height: 0.063},
						ImagePath: "testdata/test_signature.png",
					}
					return handler.AddImageToPage(document, params)
				},
				func(handler PdfHandler, document PdfDocument) error {
					params := CheckboxParams{
						Value: true,
						Page:  0,
						Location: struct {
							X float64
							Y float64
						}{X: 0.245, Y: 0.242},
						Size: struct {
							Width  float64
							Height float64
						}{Width: 0.0327, Height: 0.0253},
					}
					return handler.AddCheckboxToPage(document, params)
				},
				func(handler PdfHandler, document PdfDocument) error {
					params := CheckboxParams{
						Value: false,
						Page:  0,
						Location: struct {
							X float64
							Y float64
						}{X: 0.245, Y: 0.280},
						Size: struct {
							Width  float64
							Height float64
						}{Width: 0.0327, Height: 0.0253},
					}
					return handler.AddCheckboxToPage(document, params)
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

			for _, operation := range tt.operations {
				err := operation(handler, document)
				require.NoError(t, err, "Operation failed")
			}

			err = handler.SavePDF(document, tt.outputFile)
			require.NoError(t, err, "Failed to save PDF")
		})
	}
}

func TestPdfHandler_MultipleOperationsOnTextboxes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		inputFile  string
		outputFile string
		operations []func(handler PdfHandler, document PdfDocument) error
	}{
		{
			name:       "Multiple operations on texboxes.pdf",
			inputFile:  "testdata/textboxes.pdf",
			outputFile: "tmp/output_textboxes.pdf",
			operations: []func(handler PdfHandler, document PdfDocument) error{
				func(handler PdfHandler, document PdfDocument) error {
					params := ImageParams{
						Page: 0,
						Location: struct {
							X float64
							Y float64
						}{X: 0.0087145969, Y: 0.0033670034},
						Size: struct {
							Width  float64
							Height float64
						}{Width: 0.1633986928, Height: 0.0151515152},
						ImagePath: "testdata/test_blue_box.png",
					}
					return handler.AddImageToPage(document, params)
				},
				func(handler PdfHandler, document PdfDocument) error {
					params := ImageParams{
						Page: 0,
						Location: struct {
							X float64
							Y float64
						}{X: 0.0889615327, Y: 0.017957347},
						Size: struct {
							Width  float64
							Height float64
						}{Width: 0.1633986928, Height: 0.0151515152},
						ImagePath: "testdata/test_blue_box.png",
					}
					return handler.AddImageToPage(document, params)
				},
				func(handler PdfHandler, document PdfDocument) error {
					params := ImageParams{
						Page: 0,
						Location: struct {
							X float64
							Y float64
						}{X: 0.0087145969, Y: 0.962962963},
						Size: struct {
							Width  float64
							Height float64
						}{Width: 0.1633986928, Height: 0.0151515152},
						ImagePath: "testdata/test_blue_box.png",
					}
					return handler.AddImageToPage(document, params)
				},
				func(handler PdfHandler, document PdfDocument) error {
					params := TextParams{
						Value: "Qjstom",
						Page:  0,
						Location: struct {
							X float64
							Y float64
						}{X: 0.0087145969, Y: 0.0033670034},
						Size: struct {
							Width  float64
							Height float64
						}{Width: 0.1633986928, Height: 0.0151515152},
						Font: struct {
							Family string
							Size   float64
						}{Family: "Times New Roman", Size: 12},
					}
					return handler.AddTextBoxToPage(document, params)
				},
				func(handler PdfHandler, document PdfDocument) error {
					params := TextParams{
						Value: "qjWaAJj",
						Page:  0,
						Location: struct {
							X float64
							Y float64
						}{X: 0.0889615327, Y: 0.017957347},
						Size: struct {
							Width  float64
							Height float64
						}{Width: 0.1633986928, Height: 0.0151515152},
						Font: struct {
							Family string
							Size   float64
						}{Family: "Times New Roman", Size: 12},
					}
					return handler.AddTextBoxToPage(document, params)
				},
				func(handler PdfHandler, document PdfDocument) error {
					params := TextParams{
						Value: "QqWwJj",
						Page:  0,
						Location: struct {
							X float64
							Y float64
						}{X: 0.0087145969, Y: 0.962962963},
						Size: struct {
							Width  float64
							Height float64
						}{Width: 0.1633986928, Height: 0.0151515152},
						Font: struct {
							Family string
							Size   float64
						}{Family: "Times New Roman", Size: 12},
					}
					return handler.AddTextBoxToPage(document, params)
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

			for _, operation := range tt.operations {
				err := operation(handler, document)
				require.NoError(t, err, "Operation failed")
			}

			err = handler.SavePDF(document, tt.outputFile)
			require.NoError(t, err, "Failed to save PDF")
		})
	}
}

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
