package shell

import (
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestSecondInstanceLaunchDoesNotOpenWindow(t *testing.T) {
	opened := []string{}
	manager := NewManager("", "", "", func(view string) bool {
		opened = append(opened, view)
		return true
	}, nil, nil)

	options := manager.SingleInstanceOptions()
	if options.OnSecondInstanceLaunch == nil {
		t.Fatal("expected second instance callback")
	}

	options.OnSecondInstanceLaunch(application.SecondInstanceData{Args: []string{"ariadne.exe"}})
	if len(opened) > 0 {
		t.Fatalf("second instance launch should stay in background, opened %v", opened)
	}
}

func TestOpenWorkMemoryUsesToolOpenerWithoutStartupWindow(t *testing.T) {
	opened := []string{}
	manager := NewManager("", "", "", func(view string) bool {
		opened = append(opened, view)
		return true
	}, nil, nil)

	manager.OpenWorkMemory()

	if len(opened) != 1 || opened[0] != "work-memory" {
		t.Fatalf("work memory should open through tool opener, got %v", opened)
	}
}
