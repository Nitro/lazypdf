package lazypdf

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSaveToPNGOK(t *testing.T) {
	for i := uint16(0); i < 13; i++ {
		file, err := os.Open("testdata/sample.pdf")
		require.NoError(t, err)
		defer func() { require.NoError(t, file.Close()) }()

		buf := bytes.NewBuffer([]byte{})
		err = SaveToPNG(context.Background(), i, 0, 0, file, buf)
		require.NoError(t, err)

		expectedPage, err := ioutil.ReadFile(fmt.Sprintf("testdata/sample_page%d.png", i))
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

	err = SaveToPNG(context.Background(), 0, 0, 0, file, nil)
	require.Error(t, err)
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
	buf, err := ioutil.ReadFile("testdata/sample.pdf")
	require.NoError(b, err)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		input := bytes.NewBuffer(buf)
		output := bytes.NewBuffer([]byte{})
		err := SaveToPNG(context.Background(), page, 0, 0, input, output)
		require.NoError(b, err)
	}
}
