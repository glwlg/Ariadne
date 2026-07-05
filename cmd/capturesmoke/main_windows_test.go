//go:build windows

package main

import (
	"image"
	"image/color"
	"strings"
	"testing"
	"time"
)

func TestEvaluateContentMatchStrict(t *testing.T) {
	ok, detail := evaluateContentMatch(98.25, 1.9)
	if !ok {
		t.Fatalf("expected strict comparison to pass: %s", detail)
	}
	if !strings.Contains(detail, "mode=strict") {
		t.Fatalf("expected strict mode in detail, got %q", detail)
	}
}

func TestEvaluateContentMatchLowMeanTolerance(t *testing.T) {
	ok, detail := evaluateContentMatch(96.54, 0.312)
	if !ok {
		t.Fatalf("expected low-mean native capture variance to pass: %s", detail)
	}
	if !strings.Contains(detail, "mode=low_mean_tolerance") {
		t.Fatalf("expected low_mean_tolerance mode in detail, got %q", detail)
	}
}

func TestEvaluateContentMatchRejectsLowExactMatch(t *testing.T) {
	ok, detail := evaluateContentMatch(94.99, 0.05)
	if ok {
		t.Fatalf("expected low exact match to fail despite low mean diff: %s", detail)
	}
	if !strings.Contains(detail, "mode=failed") {
		t.Fatalf("expected failed mode in detail, got %q", detail)
	}
}

func TestEvaluateContentMatchRejectsHighMeanDiff(t *testing.T) {
	ok, detail := evaluateContentMatch(96.54, 2.5)
	if ok {
		t.Fatalf("expected high mean diff to fail despite tolerant exact match: %s", detail)
	}
}

func TestCompareImagesLowMeanToleranceScenario(t *testing.T) {
	reference := solidImage(100, 100, color.RGBA{R: 80, G: 90, B: 100, A: 255})
	captured := solidImage(100, 100, color.RGBA{R: 80, G: 90, B: 100, A: 255})
	for i := 0; i < 346; i++ {
		x := i % 100
		y := i / 100
		captured.SetRGBA(x, y, color.RGBA{R: 89, G: 99, B: 109, A: 255})
	}

	matchPercent, meanDiff := compareImages(reference, captured)
	ok, detail := evaluateContentMatch(matchPercent, meanDiff)
	if !ok {
		t.Fatalf("expected low-mean image variance to pass: %s", detail)
	}
	if matchPercent < 96.53 || matchPercent > 96.55 {
		t.Fatalf("unexpected match percent %.4f", matchPercent)
	}
	if meanDiff < 0.310 || meanDiff > 0.313 {
		t.Fatalf("unexpected mean diff %.4f", meanDiff)
	}
}

func TestCaptureDimensionsAcceptsDPIPhysicalPixels(t *testing.T) {
	ok, detail, scale := captureDimensionsOK(390, 270, 260, 180)
	if !ok {
		t.Fatalf("expected DPI physical pixel dimensions to pass: %s", detail)
	}
	if scale < 1.49 || scale > 1.51 {
		t.Fatalf("expected 1.5 scale, got %.4f", scale)
	}
	if !strings.Contains(detail, "mode=physical_pixels") {
		t.Fatalf("expected physical pixel mode, got %q", detail)
	}
}

