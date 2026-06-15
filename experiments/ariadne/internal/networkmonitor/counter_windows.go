//go:build windows

package networkmonitor

import (
	"errors"
	"fmt"
	"net"

	"golang.org/x/sys/windows"
)

const ifOperStatusUp = 1

func readInterfaceCounters() ([]interfaceCounter, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	counters := make([]interfaceCounter, 0, len(interfaces))
	var firstErr error
	for _, item := range interfaces {
		if item.Index <= 0 || item.Flags&net.FlagLoopback != 0 || item.Flags&net.FlagUp == 0 {
			continue
		}

		row := windows.MibIfRow2{InterfaceIndex: uint32(item.Index)}
		if err := windows.GetIfEntry2Ex(windows.MibIfEntryNormal, &row); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("read interface %s: %w", item.Name, err)
			}
			continue
		}

		alias := windows.UTF16ToString(row.Alias[:])
		description := windows.UTF16ToString(row.Description[:])
		counters = append(counters, interfaceCounter{
			name:                   item.Name,
			alias:                  alias,
			description:            description,
			interfaceIndex:         row.InterfaceIndex,
			operational:            row.OperStatus == ifOperStatusUp,
			transmitLinkBitsPerSec: row.TransmitLinkSpeed,
			receiveLinkBitsPerSec:  row.ReceiveLinkSpeed,
			bytesSent:              row.OutOctets,
			bytesReceived:          row.InOctets,
		})
	}

	if len(counters) == 0 && firstErr != nil {
		return counters, firstErr
	}
	if len(counters) == 0 {
		return counters, errors.New("no active non-loopback network interfaces")
	}
	return counters, nil
}
