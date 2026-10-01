// nolint
package lazypdf

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

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

func TestPdfHandler_GetFontAttributes_Descender(t *testing.T) {
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
				Location: Location{
					X: 0,
					Y: 779,
				},
				Size: Size{
					Width:  443,
					Height: 12,
				},
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
				Location: Location{
					X: 0,
					Y: 603,
				},
				Size: Size{
					Width:  50,
					Height: 8,
				},
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
				Location: Location{
					X: 742,
					Y: 0,
				},
				Size: Size{
					Width:  49,
					Height: 8,
				},
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
				Location: Location{
					X: 432,
					Y: 0,
				},
				Size: Size{
					Width:  160,
					Height: 24,
				},
				Font: struct {
					Family string
					Size   float64
				}{Family: "Times New Roman", Size: 24},
			},
		},
		{
			name:       "Text - unreliable content stream",
			inputFile:  "testdata/sample_content.pdf",
			outputFile: "tmp/output_add_text_to_unreliable_content_stream.pdf",
			params: TextParams{
				Value: "Hello, World!",
				Page:  0,
				Location: Location{
					X: 0,
					Y: 780,
				},
				Size: Size{
					Width:  443,
					Height: 12,
				},
				Font: struct {
					Family string
					Size   float64
				}{Family: "Times New Roman", Size: 12},
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
		Location: Location{
			X: 0.0,
			Y: 632,
		},
		Size: Size{
			Width:  443,
			Height: 12,
		},
		Font: struct {
			Family string
			Size   float64
		}{Family: "Times New Roman", Size: 12},
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
		Value: strings.Repeat("a", 501),
		Page:  0,
		Location: Location{
			X: 0.0,
			Y: 0.0,
		},
		Size: Size{
			Width:  443,
			Height: 12,
		},
		Font: struct {
			Family string
			Size   float64
		}{Family: "Times New Roman", Size: 12},
	}

	err = handler.AddTextBoxToPage(document, params)
	require.Error(t, err)
	require.Equal(t, "failure at the C/MuPDF add_text_to_page function: Text exceeds maximum allowed size. Expected: 500, Actual: 501", err.Error())
}

// The limit counts characters, not bytes, so multi-byte text is allowed the same
// number of characters as ASCII text.
func TestPdfHandler_AddTextBoxToPage_MultiByteTextAtLimit(t *testing.T) {
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

	// 500 characters, 1500 bytes in UTF-8.
	params := TextParams{
		Value: strings.Repeat("漢", 500),
		Page:  0,
		Location: Location{
			X: 0.0,
			Y: 0.0,
		},
		Size: Size{
			Width:  443,
			Height: 12,
		},
		Font: struct {
			Family string
			Size   float64
		}{Family: "Times New Roman", Size: 12},
	}

	require.NoError(t, handler.AddTextBoxToPage(document, params))
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
		Location: Location{
			X: 0.0,
			Y: 0.0,
		},
		Size: Size{
			Width:  443,
			Height: 12,
		},
		Font: struct {
			Family string
			Size   float64
		}{Family: "[not existing font]", Size: 12},
	}

	err = handler.AddTextBoxToPage(document, params)
	require.Error(t, err)
	require.Equal(t, "failure at PdfHandler AddTextBoxToPage function: failed to find font path for \"[not existing font]\"", err.Error())
}

