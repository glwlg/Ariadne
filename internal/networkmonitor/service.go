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

type Service struct {
	mu       sync.Mutex
	reader   counterReader
	now      func() time.Time
	previous map[uint32]counterPoint
	seenAt   time.Time
}

func NewService() *Service {
	return NewServiceWithReader(readInterfaceCounters)
}

func NewServiceWithReader(reader counterReader) *Service {
	if reader == nil {
		reader = func() ([]interfaceCounter, error) { return nil, nil }
	}
	return &Service{
		reader:   reader,
		now:      time.Now,
		previous: map[uint32]counterPoint{},
	}
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

func bytesPerSecond(current uint64, previous uint64, elapsedSeconds float64) float64 {
	if current < previous || elapsedSeconds <= 0 {
		return 0
	}
	return float64(current-previous) / elapsedSeconds
}
