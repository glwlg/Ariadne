//go:build windows

package networkmonitor

import (
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	afINET              = 2
	tcpTableOwnerPIDAll = 5
	tcpStateEstablished = 5
	errorInsufficient   = 122
)

var (
	iphlpapi                = windows.NewLazySystemDLL("iphlpapi.dll")
	procGetExtendedTCPTable = iphlpapi.NewProc("GetExtendedTcpTable")
)

type mibTCPRowOwnerPID struct {
	State      uint32
	LocalAddr  uint32
	LocalPort  uint32
	RemoteAddr uint32
	RemotePort uint32
	PID        uint32
}

func readProcessCounters() ([]processCounter, error) {
	traffic, trafficErr := readPrivilegedTraffic()
	rows, rowsErr := establishedTCPRows()
	byPID := make(map[uint32]*processCounter, len(traffic.Processes))
	processes := make(map[uint32][2]string)
	resolve := func(pid uint32) [2]string {
		if value, ok := processes[pid]; ok {
			return value
		}
		value := processNamePath(pid)
		processes[pid] = value
		return value
	}
	ensure := func(pid uint32) *processCounter {
		if counter := byPID[pid]; counter != nil {
			return counter
		}
		namePath := resolve(pid)
		counter := &processCounter{pid: pid, name: namePath[0], path: namePath[1], connections: []ProcessConnection{}}
		byPID[pid] = counter
		return counter
	}

	for _, process := range traffic.Processes {
		counter := ensure(process.PID)
		counter.bytesSent = process.BytesSent
		counter.bytesReceived = process.BytesReceived
	}
	for _, row := range rows {
		counter := ensure(row.PID)
		counter.connections = append(counter.connections, ProcessConnection{
			LocalAddress:  net.JoinHostPort(ipv4String(row.LocalAddr), fmt.Sprintf("%d", networkPort(row.LocalPort))),
			RemoteAddress: net.JoinHostPort(ipv4String(row.RemoteAddr), fmt.Sprintf("%d", networkPort(row.RemotePort))),
		})
	}

	counters := make([]processCounter, 0, len(byPID))
	for _, counter := range byPID {
		counters = append(counters, *counter)
	}
	sort.Slice(counters, func(i, j int) bool {
		if counters[i].pid != counters[j].pid {
			return counters[i].pid < counters[j].pid
		}
		return counters[i].name < counters[j].name
	})
	if trafficErr != nil {
		return counters, trafficErr
	}
	return counters, rowsErr
}

func establishedTCPRows() ([]mibTCPRowOwnerPID, error) {
	var size uint32
	result, _, _ := procGetExtendedTCPTable.Call(0, uintptr(unsafe.Pointer(&size)), 0, afINET, tcpTableOwnerPIDAll, 0)
	if result != errorInsufficient && result != 0 {
		return nil, syscall.Errno(result)
	}
	if size < 4 {
		return []mibTCPRowOwnerPID{}, nil
	}
	buffer := make([]byte, size)
	result, _, _ = procGetExtendedTCPTable.Call(
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(&size)),
		0,
		afINET,
		tcpTableOwnerPIDAll,
		0,
	)
	if result != 0 {
		return nil, syscall.Errno(result)
	}

	count := *(*uint32)(unsafe.Pointer(&buffer[0]))
	rowSize := int(unsafe.Sizeof(mibTCPRowOwnerPID{}))
	available := (len(buffer) - 4) / rowSize
	if int(count) > available {
		return nil, errors.New("Windows 返回了无效的 TCP 连接表")
	}
	rows := make([]mibTCPRowOwnerPID, 0, count)
	for index := 0; index < int(count); index++ {
		row := *(*mibTCPRowOwnerPID)(unsafe.Pointer(&buffer[4+index*rowSize]))
		if row.State == tcpStateEstablished && row.PID != 0 {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

func processNamePath(pid uint32) [2]string {
	if pid == 0 {
		return [2]string{"系统/其他", ""}
	}
	if pid == 4 {
		return [2]string{"System", ""}
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return [2]string{fmt.Sprintf("PID %d", pid), ""}
	}
	defer windows.CloseHandle(handle)

	buffer := make([]uint16, windows.MAX_PATH*4)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size); err != nil {
		return [2]string{fmt.Sprintf("PID %d", pid), ""}
	}
	path := windows.UTF16ToString(buffer[:size])
	name := strings.TrimSpace(filepath.Base(path))
	if name == "" {
		name = fmt.Sprintf("PID %d", pid)
	}
	return [2]string{name, path}
}

func ipv4String(value uint32) string {
	return net.IPv4(byte(value), byte(value>>8), byte(value>>16), byte(value>>24)).String()
}

func networkPort(value uint32) uint16 {
	port := uint16(value)
	return port>>8 | port<<8
}