func TestPdfHandler_MultipleOperations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		inputFile  string
		outputFile string
		operations []func(handler PdfHandler, document *PdfDocument) error
	}{
		{
			name:       "Multiple operations on pdf_handler_sample.pdf",
			inputFile:  "testdata/pdf_handler_sample.pdf",
			outputFile: "tmp/output_multiple_operations.pdf",
			operations: []func(handler PdfHandler, document *PdfDocument) error{
				func(handler PdfHandler, document *PdfDocument) error {
					params := TextParams{
						Value: "The quick brown fox jumps over the lazy dog!",
						Page:  0,
						Location: Location{
							X: 50,
							Y: 41,
						},
						Size: Size{
							Width:  443,
							Height: 12,
						},
						Font: struct {
							Family string
							Size   float64
						}{Family: "Courier", Size: 12},
					}
					return handler.AddTextBoxToPage(document, params)
				},
				func(handler PdfHandler, document *PdfDocument) error {
					params := TextParams{
						Value: "The quick brown fox jumps over the lazy dog!",
						Page:  0,
						Location: Location{
							X: 50,
							Y: 72,
						},
						Size: Size{
							Width:  443,
							Height: 14,
						},
						Font: struct {
							Family string
							Size   float64
						}{Family: "Courier", Size: 14},
					}
					return handler.AddTextBoxToPage(document, params)
				},
				func(handler PdfHandler, document *PdfDocument) error {
					params := ImageParams{
						Page: 0,
						Location: Location{
							X: 100,
							Y: 141,
						},
						Size: Size{
							Width:  100,
							Height: 50,
						},
						ImagePath: "testdata/test_signature.png",
					}
					return handler.AddImageToPage(document, params)
				},
				func(handler PdfHandler, document *PdfDocument) error {
					params := CheckboxParams{
						Value: true,
						Page:  0,
						Location: Location{
							X: 149,
							Y: 191,
						},
						Size: Size{
							Width:  20,
							Height: 20,
						},
					}
					return handler.AddCheckboxToPage(document, params)
				},
				func(handler PdfHandler, document *PdfDocument) error {
					params := CheckboxParams{
						Value: false,
						Page:  0,
						Location: Location{
							X: 150,
							Y: 221,
						},
						Size: Size{
							Width:  20,
							Height: 20,
						},
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
		operations []func(handler PdfHandler, document *PdfDocument) error
	}{
		{
			name:       "Multiple operations on texboxes.pdf",
			inputFile:  "testdata/textboxes.pdf",
			outputFile: "tmp/output_textboxes.pdf",
			operations: []func(handler PdfHandler, document *PdfDocument) error{
				func(handler PdfHandler, document *PdfDocument) error {
					params := ImageParams{
						Page: 0,
						Location: Location{
							X: 5,
							Y: 2,
						},
						Size: Size{
							Width:  100,
							Height: 12,
						},
						ImagePath: "testdata/test_blue_box.png",
					}
					return handler.AddImageToPage(document, params)
				},
				func(handler PdfHandler, document *PdfDocument) error {
					params := ImageParams{
						Page: 0,
						Location: Location{
							X: 54,
							Y: 18,
						},
						Size: Size{
							Width:  100,
							Height: 12,
						},
						ImagePath: "testdata/test_blue_box.png",
					}
					return handler.AddImageToPage(document, params)
				},
				func(handler PdfHandler, document *PdfDocument) error {
					params := ImageParams{
						Page: 0,
						Location: Location{
							X: 5,
							Y: 768,
						},
						Size: Size{
							Width:  100,
							Height: 12,
						},
						ImagePath: "testdata/test_blue_box.png",
					}
					return handler.AddImageToPage(document, params)
				},
				func(handler PdfHandler, document *PdfDocument) error {
					params := TextParams{
						Value: "Qjstom",
						Page:  0,
						Location: Location{
							X: 5,
							Y: 2,
						},
						Size: Size{
							Width:  100,
							Height: 12,
						},
						Font: struct {
							Family string
							Size   float64
						}{Family: "Times New Roman", Size: 12},
					}
					return handler.AddTextBoxToPage(document, params)
				},
				func(handler PdfHandler, document *PdfDocument) error {
					params := TextParams{
						Value: "qjWaAJj",
						Page:  0,
						Location: Location{
							X: 54,
							Y: 18,
						},
						Size: Size{
							Width:  100,
							Height: 12,
						},
						Font: struct {
							Family string
							Size   float64
						}{Family: "Times New Roman", Size: 12},
					}
					return handler.AddTextBoxToPage(document, params)
				},
				func(handler PdfHandler, document *PdfDocument) error {
					params := TextParams{
						Value: "QqWwJj",
						Page:  0,
						Location: Location{
							X: 5,
							Y: 768,
						},
						Size: Size{
							Width:  100,
							Height: 12,
						},
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

func TestPdfHandler_WrapPageContents(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := NewPdfHandler(context.Background(), logger)

	file, err := os.Open("testdata/textboxes.pdf")
	require.NoError(t, err)
	defer func() { require.NoError(t, file.Close()) }()

	document, err := handler.OpenPDF(file)
	require.NoError(t, err)
	defer func() { require.NoError(t, handler.ClosePDF(document)) }()

	// Initially, no pages should be wrapped
	require.False(t, document.wrappedPages[0])

	// First call to wrapPageContents for page 0 should mark it as wrapped
	err = handler.wrapPageContents(context.Background(), document, 0)
	require.NoError(t, err)
	require.True(t, document.wrappedPages[0])

	// Second call to wrapPageContents for page 0 should not error and page should still be marked as wrapped
	err = handler.wrapPageContents(context.Background(), document, 0)
	require.NoError(t, err)
	require.True(t, document.wrappedPages[0])

	// Test that other pages are not affected
	require.False(t, document.wrappedPages[1])
	require.False(t, document.wrappedPages[2])
}

func TestPdfHandler_WrapPageContents_InvalidPage(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := NewPdfHandler(context.Background(), logger)

	file, err := os.Open("testdata/pdf_handler_sample.pdf")
	require.NoError(t, err)
	defer func() { require.NoError(t, file.Close()) }()

	document, err := handler.OpenPDF(file)
	require.NoError(t, err)
	defer func() { require.NoError(t, handler.ClosePDF(document)) }()

	// Call wrapPageContents with invalid page number should return error
	err = handler.wrapPageContents(context.Background(), document, 2)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failure at wrap_page_contents_for_page")
}

func TestPdfHandler_WrapPageContents_Integration(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := NewPdfHandler(context.Background(), logger)

	file, err := os.Open("testdata/textboxes.pdf")
	require.NoError(t, err)
	defer func() { require.NoError(t, file.Close()) }()

	document, err := handler.OpenPDF(file)
	require.NoError(t, err)
	defer func() { require.NoError(t, handler.ClosePDF(document)) }()

	// Test that annotation functions automatically wrap page contents
	require.False(t, document.wrappedPages[0])
	textParams := TextParams{
		Value:    "Test Text",
		Page:     0,
		Location: Location{X: 60, Y: 80},
		Size:     Size{Width: 180, Height: 40},
		Font: struct {
			Family string
			Size   float64
		}{
			Family: "Times New Roman",
			Size:   12,
		},
	}
	err = handler.AddTextBoxToPage(document, textParams)
	require.NoError(t, err)
	require.True(t, document.wrappedPages[0])

	// Test that annotation functions automatically wrap page contents
	require.False(t, document.wrappedPages[1])
	imageParams := ImageParams{
		Page:      1,
		Location:  Location{X: 306, Y: 396},
		Size:      Size{Width: 122, Height: 158},
		ImagePath: "testdata/test_signature.png",
	}
	err = handler.AddImageToPage(document, imageParams)
	require.NoError(t, err)
	require.True(t, document.wrappedPages[1]) // Should still be true

	// Test that annotation functions automatically wrap page contents
	require.False(t, document.wrappedPages[2])
	checkboxParams := CheckboxParams{
		Value:    true,
		Page:     2,
		Location: Location{X: 428, Y: 554},
		Size:     Size{Width: 30, Height: 40},
	}
	err = handler.AddCheckboxToPage(document, checkboxParams)
	require.NoError(t, err)
	require.True(t, document.wrappedPages[2])
}

func BenchmarkPdfHandler_WrapPageContentsPerformance(b *testing.B) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := NewPdfHandler(context.Background(), logger)

	file, err := os.Open("testdata/pdf_handler_sample.pdf")
	require.NoError(b, err)
	defer func() { require.NoError(b, file.Close()) }()

	document, err := handler.OpenPDF(file)
	require.NoError(b, err)
	defer func() { require.NoError(b, handler.ClosePDF(document)) }()

	// Add first annotation to trigger initial wrap_page_contents call
	// This is outside the benchmark timing
	firstTextParams := TextParams{
		Value:    "First annotation - setup",
		Page:     0,
		Location: Location{X: 61, Y: 80},
		Size:     Size{Width: 183, Height: 40},
		Font: struct {
			Family string
			Size   float64
		}{
			Family: "Times New Roman",
			Size:   12,
		},
	}
	err = handler.AddTextBoxToPage(document, firstTextParams)
	require.NoError(b, err)

	require.True(b, document.wrappedPages[0], "Page should be wrapped after first annotation")

	// Reset timer to exclude setup time
	b.ResetTimer()
	b.ReportAllocs()

	// Benchmark loop - all subsequent annotations should be faster
	// since wrapPageContents will return early (already wrapped)
	for i := 0; i < b.N; i++ {
		textParams := TextParams{
			Value:    fmt.Sprintf("Benchmark annotation %d", i),
			Page:     0,
			Location: Location{X: 61, Y: 158 + float64(i%10)*40},
			Size:     Size{Width: 183, Height: 32},
			Font: struct {
				Family string
				Size   float64
			}{
				Family: "Times New Roman",
				Size:   10,
			},
		}
		err = handler.AddTextBoxToPage(document, textParams)
		require.NoError(b, err)
	}
}

func TestPdfHandler_AddTextBoxToPage_UnicodeAndSpecialCharacters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		text        string
		fontFamily  string
		description string
	}{
		// Latin accents and diacritics
		{
			name:        "French accents",
			text:        "Frédéric Château",
			fontFamily:  "Times New Roman",
			description: "French text with acute and circumflex accents",
		},
		{
			name:        "Spanish accents",
			text:        "José García Señor",
			fontFamily:  "Times New Roman",
			description: "Spanish text with acute accent and tilde",
		},
		{
			name:        "German umlauts",
			text:        "Müller Größe Übung",
			fontFamily:  "Times New Roman",
			description: "German text with umlauts and eszett",
		},
		{
			name:        "Portuguese tildes",
			text:        "São Paulo não",
			fontFamily:  "Times New Roman",
			description: "Portuguese text with tilde",
		},
		{
			name:        "Nordic characters",
			text:        "Søren Ångström",
			fontFamily:  "Times New Roman",
			description: "Nordic text with ø and å",
		},
		{
			name:        "Mixed Latin accents",
			text:        "Café résumé naïve",
			fontFamily:  "Times New Roman",
			description: "Mixed Latin accents: acute, diaeresis",
		},
		{
			name:        "Eastern European",
			text:        "Kraków Łódź",
			fontFamily:  "Times New Roman",
			description: "Polish characters with stroke and acute",
		},
		{
			name:        "Czech and Slovak",
			text:        "Přemysl Dvořák",
			fontFamily:  "Times New Roman",
			description: "Czech characters with caron",
		},

		// Greek alphabet
		{
			name:        "Greek modern",
			text:        "Ελληνικά Αθήνα",
			fontFamily:  "Times New Roman",
			description: "Modern Greek characters",
		},
		{
			name:        "Greek with accents",
			text:        "Καλημέρα κόσμε",
			fontFamily:  "Times New Roman",
			description: "Greek with accent marks",
		},

		// Cyrillic alphabet
		{
			name:        "Russian",
			text:        "Привет мир",
			fontFamily:  "Times New Roman",
			description: "Russian Cyrillic text",
		},
		{
			name:        "Ukrainian",
			text:        "Київ Україна",
			fontFamily:  "Times New Roman",
			description: "Ukrainian Cyrillic with specific characters",
		},

		// PDF special characters that need escaping
		{
			name:        "Parentheses",
			text:        "Text with (parentheses) inside",
			fontFamily:  "Times New Roman",
			description: "Parentheses must be escaped in PDF strings",
		},
		{
			name:        "Backslashes",
			text:        "Path\\to\\file",
			fontFamily:  "Times New Roman",
			description: "Backslashes must be escaped",
		},
		{
			name:        "Mixed special chars",
			text:        "Formula: (a\\b) = c",
			fontFamily:  "Times New Roman",
			description: "Combined parentheses and backslashes",
		},
		{
			name:        "Complex expression",
			text:        "f(x) = \\frac{1}{x}",
			fontFamily:  "Times New Roman",
			description: "Mathematical expression with special chars",
		},

		// Combined complexity
		{
			name:        "Accents with special chars",
			text:        "Frédéric's (café)",
			fontFamily:  "Times New Roman",
			description: "Combining accents with PDF special characters",
		},
		{
			name:        "Real world example",
			text:        "€50 für Müller (Zürich)",
			fontFamily:  "Times New Roman",
			description: "Real-world text with multiple Unicode types",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
			handler := NewPdfHandler(context.Background(), logger)

			file, err := os.Open("testdata/pdf_handler_sample.pdf")
			require.NoError(t, err)
			defer func() { require.NoError(t, file.Close()) }()

			document, err := handler.OpenPDF(file)
			require.NoError(t, err, "OpenPDF failed")
			defer func() { require.NoError(t, handler.ClosePDF(document)) }()

			params := TextParams{
				Value: tt.text,
				Page:  0,
				Location: Location{
					X: 50,
					Y: 100,
				},
				Size: Size{
					Width:  400,
					Height: 20,
				},
				Font: struct {
					Family string
					Size   float64
				}{Family: tt.fontFamily, Size: 12},
			}

			err = handler.AddTextBoxToPage(document, params)
			require.NoError(t, err, "failed to add text: %s (description: %s)", tt.text, tt.description)

			outputFile := fmt.Sprintf("tmp/output_unicode_%s.pdf", strings.ReplaceAll(tt.name, " ", "_"))
			err = handler.SavePDF(document, outputFile)
			require.NoError(t, err, "failed to save PDF")

			// Verify file exists and has content
			info, err := os.Stat(outputFile)
			require.NoError(t, err, "output file should exist")
			require.Greater(t, info.Size(), int64(0), "output file should have content")
		})
	}
}
