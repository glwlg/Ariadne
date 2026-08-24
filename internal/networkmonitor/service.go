package networkmonitor

import (
	"sort"
	"sync"
	"time"
)

type AdapterTraffic struct {
	Name                   string  `json:"name"`
	Alias                  string  `json:"alias"`
	Description            string  `json:"description"`
	InterfaceIndex         uint32  `json:"interfaceIndex"`
	Operational            bool    `json:"operational"`
	TransmitLinkBitsPerSec uint64  `json:"transmitLinkBitsPerSec"`
	ReceiveLinkBitsPerSec  uint64  `json:"receiveLinkBitsPerSec"`
	BytesSent              uint64  `json:"bytesSent"`
	BytesReceived          uint64  `json:"bytesReceived"`
	UploadBytesPerSecond   float64 `json:"uploadBytesPerSecond"`
	DownloadBytesPerSecond float64 `json:"downloadBytesPerSecond"`
}

type TrafficSnapshot struct {
	TimestampUnix          int64            `json:"timestampUnix"`
	AdapterCount           int              `json:"adapterCount"`
	ActiveAdapterCount     int              `json:"activeAdapterCount"`
	BytesSent              uint64           `json:"bytesSent"`
	BytesReceived          uint64           `json:"bytesReceived"`
	UploadBytesPerSecond   float64          `json:"uploadBytesPerSecond"`
	DownloadBytesPerSecond float64          `json:"downloadBytesPerSecond"`
	Adapters               []AdapterTraffic `json:"adapters"`
	LastError              string           `json:"lastError,omitempty"`
}

type ProcessConnection struct {
	LocalAddress           string  `json:"localAddress"`
	RemoteAddress          string  `json:"remoteAddress"`
	UploadBytesPerSecond   float64 `json:"uploadBytesPerSecond"`
	DownloadBytesPerSecond float64 `json:"downloadBytesPerSecond"`
	BytesSent              uint64  `json:"bytesSent"`
	BytesReceived          uint64  `json:"bytesReceived"`
}

type ProcessTraffic struct {
	PID                    uint32              `json:"pid"`
	Name                   string              `json:"name"`
	Path                   string              `json:"path,omitempty"`
	IconURL                string              `json:"iconUrl,omitempty"`
	UploadBytesPerSecond   float64             `json:"uploadBytesPerSecond"`
	DownloadBytesPerSecond float64             `json:"downloadBytesPerSecond"`
	BytesSent              uint64              `json:"bytesSent"`
	BytesReceived          uint64              `json:"bytesReceived"`
	Connections            []ProcessConnection `json:"connections"`
}

type ProcessTrafficSnapshot struct {
	TimestampUnix          int64            `json:"timestampUnix"`
	ProcessCount           int              `json:"processCount"`
	ConnectionCount        int              `json:"connectionCount"`
	UploadBytesPerSecond   float64          `json:"uploadBytesPerSecond"`
	DownloadBytesPerSecond float64          `json:"downloadBytesPerSecond"`
	Processes              []ProcessTraffic `json:"processes"`
	LastError              string           `json:"lastError,omitempty"`
}

type interfaceCounter struct {
	name                   string
	alias                  string
	description            string
	interfaceIndex         uint32
	operational            bool
	transmitLinkBitsPerSec uint64
	receiveLinkBitsPerSec  uint64
	bytesSent              uint64
	bytesReceived          uint64
}

type counterReader func() ([]interfaceCounter, error)

type counterPoint struct {
	bytesSent     uint64
	bytesReceived uint64
}

type processCounter struct {
	pid                    uint32
	name                   string
	path                   string
	uploadBytesPerSecond   float64
	downloadBytesPerSecond float64
	bytesSent              uint64
	bytesReceived          uint64
	connections            []ProcessConnection
}

type processCounterReader func() ([]processCounter, error)

type Service struct {
	mu       sync.Mutex
	reader   counterReader
	now      func() time.Time
	previous map[uint32]counterPoint
	seenAt   time.Time

	processReader        processCounterReader
	processAdapters      map[uint32]counterPoint
	processAdapterSeenAt time.Time
	processPrevious      map[uint32]counterPoint
	processSeenAt        time.Time
	processIcon          func(string) string
}

