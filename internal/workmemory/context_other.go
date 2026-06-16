//go:build !windows

package workmemory

import "errors"

func defaultWindowContextProvider() func() windowContext {
	return func() windowContext {
		return windowContext{title: "Ariadne", app: "Ariadne"}
	}
}

func watchForegroundWindow(stop <-chan struct{}, onChange func()) error {
	_ = stop
	_ = onChange
	return errors.New("foreground window events are not supported on this platform")
}
