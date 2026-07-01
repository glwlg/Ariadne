//go:build !windows

package setupstub

func RunInteractive(payload []byte, options Options) (Result, error) {
	return Run(payload, options)
}
