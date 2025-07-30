package lazypdf

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// normalizeLineEndings converts all line endings to \n
func normalizeLineEndings(data []byte) []byte {
	s := string(data)
	s = strings.ReplaceAll(s, "\r\n", "\n") // Windows to Unix
	s = strings.ReplaceAll(s, "\r", "\n")   // Old Mac to Unix
	return []byte(s)
}

func TestSaveToHTMLOK(t *testing.T) {
	for i := uint16(0); i < 13; i++ {
		file, err := os.Open("testdata/sample.pdf")
		require.NoError(t, err)
		defer func() { require.NoError(t, file.Close()) }()

		buf := bytes.NewBuffer([]byte{})
		err = SaveToHTML(context.Background(), i, 0, 0, 0, file, buf)
		require.NoError(t, err)

		expectedPage, err := os.ReadFile(fmt.Sprintf("testdata/sample_page%d.html", i))
		require.NoError(t, err)
		resultPage, err := io.ReadAll(buf)
		require.NoError(t, err)

		expectedNormalized := normalizeLineEndings(expectedPage)
		resultNormalized := normalizeLineEndings(resultPage)
		require.Equal(t, expectedNormalized, resultNormalized)
	}
}

func TestSaveToHTMLFail(t *testing.T) {
	file, err := os.Open("testdata/sample-invalid.pdf")
	require.NoError(t, err)
	defer func() { require.NoError(t, file.Close()) }()

	err = SaveToHTML(context.Background(), 0, 0, 0, 0, file, bytes.NewBuffer([]byte{}))
	require.Error(t, err)
	require.Equal(t, "failure at the C/MuPDF layer: no objects found", err.Error())
}

func TestSaveToPNGOK(t *testing.T) {
	for i := uint16(0); i < 13; i++ {
		file, err := os.Open("testdata/sample.pdf")
		require.NoError(t, err)
		defer func() { require.NoError(t, file.Close()) }()

		buf := bytes.NewBuffer([]byte{})
		err = SaveToPNG(context.Background(), i, 0, 0, 0, file, buf)
		require.NoError(t, err)

		expectedPage, err := os.ReadFile(fmt.Sprintf("testdata/sample_page%d.png", i))
		require.NoError(t, err)
		resultPage, err := io.ReadAll(buf)
		require.NoError(t, err)
		require.Equal(t, expectedPage, resultPage)
	}
}

func TestSaveToPNGFail(t *testing.T) {
	file, err := os.Open("testdata/sample-invalid.pdf")
	require.NoError(t, err)
	defer func() { require.NoError(t, file.Close()) }()

	err = SaveToPNG(context.Background(), 0, 0, 0, 0, file, bytes.NewBuffer([]byte{}))
	require.Error(t, err)
	require.Equal(t, "failure at the C/MuPDF layer: no objects found", err.Error())
}

func TestPageCount(t *testing.T) {
	file, err := os.Open("testdata/sample.pdf")
	require.NoError(t, err)
	defer func() { require.NoError(t, file.Close()) }()

	count, err := PageCount(context.Background(), file)
	require.NoError(t, err)
	require.Equal(t, 13, count)
}

func TestPageCountFail(t *testing.T) {
	file, err := os.Open("testdata/sample-invalid.pdf")
	require.NoError(t, err)
	defer func() { require.NoError(t, file.Close()) }()

	_, err = PageCount(context.Background(), file)
	require.Error(t, err)
	require.Equal(t, "failure at the C/MuPDF layer: no objects found", err.Error())
}

func BenchmarkSaveToPNGPage0(b *testing.B)  { benchmarkSaveToPNGRunner(0, b) }
func BenchmarkSaveToPNGPage1(b *testing.B)  { benchmarkSaveToPNGRunner(1, b) }
func BenchmarkSaveToPNGPage2(b *testing.B)  { benchmarkSaveToPNGRunner(2, b) }
func BenchmarkSaveToPNGPage3(b *testing.B)  { benchmarkSaveToPNGRunner(3, b) }
func BenchmarkSaveToPNGPage4(b *testing.B)  { benchmarkSaveToPNGRunner(4, b) }
func BenchmarkSaveToPNGPage5(b *testing.B)  { benchmarkSaveToPNGRunner(5, b) }
func BenchmarkSaveToPNGPage6(b *testing.B)  { benchmarkSaveToPNGRunner(6, b) }
func BenchmarkSaveToPNGPage7(b *testing.B)  { benchmarkSaveToPNGRunner(7, b) }
func BenchmarkSaveToPNGPage8(b *testing.B)  { benchmarkSaveToPNGRunner(8, b) }
func BenchmarkSaveToPNGPage9(b *testing.B)  { benchmarkSaveToPNGRunner(9, b) }
func BenchmarkSaveToPNGPage10(b *testing.B) { benchmarkSaveToPNGRunner(10, b) }
func BenchmarkSaveToPNGPage11(b *testing.B) { benchmarkSaveToPNGRunner(11, b) }
func BenchmarkSaveToPNGPage12(b *testing.B) { benchmarkSaveToPNGRunner(12, b) }

func benchmarkSaveToPNGRunner(page uint16, b *testing.B) {
	buf, err := os.ReadFile("testdata/sample.pdf")
	require.NoError(b, err)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		input := bytes.NewBuffer(buf)
		output := bytes.NewBuffer([]byte{})
		err := SaveToPNG(context.Background(), page, 0, 0, 0, input, output)
		require.NoError(b, err)
	}
}
