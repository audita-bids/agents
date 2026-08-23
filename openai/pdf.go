package openai

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/ledongthuc/pdf"
)

var (
	ErrUnsupportedPDF = errors.New("unsupported pdf")
	ErrEmptyPDF       = errors.New("pdf has no extractable text")
)

func ExtractText(r io.Reader) (_ string, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("%w: %v", ErrUnsupportedPDF, rec)
		}
	}()

	data, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("read: %w", err)
	}

	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("pdf: %w", err)
	}

	var text strings.Builder
	for i := 1; i <= reader.NumPage(); i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		content := page.Content()
		for _, t := range content.Text {
			text.WriteString(t.S)
		}
		text.WriteString("\n")
	}

	out := text.String()

	if strings.TrimSpace(out) == "" {
		return "", ErrEmptyPDF
	}

	return out, nil
}
