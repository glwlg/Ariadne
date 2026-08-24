//go:build windows

package networkmonitor

import (
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestInstalledCollectorAttributesLiveTraffic(t *testing.T) {
	if os.Getenv("ARIADNE_NETWORK_INTEGRATION") != "1" {
		t.Skip("set ARIADNE_NETWORK_INTEGRATION=1 with the Ariadne background service installed")
	}
	pid := uint32(os.Getpid())
	baseline, err := readPrivilegedTraffic()
	if err != nil {
		t.Fatal(err)
	}
	baselineSent, baselineReceived := collectorTotals(baseline, pid)
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}, Timeout: 15 * time.Second}
	defer client.CloseIdleConnections()
	var last privilegedTrafficSnapshot
	attributed := false
	for attempt := 0; attempt < 5; attempt++ {
		downloadTraffic(t, client)
		time.Sleep(time.Second)
		last, err = readPrivilegedTraffic()
		if err != nil {
			t.Fatal(err)
		}
		sent, received := collectorTotals(last, pid)
		if sent > baselineSent && received > baselineReceived {
			attributed = true
			break
		}
	}
	if !attributed {
		t.Fatalf("live process %d missing from collector snapshots: %#v", pid, last)
	}

	service := NewService()
	if first := service.ProcessSnapshot(); first.LastError != "" {
		t.Fatal(first.LastError)
	}
	for attempt := 0; attempt < 5; attempt++ {
		downloadTraffic(t, client)
		time.Sleep(time.Second)
		snapshot := service.ProcessSnapshot()
		if snapshot.LastError != "" {
			t.Fatal(snapshot.LastError)
		}
		for _, process := range snapshot.Processes {
			if process.PID == pid && process.UploadBytesPerSecond > 0 && process.DownloadBytesPerSecond > 0 && snapshot.DownloadBytesPerSecond > 0 {
				return
			}
		}
	}
	t.Fatalf("ProcessSnapshot kept live process %d at zero", pid)
}

func downloadTraffic(t *testing.T, client *http.Client) {
	t.Helper()
	response, err := client.Get("https://speed.cloudflare.com/__down?bytes=131072")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if _, err := io.Copy(io.Discard, response.Body); err != nil {
		t.Fatal(err)
	}
}

func collectorTotals(snapshot privilegedTrafficSnapshot, pid uint32) (uint64, uint64) {
	for _, process := range snapshot.Processes {
		if process.PID == pid {
			return process.BytesSent, process.BytesReceived
		}
	}
	return 0, 0
}
