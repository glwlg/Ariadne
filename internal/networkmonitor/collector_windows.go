//go:build windows

package networkmonitor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/tekert/goetw/etw"
)

const (
	networkCollectorPipe = `\\.\pipe\AriadneNetworkCollector.v1`
	networkCollectorSDDL = `D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GRGW;;;AU)`
)

type collectorRequest struct {
	Version int    `json:"version"`
	Command string `json:"command"`
}

type kernelNetworkCollector struct {
	accumulator *trafficAccumulator
	session     *etw.RealTimeSession
	consumer    *etw.Consumer
}

func RunCollector(ctx context.Context) error {
	collector, err := startKernelNetworkCollector(ctx)
	if err != nil {
		return err
	}
	defer collector.stop()

	listener, err := winio.ListenPipe(networkCollectorPipe, &winio.PipeConfig{
		SecurityDescriptor: networkCollectorSDDL,
		InputBufferSize:    1024,
		OutputBufferSize:   64 * 1024,
	})
	if err != nil {
		return fmt.Errorf("启动进程网络通信: %w", err)
	}
	defer listener.Close()
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			if ctx.Err() != nil || errors.Is(acceptErr, net.ErrClosed) {
				return nil
			}
			return acceptErr
		}
		go collector.handle(connection)
	}
}

func startKernelNetworkCollector(ctx context.Context) (*kernelNetworkCollector, error) {
	collector := &kernelNetworkCollector{accumulator: newTrafficAccumulator()}
	collector.session = etw.NewRealTimeSession("AriadneNetworkMonitor")
	provider := etw.Provider{
		GUID:            *etw.MustParseGUID("{7DD42A49-5329-4832-8DFD-43D979153A88}"),
		Name:            "Microsoft-Windows-Kernel-Network",
		EnableLevel:     4,
		MatchAnyKeyword: 0x30,
	}
	if err := collector.session.EnableProvider(provider); err != nil {
		_ = collector.session.Stop()
		return nil, fmt.Errorf("启动 Windows 进程网络跟踪: %w", err)
	}
	collector.consumer = etw.NewConsumer(ctx).FromSessions(collector.session)
	collector.consumer.EventCallback = func(event *etw.EventRecordHelper) error {
		collector.consume(event)
		return nil
	}
	if err := collector.consumer.Start(); err != nil {
		_ = collector.session.Stop()
		return nil, fmt.Errorf("读取 Windows 进程网络事件: %w", err)
	}
	return collector, nil
}

func (c *kernelNetworkCollector) consume(event *etw.EventRecordHelper) {
	if event == nil || event.EventRec == nil {
		return
	}
	eventID := event.EventRec.EventHeader.EventDescriptor.Id
	if networkEventDirection(eventID) == 0 {
		return
	}
	size, ok := firstEventUint(event, "size", "Size", "TransferSize")
	if !ok || size == 0 {
		return
	}
	pid, ok := firstEventUint(event, "PID", "Pid", "pid", "ProcessId")
	if !ok {
		pid = uint64(event.EventRec.EventHeader.ProcessId)
	}
	if pid > uint64(^uint32(0)) {
		return
	}
	c.accumulator.record(eventID, uint32(pid), size)
}

func firstEventUint(event *etw.EventRecordHelper, names ...string) (uint64, bool) {
	for _, name := range names {
		value, err := event.GetPropertyUint(name)
		if err == nil {
			return value, true
		}
	}
	return 0, false
}

func (c *kernelNetworkCollector) handle(connection net.Conn) {
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(2 * time.Second))
	var request collectorRequest
	if err := json.NewDecoder(io.LimitReader(connection, 1024)).Decode(&request); err != nil {
		_ = json.NewEncoder(connection).Encode(privilegedTrafficSnapshot{LastError: "无效的进程网络请求"})
		return
	}
	if request.Version != 1 || request.Command != "snapshot" {
		_ = json.NewEncoder(connection).Encode(privilegedTrafficSnapshot{LastError: "不支持的进程网络请求"})
		return
	}
	if err := c.session.Flush(); err != nil {
		_ = json.NewEncoder(connection).Encode(privilegedTrafficSnapshot{LastError: err.Error()})
		return
	}
	snapshot := c.accumulator.snapshot(time.Now())
	if err := c.consumer.LastError(); err != nil {
		snapshot.LastError = err.Error()
	}
	sort.Slice(snapshot.Processes, func(i, j int) bool { return snapshot.Processes[i].PID < snapshot.Processes[j].PID })
	_ = json.NewEncoder(connection).Encode(snapshot)
}

func (c *kernelNetworkCollector) stop() {
	if c.consumer != nil {
		_ = c.consumer.Stop()
	}
	if c.session != nil {
		_ = c.session.Stop()
	}
}

func readPrivilegedTraffic() (privilegedTrafficSnapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
	defer cancel()
	connection, err := winio.DialPipeContext(ctx, networkCollectorPipe)
	if err != nil {
		return privilegedTrafficSnapshot{}, fmt.Errorf("进程网络后台服务未运行: %w", err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(time.Second))
	if err := json.NewEncoder(connection).Encode(collectorRequest{Version: 1, Command: "snapshot"}); err != nil {
		return privilegedTrafficSnapshot{}, err
	}
	var snapshot privilegedTrafficSnapshot
	if err := json.NewDecoder(io.LimitReader(connection, 4*1024*1024)).Decode(&snapshot); err != nil {
		return privilegedTrafficSnapshot{}, err
	}
	if snapshot.LastError != "" {
		return snapshot, errors.New(snapshot.LastError)
	}
	return snapshot, nil
}
