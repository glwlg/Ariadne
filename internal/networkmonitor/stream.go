package networkmonitor

import (
	"context"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	TelemetryStreamName    = "network-telemetry"
	TelemetryModeTraffic   = "traffic"
	TelemetryModeProcesses = "processes"
	telemetryInterval      = time.Second
)

type TelemetrySubscription struct {
	Mode string `json:"mode"`
}

type TelemetryFrame struct {
	Traffic   *TrafficSnapshot        `json:"traffic,omitempty"`
	Processes *ProcessTrafficSnapshot `json:"processes,omitempty"`
}

type telemetryStream interface {
	Context() context.Context
	ReceiveJSON(value any) error
	SendJSON(value any) error
}

func HandleTelemetryStream(service *Service, connection *application.StreamConn) {
	if service != nil {
		service.serveTelemetryStream(connection)
	}
}

func (s *Service) serveTelemetryStream(connection telemetryStream) {
	if connection == nil {
		return
	}
	var subscription TelemetrySubscription
	if err := connection.ReceiveJSON(&subscription); err != nil {
		return
	}
	mode := strings.ToLower(strings.TrimSpace(subscription.Mode))
	if mode == "" {
		mode = TelemetryModeTraffic
	}
	if mode != TelemetryModeTraffic && mode != TelemetryModeProcesses {
		return
	}

	send := func() error {
		frame := TelemetryFrame{}
		if mode == TelemetryModeProcesses {
			snapshot := s.ProcessSnapshot()
			frame.Processes = &snapshot
		} else {
			snapshot := s.Snapshot()
			frame.Traffic = &snapshot
		}
		return connection.SendJSON(frame)
	}
	if err := send(); err != nil {
		return
	}

	ticker := time.NewTicker(telemetryInterval)
	defer ticker.Stop()
	for {
		select {
		case <-connection.Context().Done():
			return
		case <-ticker.C:
			if err := send(); err != nil {
				return
			}
		}
	}
}
