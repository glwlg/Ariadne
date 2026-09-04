//go:build windows

package appupdate

import (
	"reflect"
	"testing"
)

func TestInstallerArgumentsPassCurrentInstallation(t *testing.T) {
	got := installerArguments(`C:\Apps\Ariadne\ariadne.exe`)
	want := []string{"--update-from", `C:\Apps\Ariadne`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("installer args = %#v, want %#v", got, want)
	}
}
