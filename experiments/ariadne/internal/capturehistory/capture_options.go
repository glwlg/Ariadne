package capturehistory

import (
	"fmt"
	"strings"
)

func normalizeCaptureOptions(options CaptureOptions) CaptureOptions {
	options.CaptureScope = strings.TrimSpace(options.CaptureScope)
	options.MultiMonitor = strings.TrimSpace(options.MultiMonitor)
	switch options.CaptureScope {
	case "active_window", "primary_screen":
	default:
		options.CaptureScope = "all_screens"
	}
	switch options.MultiMonitor {
	case "per_monitor", "primary_only":
	default:
		options.MultiMonitor = "combined"
	}
	return options
}

func captureBoundsForOptions(options CaptureOptions) ([]ScreenBounds, error) {
	options = normalizeCaptureOptions(options)
	switch {
	case options.CaptureScope == "active_window":
		bounds, err := activeWindowBounds()
		if err != nil {
			return nil, err
		}
		return []ScreenBounds{bounds}, nil
	case options.CaptureScope == "primary_screen" || options.MultiMonitor == "primary_only":
		bounds := primaryScreenBounds()
		if bounds.Width <= 0 || bounds.Height <= 0 {
			return nil, fmt.Errorf("无法读取主屏幕尺寸")
		}
		return []ScreenBounds{bounds}, nil
	case options.MultiMonitor == "per_monitor":
		bounds := monitorBounds()
		if len(bounds) == 0 {
			return nil, fmt.Errorf("无法枚举显示器")
		}
		return bounds, nil
	default:
		bounds := VirtualScreenBounds()
		if bounds.Width <= 0 || bounds.Height <= 0 {
			return nil, fmt.Errorf("无法读取虚拟屏幕尺寸")
		}
		return []ScreenBounds{bounds}, nil
	}
}

func captureActionTags(options CaptureOptions, index int, total int) []string {
	options = normalizeCaptureOptions(options)
	actions := []string{options.CaptureScope, options.MultiMonitor}
	if total > 1 {
		actions = append(actions, fmt.Sprintf("monitor_%d", index+1))
	}
	return actions
}

func captureMetadataTags(options CaptureOptions, bounds ScreenBounds, index int, total int) []string {
	options = normalizeCaptureOptions(options)
	tags := []string{
		"范围:" + captureScopeLabel(options.CaptureScope),
		"多屏:" + multiMonitorLabel(options.MultiMonitor),
		fmt.Sprintf("区域:%d,%d,%dx%d", bounds.X, bounds.Y, bounds.Width, bounds.Height),
	}
	if total > 1 {
		tags = append(tags, fmt.Sprintf("显示器:%d/%d", index+1, total))
	}
	return tags
}

func captureScopeLabel(value string) string {
	switch normalizeCaptureOptions(CaptureOptions{CaptureScope: value}).CaptureScope {
	case "active_window":
		return "前台窗口"
	case "primary_screen":
		return "主屏幕"
	default:
		return "全部屏幕"
	}
}

func multiMonitorLabel(value string) string {
	switch normalizeCaptureOptions(CaptureOptions{MultiMonitor: value}).MultiMonitor {
	case "per_monitor":
		return "按屏幕分条"
	case "primary_only":
		return "仅主屏"
	default:
		return "合并截图"
	}
}

func rectToBounds(rect winRect) ScreenBounds {
	return ScreenBounds{
		X:      int(rect.Left),
		Y:      int(rect.Top),
		Width:  int(rect.Right - rect.Left),
		Height: int(rect.Bottom - rect.Top),
	}
}

func intersectBounds(bounds ScreenBounds, screen ScreenBounds) (ScreenBounds, error) {
	left := maxInt(bounds.X, screen.X)
	top := maxInt(bounds.Y, screen.Y)
	right := minInt(bounds.X+bounds.Width, screen.X+screen.Width)
	bottom := minInt(bounds.Y+bounds.Height, screen.Y+screen.Height)
	if right <= left || bottom <= top {
		return ScreenBounds{}, fmt.Errorf("截图区域不在虚拟屏幕内")
	}
	return ScreenBounds{X: left, Y: top, Width: right - left, Height: bottom - top}, nil
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
