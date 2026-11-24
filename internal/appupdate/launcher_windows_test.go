//go:build windows

package appupdate

import (
	"reflect"
	"testing"
)

func TestInstallerArgumentsPassCurrentInstallation(t *testing.T) {
	got := installerArguments(`C:\Apps\Fancun\fancun.exe`)
	want := []string{"--update-from", `C:\Apps\Fancun`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("installer args = %#v, want %#v", got, want)
	}
}
