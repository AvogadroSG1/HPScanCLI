package output

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveFileAutoName(t *testing.T) {
	dir := t.TempDir()
	data := []byte("fake-jpeg-data")

	path, err := SaveFile(context.Background(), data, "", "jpeg", dir)
	if err != nil {
		t.Fatalf("SaveFile returned error: %v", err)
	}

	base := filepath.Base(path)
	if !strings.HasPrefix(base, "SCAN_") {
		t.Errorf("expected SCAN_ prefix, got %s", base)
	}

	if !strings.HasSuffix(base, ".jpg") {
		t.Errorf("expected .jpg suffix, got %s", base)
	}
}

func TestSaveFileCollisionAvoidance(t *testing.T) {
	dir := t.TempDir()

	// Create an existing file to force a collision.
	existing := filepath.Join(dir, "myscan.jpg")
	if err := os.WriteFile(existing, []byte("first"), 0644); err != nil {
		t.Fatal(err)
	}

	path, err := SaveFile(context.Background(), []byte("second"), "myscan", "jpeg", dir)
	if err != nil {
		t.Fatalf("SaveFile returned error: %v", err)
	}

	if filepath.Base(path) != "myscan_1.jpg" {
		t.Errorf("expected myscan_1.jpg, got %s", filepath.Base(path))
	}
}

func TestConvertToPNG(t *testing.T) {
	// Create a minimal 2x2 JPEG image.
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	img.Set(1, 0, color.RGBA{G: 255, A: 255})
	img.Set(0, 1, color.RGBA{B: 255, A: 255})
	img.Set(1, 1, color.RGBA{R: 128, G: 128, B: 128, A: 255})

	var jpegBuf bytes.Buffer
	if err := jpeg.Encode(&jpegBuf, img, nil); err != nil {
		t.Fatalf("failed to create test JPEG: %v", err)
	}

	pngData, err := ConvertToPNG(jpegBuf.Bytes())
	if err != nil {
		t.Fatalf("ConvertToPNG returned error: %v", err)
	}

	// Verify PNG magic bytes: \x89PNG
	if len(pngData) < 4 {
		t.Fatal("PNG data too short")
	}

	magic := pngData[:4]
	expected := []byte{0x89, 0x50, 0x4E, 0x47}
	if !bytes.Equal(magic, expected) {
		t.Errorf("expected PNG magic bytes %x, got %x", expected, magic)
	}
}
