package networkmonitor

import (
	"context"
	"testing"
)

type fakeTelemetryStream struct {
	ctx     context.Context
	cancel  context.CancelFunc
	request TelemetrySubscription
	frames  []TelemetryFrame
}

func newFakeTelemetryStream(request TelemetrySubscription) *fakeTelemetryStream {
	ctx, cancel := context.WithCancel(context.Background())
	return &fakeTelemetryStream{ctx: ctx, cancel: cancel, request: request}
}

func (f *fakeTelemetryStream) Context() context.Context {
	return f.ctx
}

func (f *fakeTelemetryStream) ReceiveJSON(value any) error {
	subscription := value.(*TelemetrySubscription)
	*subscription = f.request
	return nil
}

func (f *fakeTelemetryStream) SendJSON(value any) error {
	frame := value.(TelemetryFrame)
	f.frames = append(f.frames, frame)
	f.cancel()
	return nil
}

func TestTelemetryStreamSendsTrafficSnapshotImmediately(t *testing.T) {
	service := NewServiceWithReader(func() ([]interfaceCounter, error) {
		return []interfaceCounter{counterFixture(1024, 4096)}, nil
	})
	stream := newFakeTelemetryStream(TelemetrySubscription{Mode: TelemetryModeTraffic})

	service.serveTelemetryStream(stream)

	if len(stream.frames) != 1 {
		t.Fatalf("expected one immediate frame, got %#v", stream.frames)
	}
	frame := stream.frames[0]
	if frame.Traffic == nil || frame.Traffic.BytesSent != 1024 || frame.Processes != nil {
		t.Fatalf("expected traffic-only telemetry frame, got %#v", frame)
	}
}

func TestTelemetryStreamSendsProcessSnapshotOnSameEndpoint(t *testing.T) {
	service := NewServiceWithReader(func() ([]interfaceCounter, error) { return nil, nil })
	service.processReader = func() ([]processCounter, error) {
		return []processCounter{{pid: 7, name: "browser.exe"}}, nil
	}
	stream := newFakeTelemetryStream(TelemetrySubscription{Mode: TelemetryModeProcesses})

	service.serveTelemetryStream(stream)

	if len(stream.frames) != 1 {
		t.Fatalf("expected one immediate frame, got %#v", stream.frames)
	}
	frame := stream.frames[0]
	if frame.Processes == nil || frame.Processes.ProcessCount != 1 || frame.Traffic != nil {
		t.Fatalf("expected process-only telemetry frame, got %#v", frame)
	}
}
