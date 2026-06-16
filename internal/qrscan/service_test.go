package qrscan

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"path/filepath"
	"testing"

	"ariadne/internal/capturehistory"

	goqrcode "github.com/skip2/go-qrcode"
)

func TestDecodeCaptureReadsQRCodeFromCaptureHistory(t *testing.T) {
	root := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(root, "capture_history.json"), filepath.Join(root, "capture_images"))
	text := "https://ariadne.local/qr-smoke"
	data, err := goqrcode.Encode(text, goqrcode.Medium, 256)
	if err != nil {
		t.Fatal(err)
	}
	status := captures.AddPNG(data, 256, 256, "qr_test", "", []string{"qr"})
	if status.Count != 1 || len(status.Entries) != 1 {
		t.Fatalf("expected capture entry, got %#v", status)
	}

	service := NewService(captures)
	result := service.DecodeCapture(status.Entries[0].ID)

	if !result.OK {
		t.Fatalf("expected qr decode success, got %#v", result)
	}
	if result.Text != text || result.CaptureID != status.Entries[0].ID {
		t.Fatalf("unexpected decode result: %#v", result)
	}
	if result.Source != "capture_history" || result.Format == "" || result.ImagePath == "" {
		t.Fatalf("decode result should include evidence fields: %#v", result)
	}
	if service.LastResult().Text != text {
		t.Fatalf("last result not recorded: %#v", service.LastResult())
	}
}

func TestDecodeCaptureReportsMissingQRCode(t *testing.T) {
	root := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(root, "capture_history.json"), filepath.Join(root, "capture_images"))
	data := blankPNG(t, 96, 96)
	status := captures.AddPNG(data, 96, 96, "blank", "", nil)

	result := NewService(captures).DecodeCapture(status.Entries[0].ID)

	if result.OK {
		t.Fatalf("blank image should not decode: %#v", result)
	}
	if result.Error == "" || result.CaptureID != status.Entries[0].ID {
		t.Fatalf("failure should include error and capture id: %#v", result)
	}
}

func TestDecodeCaptureValidatesMissingCapture(t *testing.T) {
	result := NewService(capturehistory.NewServiceWithPaths("", "")).DecodeCapture("missing")

	if result.OK || result.Error == "" {
		t.Fatalf("missing capture should fail clearly: %#v", result)
	}
}

func blankPNG(t *testing.T, width int, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.White)
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