func NewService() *Service {
	return NewServiceWithReader(readInterfaceCounters)
}

func NewServiceWithReader(reader counterReader) *Service {
	if reader == nil {
		reader = func() ([]interfaceCounter, error) { return nil, nil }
	}
	return &Service{
		reader:          reader,
		now:             time.Now,
		previous:        map[uint32]counterPoint{},
		processReader:   readProcessCounters,
		processAdapters: map[uint32]counterPoint{},
		processPrevious: map[uint32]counterPoint{},
	}
}

func (s *Service) SetProcessIconResolver(resolve func(string) string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.processIcon = resolve
}

func (s *Service) Snapshot() TrafficSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	counters, err := s.reader()
	sort.SliceStable(counters, func(i, j int) bool {
		if counters[i].operational != counters[j].operational {
			return counters[i].operational
		}
		return counters[i].name < counters[j].name
	})

	elapsedSeconds := 0.0
	if !s.seenAt.IsZero() {
		elapsedSeconds = now.Sub(s.seenAt).Seconds()
	}

	nextPrevious := make(map[uint32]counterPoint, len(counters))
	snapshot := TrafficSnapshot{
		TimestampUnix: now.Unix(),
		AdapterCount:  len(counters),
		Adapters:      make([]AdapterTraffic, 0, len(counters)),
	}
	if err != nil {
		snapshot.LastError = err.Error()
	}

	for _, counter := range counters {
		uploadRate := 0.0
		downloadRate := 0.0
		if previous, ok := s.previous[counter.interfaceIndex]; ok && elapsedSeconds > 0 {
			uploadRate = bytesPerSecond(counter.bytesSent, previous.bytesSent, elapsedSeconds)
			downloadRate = bytesPerSecond(counter.bytesReceived, previous.bytesReceived, elapsedSeconds)
		}

		if counter.operational {
			snapshot.ActiveAdapterCount++
		}
		snapshot.BytesSent += counter.bytesSent
		snapshot.BytesReceived += counter.bytesReceived
		snapshot.UploadBytesPerSecond += uploadRate
		snapshot.DownloadBytesPerSecond += downloadRate
		snapshot.Adapters = append(snapshot.Adapters, AdapterTraffic{
			Name:                   counter.name,
			Alias:                  counter.alias,
			Description:            counter.description,
			InterfaceIndex:         counter.interfaceIndex,
			Operational:            counter.operational,
			TransmitLinkBitsPerSec: counter.transmitLinkBitsPerSec,
			ReceiveLinkBitsPerSec:  counter.receiveLinkBitsPerSec,
			BytesSent:              counter.bytesSent,
			BytesReceived:          counter.bytesReceived,
			UploadBytesPerSecond:   uploadRate,
			DownloadBytesPerSecond: downloadRate,
		})
		nextPrevious[counter.interfaceIndex] = counterPoint{
			bytesSent:     counter.bytesSent,
			bytesReceived: counter.bytesReceived,
		}
	}

	s.previous = nextPrevious
	s.seenAt = now
	return snapshot
}

