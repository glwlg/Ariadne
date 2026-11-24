package imagepreview

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestCreatePNGThumbnailFromImageSupportsNRGBAAndRGBAAndSkipsSmallImages(t *testing.T) {
	small := image.NewRGBA(image.Rect(0, 0, 64, 32))
	if _, _, _, ok, err := CreatePNGThumbnailFromImage(small, DefaultMaxSide); err != nil || ok {
		t.Fatalf("small image should skip thumbnailing, ok=%t err=%v", ok, err)
	}

	nrgba := image.NewNRGBA(image.Rect(10, 20, 1010, 520))
	nrgba.SetNRGBA(10, 20, color.NRGBA{R: 12, G: 34, B: 56, A: 200})
	nrgba.SetNRGBA(1009, 519, color.NRGBA{R: 200, G: 100, B: 50, A: 255})
	thumbBytes, w, h, ok, err := CreatePNGThumbnailFromImage(nrgba, 200)
	if err != nil || !ok || w != 200 || h != 100 {
		t.Fatalf("unexpected NRGBA thumbnail result: ok=%t size=%dx%d err=%v", ok, w, h, err)
	}
	decoded, err := png.Decode(bytes.NewReader(thumbBytes))
	if err != nil {
		t.Fatalf("decode NRGBA thumbnail: %v", err)
	}
	if got := color.NRGBAModel.Convert(decoded.At(0, 0)).(color.NRGBA); got != (color.NRGBA{R: 12, G: 34, B: 56, A: 200}) {
		t.Fatalf("unexpected NRGBA pixel: %#v", got)
	}

	rgba := image.NewRGBA(image.Rect(0, 0, 1000, 500))
	rgba.SetRGBA(0, 0, color.RGBA{R: 50, G: 100, B: 128, A: 128})
	rgba.SetRGBA(2, 2, color.RGBA{R: 0, G: 0, B: 0, A: 0})
	rgba.SetRGBA(4, 4, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	thumbRGBA, w2, h2, ok, err := CreatePNGThumbnailFromImage(rgba, 200)
	if err != nil || !ok || w2 != 200 || h2 != 100 {
		t.Fatalf("unexpected RGBA thumbnail result: ok=%t size=%dx%d err=%v", ok, w2, h2, err)
	}
	decodedRGBA, err := png.Decode(bytes.NewReader(thumbRGBA))
	if err != nil {
		t.Fatalf("decode RGBA thumbnail: %v", err)
	}
	wantPremul := color.NRGBAModel.Convert(color.RGBA{R: 50, G: 100, B: 128, A: 128}).(color.NRGBA)
	if got := color.NRGBAModel.Convert(decodedRGBA.At(0, 0)).(color.NRGBA); got != wantPremul {
		t.Fatalf("unexpected premultiplied RGBA pixel: got=%#v want=%#v", got, wantPremul)
	}
}

func TestCreatePNGThumbnailMatchesCreatePNGThumbnailFromImage(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 800, 400))
	for y := 0; y < 400; y++ {
		for x := 0; x < 800; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: 180, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	fromBytes, w1, h1, ok1, err1 := CreatePNGThumbnail(buf.Bytes(), 200)
	fromImage, w2, h2, ok2, err2 := CreatePNGThumbnailFromImage(img, 200)
	if err1 != nil || err2 != nil || !ok1 || !ok2 || w1 != w2 || h1 != h2 {
		t.Fatalf("mismatch: bytes=(%d,%d,%t,%v) image=(%d,%d,%t,%v)", w1, h1, ok1, err1, w2, h2, ok2, err2)
	}
	if !bytes.Equal(fromBytes, fromImage) {
		t.Fatalf("expected identical PNG bytes between byte and in-memory thumbnail paths")
	}
}
