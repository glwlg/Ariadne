//go:build !windows

package setupstub

import (
	"fmt"
	"os"
)

func ShowInfo(title string, message string) {
	fmt.Fprintf(os.Stdout, "%s\n%s\n", title, message)
}

func ShowError(title string, message string) {
	fmt.Fprintf(os.Stderr, "%s\n%s\n", title, message)
}