func (s *Service) ProcessSnapshot() ProcessTrafficSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	counters, err := s.processReader()
	elapsedSeconds := 0.0
	if !s.processSeenAt.IsZero() {
		elapsedSeconds = now.Sub(s.processSeenAt).Seconds()
	}
	nextPrevious := make(map[uint32]counterPoint, len(counters))
	for index := range counters {
		counter := &counters[index]
		if previous, ok := s.processPrevious[counter.pid]; ok && elapsedSeconds > 0 {
			counter.uploadBytesPerSecond = bytesPerSecond(counter.bytesSent, previous.bytesSent, elapsedSeconds)
			counter.downloadBytesPerSecond = bytesPerSecond(counter.bytesReceived, previous.bytesReceived, elapsedSeconds)
		}
		nextPrevious[counter.pid] = counterPoint{bytesSent: counter.bytesSent, bytesReceived: counter.bytesReceived}
	}
	s.processPrevious = nextPrevious
	s.processSeenAt = now
	snapshot := ProcessTrafficSnapshot{TimestampUnix: now.Unix()}
	if err != nil {
		snapshot.LastError = err.Error()
	}

	for _, counter := range counters {
		process := ProcessTraffic{
			PID:                    counter.pid,
			Name:                   counter.name,
			Path:                   counter.path,
			UploadBytesPerSecond:   counter.uploadBytesPerSecond,
			DownloadBytesPerSecond: counter.downloadBytesPerSecond,
			BytesSent:              counter.bytesSent,
			BytesReceived:          counter.bytesReceived,
			Connections:            counter.connections,
		}
		if s.processIcon != nil && process.Path != "" {
			process.IconURL = s.processIcon(process.Path)
		}
		sort.SliceStable(process.Connections, func(i, j int) bool {
			left := process.Connections[i].DownloadBytesPerSecond + process.Connections[i].UploadBytesPerSecond
			right := process.Connections[j].DownloadBytesPerSecond + process.Connections[j].UploadBytesPerSecond
			return left > right
		})
		snapshot.UploadBytesPerSecond += process.UploadBytesPerSecond
		snapshot.DownloadBytesPerSecond += process.DownloadBytesPerSecond
		snapshot.ConnectionCount += len(process.Connections)
		snapshot.Processes = append(snapshot.Processes, process)
	}

	adapterUpload, adapterDownload, adapterErr := s.processAdapterRates(now)
	if adapterErr == nil {
		attributedUpload := snapshot.UploadBytesPerSecond
		attributedDownload := snapshot.DownloadBytesPerSecond
		snapshot.UploadBytesPerSecond = adapterUpload
		snapshot.DownloadBytesPerSecond = adapterDownload
		residualUpload := max(0, adapterUpload-attributedUpload)
		residualDownload := max(0, adapterDownload-attributedDownload)
		if residualUpload > 0 || residualDownload > 0 {
			merged := false
			for index := range snapshot.Processes {
				if snapshot.Processes[index].PID == 0 {
					snapshot.Processes[index].UploadBytesPerSecond += residualUpload
					snapshot.Processes[index].DownloadBytesPerSecond += residualDownload
					merged = true
					break
				}
			}
			if !merged {
				snapshot.Processes = append(snapshot.Processes, ProcessTraffic{
					Name:                   "系统/其他",
					UploadBytesPerSecond:   residualUpload,
					DownloadBytesPerSecond: residualDownload,
					Connections:            []ProcessConnection{},
				})
			}
		}
	} else if snapshot.LastError == "" {
		snapshot.LastError = adapterErr.Error()
	}
	sort.SliceStable(snapshot.Processes, func(i, j int) bool {
		left := snapshot.Processes[i].DownloadBytesPerSecond + snapshot.Processes[i].UploadBytesPerSecond
		right := snapshot.Processes[j].DownloadBytesPerSecond + snapshot.Processes[j].UploadBytesPerSecond
		if left != right {
			return left > right
		}
		return snapshot.Processes[i].Name < snapshot.Processes[j].Name
	})
	snapshot.ProcessCount = len(snapshot.Processes)
	return snapshot
}

func (s *Service) processAdapterRates(now time.Time) (float64, float64, error) {
	counters, err := s.reader()
	elapsedSeconds := 0.0
	if !s.processAdapterSeenAt.IsZero() {
		elapsedSeconds = now.Sub(s.processAdapterSeenAt).Seconds()
	}
	next := make(map[uint32]counterPoint, len(counters))
	upload := 0.0
	download := 0.0
	for _, counter := range counters {
		if previous, ok := s.processAdapters[counter.interfaceIndex]; ok && elapsedSeconds > 0 {
			upload += bytesPerSecond(counter.bytesSent, previous.bytesSent, elapsedSeconds)
			download += bytesPerSecond(counter.bytesReceived, previous.bytesReceived, elapsedSeconds)
		}
		next[counter.interfaceIndex] = counterPoint{bytesSent: counter.bytesSent, bytesReceived: counter.bytesReceived}
	}
	s.processAdapters = next
	s.processAdapterSeenAt = now
	return upload, download, err
}

func bytesPerSecond(current uint64, previous uint64, elapsedSeconds float64) float64 {
	if current < previous || elapsedSeconds <= 0 {
		return 0
	}
	return float64(current-previous) / elapsedSeconds
}
