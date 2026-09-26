package scrub

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func TestCleanPNG(t *testing.T) {
	// Create dummy RGBA image
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for x := 0; x < 100; x++ {
		for y := 0; y < 100; y++ {
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("Failed to encode PNG: %v", err)
	}

	cleanBytes, report, err := CleanImage(buf.Bytes(), "test.png")
	if err != nil {
		t.Fatalf("CleanImage failed: %v", err)
	}

	if len(cleanBytes) == 0 {
		t.Fatalf("Expected non-empty cleaned bytes")
	}

	if report.CleanedSize <= 0 {
		t.Fatalf("Expected positive cleaned size, got %d", report.CleanedSize)
	}
}

func TestCleanJPEG(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 80, 80))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatalf("Failed to encode JPEG: %v", err)
	}

	cleanBytes, report, err := CleanImage(buf.Bytes(), "sample.jpg")
	if err != nil {
		t.Fatalf("CleanImage JPEG failed: %v", err)
	}

	if len(cleanBytes) == 0 || report == nil {
		t.Fatalf("Expected valid report and bytes")
	}
}

func TestInspectEmpty(t *testing.T) {
	report, err := InspectMetadata([]byte("invalid-image-data"), "fake.jpg")
	if err != nil {
		t.Fatalf("InspectMetadata should not error on invalid data: %v", err)
	}
	if report.TotalTagsFound != 0 {
		t.Fatalf("Expected 0 tags, got %d", report.TotalTagsFound)
	}
}
