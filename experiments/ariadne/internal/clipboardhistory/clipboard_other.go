//go:build !windows

package clipboardhistory

func readSystemClipboardText() (string, error) {
	return "", errClipboardUnsupported
}

func readSystemClipboardEntry(imageDir string, source string) (Entry, error) {
	return Entry{}, errClipboardUnsupported
}

func writeImageToSystemClipboard(path string) error {
	return errClipboardUnsupported
}
