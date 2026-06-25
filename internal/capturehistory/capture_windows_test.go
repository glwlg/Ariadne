//go:build windows

package capturehistory

import "testing"

func BenchmarkCaptureScreenPNG(b *testing.B) {
	bounds := VirtualScreenBounds()
	if bounds.Width <= 0 || bounds.Height <= 0 {
		b.Skip("virtual screen is unavailable")
	}
	monitors := MonitorBounds()

	b.Run("default_virtual_screen", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			data, capturedBounds, err := CaptureScreenPNG()
			if err != nil {
				b.Fatal(err)
			}
			if len(data) == 0 || capturedBounds.Width <= 0 || capturedBounds.Height <= 0 {
				b.Fatalf("expected populated capture, bytes=%d bounds=%#v", len(data), capturedBounds)
			}
		}
	})

	b.Run("fast_virtual_screen", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			data, capturedBounds, err := CaptureScreenPNGFast()
			if err != nil {
				b.Fatal(err)
			}
			if len(data) == 0 || capturedBounds.Width <= 0 || capturedBounds.Height <= 0 {
				b.Fatalf("expected populated capture, bytes=%d bounds=%#v", len(data), capturedBounds)
			}
		}
	})

	b.Run("fast_per_monitor", func(b *testing.B) {
		if len(monitors) == 0 {
			b.Skip("monitor bounds are unavailable")
		}
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			for _, monitor := range monitors {
				data, width, height, err := CaptureRegionPNGFast(monitor.X, monitor.Y, monitor.Width, monitor.Height)
				if err != nil {
					b.Fatal(err)
				}
				if len(data) == 0 || width <= 0 || height <= 0 {
					b.Fatalf("expected populated monitor capture, bytes=%d size=%dx%d", len(data), width, height)
				}
			}
		}
	})
}
