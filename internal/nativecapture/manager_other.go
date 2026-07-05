//go:build !windows

package nativecapture

import (
	"context"
	"errors"
)

type Options struct {
	ExePath  string
	PipeName string
}

type Request struct {
	Command             string `json:"command"`
	AutoPin             bool   `json:"autoPin"`
	DirectClipboardCopy bool   `json:"directClipboardCopy"`
	CallbackPipeName    string `json:"callbackPipeName,omitempty"`
}

type AnnotationPoint struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type AnnotationOperation struct {
	Kind        string            `json:"kind"`
	X           int               `json:"x"`
	Y           int               `json:"y"`
	Width       int               `json:"width,omitempty"`
	Height      int               `json:"height,omitempty"`
	EndX        int               `json:"endX,omitempty"`
	EndY        int               `json:"endY,omitempty"`
	Color       string            `json:"color,omitempty"`
	StrokeWidth int               `json:"strokeWidth,omitempty"`
	PixelSize   int               `json:"pixelSize,omitempty"`
	Points      []AnnotationPoint `json:"points,omitempty"`
	Text        string            `json:"text,omitempty"`
	FontSize    int               `json:"fontSize,omitempty"`
	Number      int               `json:"number,omitempty"`
}

type Response struct {
	OK               bool                  `json:"ok"`
	Canceled         bool                  `json:"canceled"`
	Message          string                `json:"message"`
	Action           string                `json:"action"`
	PNGBase64        string                `json:"pngBase64"`
	X                int                   `json:"x"`
	Y                int                   `json:"y"`
	Width            int                   `json:"width"`
	Height           int                   `json:"height"`
	SavedPath        string                `json:"savedPath"`
	ClipboardWritten bool                  `json:"clipboardWritten"`
	Pinned           bool                  `json:"pinned"`
	PinPositioned    bool                  `json:"pinPositioned"`
	PinX             int                   `json:"pinX"`
	PinY             int                   `json:"pinY"`
	NativePinID      string                `json:"nativePinId"`
	RenderMS         int64                 `json:"renderMs"`
	ClipboardMS      int64                 `json:"clipboardMs"`
	TotalMS          int64                 `json:"totalMs"`
	Operations       []AnnotationOperation `json:"operations,omitempty"`
}

type PinActionRequest struct {
	Action      string `json:"action"`
	NativePinID string `json:"nativePinId"`
	ImagePath   string `json:"imagePath"`
}

type PinActionResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	Text    string `json:"text,omitempty"`
}

type PinActionHandler func(PinActionRequest) PinActionResponse

type Manager struct{}

func NewManager(options Options) *Manager {
	return &Manager{}
}

func (m *Manager) Start() error {
	return errors.New("native capture host is only available on Windows")
}

func (m *Manager) SetPinActionHandler(handler PinActionHandler) {}

func (m *Manager) Capture(ctx context.Context, request Request) (Response, error) {
	return Response{}, errors.New("native capture host is only available on Windows")
}

func (m *Manager) Stop() error {
	return nil
}
