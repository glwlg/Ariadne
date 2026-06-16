package imagepreview

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
)

const DefaultMaxSide = 512

func CreatePNGThumbnail(pngBytes []byte, maxSide int) ([]byte, int, int, bool, error) {
	if len(pngBytes) == 0 {
		return nil, 0, 0, false, fmt.Errorf("empty png")
	}
	if maxSide <= 0 {
		maxSide = DefaultMaxSide
	}
	src, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		return nil, 0, 0, false, err
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

	for y := 0; y < thumbHeight; y++ {
		sourceY := bounds.Min.Y + min(height-1, int(float64(y)*float64(height)/float64(thumbHeight)))
		for x := 0; x < thumbWidth; x++ {
			sourceX := bounds.Min.X + min(width-1, int(float64(x)*float64(width)/float64(thumbWidth)))
			dst.SetNRGBA(x, y, color.NRGBAModel.Convert(src.At(sourceX, sourceY)).(color.NRGBA))
		}
	}

	var out bytes.Buffer
	if err := png.Encode(&out, dst); err != nil {
		return nil, 0, 0, false, err
	}
	return out.Bytes(), thumbWidth, thumbHeight, true, nil
}
