package lazypdf

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	ddTracer "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

// SaveToPNG is used to convert a page from a PDF file to PNG.
func SaveToPNG(ctx context.Context, page, width uint16, scale float32, payload io.Reader, output io.Writer) error {
	span, _ := ddTracer.StartSpanFromContext(ctx, "lazypdf.SaveToPNG")
	defer span.Finish()

	if payload == nil {
		return errors.New("payload can't be nil")
	}
	if output == nil {
		return errors.New("output can't be nil")
	}
	page++

	file, err := ioutil.TempFile("", "save-to-png")
	if err != nil {
		return fmt.Errorf("fail to create temporary file: %w", err)
	}
	defer os.Remove(file.Name())

	if _, err := io.Copy(file, payload); err != nil {
		return fmt.Errorf("fail to copy payload to temporary file: %w", err)
	}

	// nolint: gosec
	cmdOutput, err := exec.Command("mutool", "pages", file.Name(), strconv.Itoa(int(page))).CombinedOutput()
	if err != nil {
		return fmt.Errorf("fail to execute 'mutool pages': %s: %w", string(cmdOutput), err)
	}

	dpi, err := calculateDPI(string(cmdOutput), width, scale)
	if err != nil {
		return fmt.Errorf("fail to calculate the page dpi: %w", err)
	}

	currentDirectory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("fail to get the executable directory: %w", err)
	}

	outputPath := fmt.Sprintf("%s/%s", currentDirectory, uuid.New().String())
	t1 := time.Now()
	cmdOutput, err = exec.Command(
		"mutool",
		"draw",
		"-L",
		"-o",
		outputPath,
		"-r",
		strconv.FormatFloat(float64(dpi), 'f', 5, 64),
		file.Name(),
		strconv.Itoa(int(page)),
	).CombinedOutput()
	defer os.Remove(outputPath)
	span.SetTag("mutoolDrawExecutionTime", time.Since(t1))
	if err != nil {
		return fmt.Errorf("fail to execute 'mutool draw': %s: %w", string(cmdOutput), err)
	}

	outputPayload, err := os.ReadFile(outputPath)
	if err != nil {
		return fmt.Errorf("fail to read the output result at the file '%s': %w", outputPath, err)
	}

	if _, err = io.Copy(output, bytes.NewBuffer(outputPayload)); err != nil {
		return fmt.Errorf("fail to copy the result into the output: %w", err)
	}
	return nil
}

// PageCount is used to return the page count of the document.
func PageCount(ctx context.Context, payload io.Reader) (int, error) {
	span, _ := ddTracer.StartSpanFromContext(ctx, "lazypdf.PageCount")
	defer span.Finish()

	if payload == nil {
		return 0, errors.New("payload can't be nil")
	}

	file, err := ioutil.TempFile("", "page-count")
	if err != nil {
		return 0, fmt.Errorf("fail to create temporary file: %w", err)
	}
	defer os.Remove(file.Name())

	if _, err := io.Copy(file, payload); err != nil {
		return 0, fmt.Errorf("fail to copy payload to temporary file: %w", err)
	}

	// nolint: gosec
	output, err := exec.Command("mutool", "pages", file.Name()).CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("fail to execute command: %w", err)
	}

	return strings.Count(string(output), "pagenum="), nil
}

func calculateDPI(payload string, width uint16, scale float32) (float32, error) {
	const defaultDPI = 72

	type box struct {
		R string `xml:"r,attr"`
		T string `xml:"t,attr"`
	}

	type page struct {
		XMLName  xml.Name `xml:"page"`
		MediaBox box      `xml:"MediaBox"`
	}

	var p page
	if err := xml.Unmarshal([]byte(payload), &p); err != nil {
		return 0, fmt.Errorf("fail to parse XML: %w", err)
	}

	pageHeight, err := strconv.ParseFloat(p.MediaBox.T, 32)
	if err != nil {
		return 0, fmt.Errorf("fail to parse page height: %w", err)
	}

	pageWidth, err := strconv.ParseFloat(p.MediaBox.R, 32)
	if err != nil {
		return 0, fmt.Errorf("fail to parse page width: %w", err)
	}

	if width != 0 {
		return float32(defaultDPI / pageWidth), nil
	}

	if scale != 0 {
		return (defaultDPI * scale), nil
	}

	if pageHeight > pageWidth {
		return (defaultDPI * 1.5), nil
	}
	return defaultDPI, nil
}
