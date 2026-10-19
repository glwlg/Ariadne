package plugins

import (
	"testing"

	"ariadne/internal/contracts"
)

func TestAirPlayLauncherOnlyOpensToolWindow(t *testing.T) {
	service := NewService()
	for _, query := range []string{
		"airplay", "投屏", "uxplay", "手机投屏", "iphone", "mirror",
		"airplay start", "airplay stop", "airplay status", "airplay logs",
		"airplay firewall", "airplay control", "airplay fps 60",
	} {
		results := service.Search(query)
		if len(results) != 1 || len(results[0].Actions) != 1 {
			t.Fatalf("%q must only offer opening the tool window: %+v", query, results)
		}
		action := results[0].Actions[0]
		if action.ID != "open_tool" || action.Payload["command"] != "open_airplay" {
			t.Fatalf("%q must not control the receiver from the launcher: %+v", query, action)
		}
		if err := contracts.ValidateActionSurfaces(results); err != nil {
			t.Fatalf("%q returned invalid actions: %v", query, err)
		}
	}
	service.ApplyEnabled(map[string]bool{"airplay": false})
	if result := service.Search("投屏")[0]; result.ID != "plugin-disabled-airplay" {
		t.Fatalf("disabled plugin still available: %+v", result)
	}
}
