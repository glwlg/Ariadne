package launcherwindow

import (
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestReservedRelativePositionUsesExpandedLauncherHeight(t *testing.T) {
	screen := &application.Screen{WorkArea: application.Rect{Width: 1920, Height: 1040}}

	x, y, ok := ReservedRelativePosition(screen)

	if !ok {
		t.Fatal("expected reserved launcher position")
	}
	if x != 530 || y != 286 {
		t.Fatalf("unexpected reserved position: %d,%d", x, y)
	}
}

func TestReservedRelativePositionClampsSmallScreens(t *testing.T) {
	screen := &application.Screen{WorkArea: application.Rect{Width: 700, Height: 400}}

	x, y, ok := ReservedRelativePosition(screen)

	if !ok {
		t.Fatal("expected reserved launcher position")
	}
	if x != 0 || y != 0 {
		t.Fatalf("small work area should clamp to origin, got %d,%d", x, y)
	}
}

func TestSizeKeepsCollapsedAndExpandedWidthsAligned(t *testing.T) {
	collapsedWidth, collapsedHeight := Size(false)
	expandedWidth, expandedHeight := Size(true)

	if collapsedWidth != Width || expandedWidth != Width {
		t.Fatalf("launcher widths should stay aligned, got collapsed=%d expanded=%d", collapsedWidth, expandedWidth)
	}
	if collapsedHeight != CollapsedHeight || expandedHeight != ExpandedHeight {
		t.Fatalf("unexpected launcher heights: collapsed=%d expanded=%d", collapsedHeight, expandedHeight)
	}
}