func TestNormalizeCapturedForComparisonResizesDPIImage(t *testing.T) {
	reference := solidImage(2, 2, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	captured := solidImage(3, 3, color.RGBA{R: 10, G: 20, B: 30, A: 255})

	normalized := normalizeCapturedForComparison(reference, captured, 1.5)
	if normalized.Bounds().Dx() != 2 || normalized.Bounds().Dy() != 2 {
		t.Fatalf("expected normalized 2x2 image, got %dx%d", normalized.Bounds().Dx(), normalized.Bounds().Dy())
	}
	matchPercent, meanDiff := compareImages(reference, normalized)
	if matchPercent != 100 || meanDiff != 0 {
		t.Fatalf("expected normalized image to compare exactly, got match=%.2f mean=%.4f", matchPercent, meanDiff)
	}
}

func TestPhysicalDPIContentToleranceThreshold(t *testing.T) {
	matchPercent := 71.17
	meanDiff := 7.179
	ok, _ := evaluateContentMatch(matchPercent, meanDiff)
	if ok {
		t.Fatal("strict content match should not accept this physical-DPI sample")
	}
	if !(matchPercent >= contentPhysicalMatchPercent && meanDiff <= contentPhysicalMeanAbsDiff) {
		t.Fatal("expected physical-DPI tolerance to accept this sample")
	}
}

func TestDragPinnedWindowRetriesAfterZeroDelta(t *testing.T) {
	initial := windowSample{Handle: 1, ProcessID: 42, Title: "截图贴图 260x180", X: 160, Y: 140, Width: 260, Height: 180}
	moved := initial
	moved.X += pinnedDragMoveX
	moved.Y += pinnedDragMoveY
	samples := []windowSample{initial, initial, initial, moved}
	readIndex := 0
	dragCalls := 0
	sleeps := []time.Duration{}

	after, attempts := dragPinnedWindow(initial, pinnedDragRuntime{
		Sleep: func(duration time.Duration) {
			sleeps = append(sleeps, duration)
		},
		DragMouse: func(x1 int, y1 int, x2 int, y2 int) {
			dragCalls++
			if x2-x1 != pinnedDragMoveX || y2-y1 != pinnedDragMoveY {
				t.Fatalf("unexpected drag vector %d,%d", x2-x1, y2-y1)
			}
		},
		ReadWindow: func(window windowSample) windowSample {
			if readIndex >= len(samples) {
				return samples[len(samples)-1]
			}
			next := samples[readIndex]
			readIndex++
			return next
		},
	})

	if dragCalls != 2 {
		t.Fatalf("expected retry after zero delta, got %d drag call(s)", dragCalls)
	}
	if len(attempts) != 2 {
		t.Fatalf("expected 2 attempts, got %#v", attempts)
	}
	if attempts[0].DeltaX != 0 || attempts[0].DeltaY != 0 {
		t.Fatalf("expected first attempt to preserve zero delta, got %#v", attempts[0])
	}
	if attempts[1].DeltaX != pinnedDragMoveX || attempts[1].DeltaY != pinnedDragMoveY {
		t.Fatalf("expected second attempt to record movement, got %#v", attempts[1])
	}
	if after.X != moved.X || after.Y != moved.Y {
		t.Fatalf("expected final moved sample, got %#v", after)
	}
	if len(sleeps) != 4 || sleeps[0] != pinnedDragReadyDelay || sleeps[2] != pinnedDragRetryDelay {
		t.Fatalf("unexpected sleep sequence %#v", sleeps)
	}
	detail := pinnedDragDetail(attempts)
	if !strings.Contains(detail, "attempts=2") || !strings.Contains(detail, "delta=90,55") || !strings.Contains(detail, "attempt_deltas=1:0,0;2:90,55") {
		t.Fatalf("unexpected detail %q", detail)
	}
}

func TestDragPinnedWindowStillFailsAfterRetryWithoutMovement(t *testing.T) {
	initial := windowSample{Handle: 1, ProcessID: 42, Title: "截图贴图 260x180", X: 160, Y: 140, Width: 260, Height: 180}
	dragCalls := 0

	after, attempts := dragPinnedWindow(initial, pinnedDragRuntime{
		Sleep: func(time.Duration) {},
		DragMouse: func(int, int, int, int) {
			dragCalls++
		},
		ReadWindow: func(windowSample) windowSample {
			return initial
		},
	})

	if dragCalls != 2 {
		t.Fatalf("expected exactly one retry, got %d drag call(s)", dragCalls)
	}
	if len(attempts) != 2 {
		t.Fatalf("expected 2 failed attempts, got %#v", attempts)
	}
	if pinnedDragMoved(after.X-initial.X, after.Y-initial.Y) {
		t.Fatalf("expected unmoved window to remain a failed drag")
	}
	detail := pinnedDragDetail(attempts)
	if !strings.Contains(detail, "attempts=2") || !strings.Contains(detail, "delta=0,0") || !strings.Contains(detail, "attempt_deltas=1:0,0;2:0,0") {
		t.Fatalf("unexpected detail %q", detail)
	}
}

func solidImage(width int, height int, value color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, value)
		}
	}
	return img
}
