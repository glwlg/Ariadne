//go:build !windows

package workmemory

func defaultWindowContextProvider() func() windowContext {
	return func() windowContext {
		return windowContext{title: "Ariadne", app: "Ariadne"}
	}
}
