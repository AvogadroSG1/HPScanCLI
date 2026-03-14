package output

import (
	"bytes"
	"context"
	"fmt"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"time"
)

// FormatExtension maps document format names to file extensions.
var FormatExtension = map[string]string{
	"jpeg": ".jpg",
	"pdf":  ".pdf",
	"png":  ".png",
}

// FormatMIME maps document format names to MIME types.
var FormatMIME = map[string]string{
	"jpeg": "image/jpeg",
	"pdf":  "application/pdf",
	"png":  "image/png",
}

// SaveFile writes data to a file in dir, returning the final path.
// If baseName is empty a timestamped name is generated. Collisions are
// resolved by appending _1, _2, etc.
func SaveFile(_ context.Context, data []byte, baseName string, format string, dir string) (string, error) {
	if baseName == "" {
		baseName = time.Now().Format("SCAN_20060102_150405")
	}

	ext, ok := FormatExtension[format]
	if !ok {
		ext = "." + format
	}

	path := filepath.Join(dir, baseName+ext)

	for i := 1; fileExists(path); i++ {
		path = filepath.Join(dir, fmt.Sprintf("%s_%d%s", baseName, i, ext))
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}

	return path, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ConvertToPNG decodes JPEG data and re-encodes it as PNG.
func ConvertToPNG(jpegData []byte) ([]byte, error) {
	img, err := jpeg.Decode(bytes.NewReader(jpegData))
	if err != nil {
		return nil, fmt.Errorf("decoding jpeg: %w", err)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encoding png: %w", err)
	}

	return buf.Bytes(), nil
}
