package lazypdf

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// The directory list is a superset of the Linux and macOS layouts, so several entries are absent on any
// given host. A missing directory used to abort the whole search, which both hid fonts installed in a
// later directory and reported a font that is genuinely absent as a filesystem error.
func TestPdfHandler_GetFontAttributes_MissingDirectoriesAreSkipped(t *testing.T) {
	t.Parallel()

	handler := PdfHandler{Logger: slog.New(slog.NewJSONHandler(os.Stdout, nil))}

	_, _, err := handler.getFontAttributes(context.Background(), "NoSuchFontAnywhere", 12)
	require.EqualError(t, err, `font "NoSuchFontAnywhere" not found`)
}

// The space-substituting variants cover the Linux packaging of these fonts ("Times_New_Roman.ttf"), but
// macOS installs them under their spaced names, so the untransformed name has to be a candidate too.
func TestPdfHandler_GenerateFontCandidates_IncludesUntransformedName(t *testing.T) {
	t.Parallel()

	handler := PdfHandler{Logger: slog.New(slog.NewJSONHandler(os.Stdout, nil))}

	candidates := handler.generateFontCandidates(context.Background(), "Times New Roman")

	require.Subset(t, candidates, []string{
		"Times New Roman.ttf",
		"Times New Roman.otf",
		"Times_New_Roman.ttf",
		"Times-New-Roman.ttf",
		"TimesNewRoman.ttf",
	})
}
