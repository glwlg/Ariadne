package imagepreview

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"math"
)

const DefaultMaxSide = 512

func CreatePNGThumbnail(pngBytes []byte, maxSide int) ([]byte, int, int, bool, error) {
	if len(pngBytes) == 0 {
		return nil, 0, 0, false, fmt.Errorf("empty png")
	}
	src, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		return nil, 0, 0, false, err
	}
	return CreatePNGThumbnailFromImage(src, maxSide)
}

func CreatePNGThumbnailFromImage(src image.Image, maxSide int) ([]byte, int, int, bool, error) {
	if src == nil {
		return nil, 0, 0, false, fmt.Errorf("nil image")
	}
	if maxSide <= 0 {
		maxSide = DefaultMaxSide
	}
	bounds := src.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, 0, 0, false, fmt.Errorf("invalid image size")
	}
	if width <= maxSide && height <= maxSide {
		return nil, 0, 0, false, nil
	}

	scale := float64(maxSide) / float64(max(width, height))
	thumbWidth := max(1, int(math.Round(float64(width)*scale)))
	thumbHeight := max(1, int(math.Round(float64(height)*scale)))
	dst := image.NewNRGBA(image.Rect(0, 0, thumbWidth, thumbHeight))

	srcX := make([]int, thumbWidth)
	for x := 0; x < thumbWidth; x++ {
		srcX[x] = min(width-1, int(float64(x)*float64(width)/float64(thumbWidth)))
	}

	switch typed := src.(type) {
	case *image.NRGBA:
		for y := 0; y < thumbHeight; y++ {
			sourceY := bounds.Min.Y + min(height-1, int(float64(y)*float64(height)/float64(thumbHeight)))
			srcRow := typed.Pix[(sourceY-typed.Rect.Min.Y)*typed.Stride:]
			dstRow := dst.Pix[y*dst.Stride:]
			for x := 0; x < thumbWidth; x++ {
				srcOff := (bounds.Min.X + srcX[x] - typed.Rect.Min.X) * 4
				dstOff := x * 4
				dstRow[dstOff] = srcRow[srcOff]
				dstRow[dstOff+1] = srcRow[srcOff+1]
				dstRow[dstOff+2] = srcRow[srcOff+2]
				dstRow[dstOff+3] = srcRow[srcOff+3]
			}
		}
	case *image.RGBA:
		for y := 0; y < thumbHeight; y++ {
			sourceY := bounds.Min.Y + min(height-1, int(float64(y)*float64(height)/float64(thumbHeight)))
			srcRow := typed.Pix[(sourceY-typed.Rect.Min.Y)*typed.Stride:]
			dstRow := dst.Pix[y*dst.Stride:]
			for x := 0; x < thumbWidth; x++ {
				srcOff := (bounds.Min.X + srcX[x] - typed.Rect.Min.X) * 4
				dstOff := x * 4
				a := srcRow[srcOff+3]
				switch a {
				case 0:
					dstRow[dstOff] = 0
					dstRow[dstOff+1] = 0
					dstRow[dstOff+2] = 0
					dstRow[dstOff+3] = 0
				case 255:
					dstRow[dstOff] = srcRow[srcOff]
					dstRow[dstOff+1] = srcRow[srcOff+1]
					dstRow[dstOff+2] = srcRow[srcOff+2]
					dstRow[dstOff+3] = 255
				default:
					a32 := uint32(a)
					dstRow[dstOff] = uint8(((uint32(srcRow[srcOff]) * 0xffff) / a32) >> 8)
					dstRow[dstOff+1] = uint8(((uint32(srcRow[srcOff+1]) * 0xffff) / a32) >> 8)
					dstRow[dstOff+2] = uint8(((uint32(srcRow[srcOff+2]) * 0xffff) / a32) >> 8)
					dstRow[dstOff+3] = a
				}
			}
		}
	default:
		for y := 0; y < thumbHeight; y++ {
			sourceY := bounds.Min.Y + min(height-1, int(float64(y)*float64(height)/float64(thumbHeight)))
			dstRow := dst.Pix[y*dst.Stride:]
			for x := 0; x < thumbWidth; x++ {
				r, g, b, a := src.At(bounds.Min.X+srcX[x], sourceY).RGBA()
				dstOff := x * 4
				if a == 0 {
					dstRow[dstOff] = 0
					dstRow[dstOff+1] = 0
					dstRow[dstOff+2] = 0
					dstRow[dstOff+3] = 0
					continue
				}
				if a == 0xffff {
					dstRow[dstOff] = uint8(r >> 8)
					dstRow[dstOff+1] = uint8(g >> 8)
					dstRow[dstOff+2] = uint8(b >> 8)
					dstRow[dstOff+3] = 255
					continue
				}
				dstRow[dstOff] = uint8(((r * 0xffff) / a) >> 8)
				dstRow[dstOff+1] = uint8(((g * 0xffff) / a) >> 8)
				dstRow[dstOff+2] = uint8(((b * 0xffff) / a) >> 8)
				dstRow[dstOff+3] = uint8(a >> 8)
			}
		}
	}

	var out bytes.Buffer
	if err := png.Encode(&out, dst); err != nil {
		return nil, 0, 0, false, err
	}
	return out.Bytes(), thumbWidth, thumbHeight, true, nil
}
