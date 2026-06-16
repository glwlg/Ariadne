package qrscan

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"
	"sync"
	"time"

	"ariadne/internal/capturehistory"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

type CaptureProvider interface {
	Entry(id string) capturehistory.Entry
	CaptureScreen(source string) capturehistory.Status
}

type Result struct {
	OK        bool   `json:"ok"`
	Text      string `json:"text,omitempty"`
	Format    string `json:"format,omitempty"`
	Source    string `json:"source,omitempty"`
	CaptureID string `json:"captureId,omitempty"`
	ImagePath string `json:"imagePath,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	Error     string `json:"error,omitempty"`
	DecodedAt int64  `json:"decodedAt,omitempty"`
}

type Service struct {
	mu       sync.RWMutex
	captures CaptureProvider
	last     Result
}

func NewService(captures CaptureProvider) *Service {
	return &Service{captures: captures}
}

func (s *Service) LastResult() Result {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.last
}

func (s *Service) DecodeCapture(captureID string) Result {
	captureID = strings.TrimSpace(captureID)
	if captureID == "" {
		return s.record(Result{OK: false, Error: "缺少截图记录 ID", DecodedAt: time.Now().Unix()})
	}
	if s.captures == nil {
		return s.record(Result{OK: false, CaptureID: captureID, Error: "截图历史服务不可用", DecodedAt: time.Now().Unix()})
	}
	entry := s.captures.Entry(captureID)
	if entry.ID == "" || entry.ImagePath == "" {
		return s.record(Result{OK: false, CaptureID: captureID, Error: "未找到截图记录", DecodedAt: time.Now().Unix()})
	}
	result := decodeImagePath(entry.ImagePath)
	result.Source = "capture_history"
	result.CaptureID = entry.ID
	result.ImagePath = entry.ImagePath
	result.Width = entry.Width
	result.Height = entry.Height
	return s.record(result)
}

func (s *Service) DecodeCurrentScreen() Result {
	if s.captures == nil {
		return s.record(Result{OK: false, Error: "截图历史服务不可用", DecodedAt: time.Now().Unix()})
	}
	status := s.captures.CaptureScreen("qr_scan")
	if status.LastCaptureError != "" {
		return s.record(Result{OK: false, Source: "current_screen", Error: status.LastCaptureError, DecodedAt: time.Now().Unix()})
	}
	var newest capturehistory.Entry
	for _, entry := range status.Entries {
		if entry.Source != "qr_scan" {
			continue
		}
		if newest.ID == "" || entry.CreatedAt > newest.CreatedAt {
			newest = entry
		}
	}
	if newest.ID == "" {
		return s.record(Result{OK: false, Source: "current_screen", Error: "截图服务未返回二维码识别记录", DecodedAt: time.Now().Unix()})
	}
	result := decodeImagePath(newest.ImagePath)
	result.Source = "current_screen"
	result.CaptureID = newest.ID
	result.ImagePath = newest.ImagePath
	result.Width = newest.Width
	result.Height = newest.Height
	return s.record(result)
}

func DecodeImagePath(path string) Result {
	return decodeImagePath(path)
}

func (s *Service) record(result Result) Result {
	if result.DecodedAt == 0 {
		result.DecodedAt = time.Now().Unix()
	}
	if result.OK && result.Format == "" {
		result.Format = "QR_CODE"
	}
	if !result.OK && result.Error == "" {
		result.Error = "未识别到二维码"
	}
	s.mu.Lock()
	s.last = result
	s.mu.Unlock()
	return result
}

func decodeImagePath(path string) Result {
	path = strings.TrimSpace(path)
	if path == "" {
		return Result{OK: false, Error: "缺少图片路径", DecodedAt: time.Now().Unix()}
	}
	file, err := os.Open(path)
	if err != nil {
		return Result{OK: false, ImagePath: path, Error: err.Error(), DecodedAt: time.Now().Unix()}
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return Result{OK: false, ImagePath: path, Error: "图片解码失败: " + err.Error(), DecodedAt: time.Now().Unix()}
	}
	bitmap, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return Result{OK: false, ImagePath: path, Error: err.Error(), DecodedAt: time.Now().Unix()}
	}
	decoded, err := qrcode.NewQRCodeReader().Decode(bitmap, nil)
	if err != nil {
		return Result{OK: false, ImagePath: path, Error: "未识别到二维码", DecodedAt: time.Now().Unix()}
	}
	text := strings.TrimSpace(decoded.GetText())
	if text == "" {
		return Result{OK: false, ImagePath: path, Error: "二维码内容为空", DecodedAt: time.Now().Unix()}
	}
	return Result{
		OK:        true,
		Text:      text,
		Format:    fmt.Sprint(decoded.GetBarcodeFormat()),
		ImagePath: path,
		DecodedAt: time.Now().Unix(),
	}
}
