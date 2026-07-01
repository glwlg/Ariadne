//go:build windows

package platform

import (
	"errors"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
)

const fileSearchWindowsServiceName = "AriadneFileSearch"

func enrichFileSearchServiceStatus(status FileSearchStatus) FileSearchStatus {
	status.ServiceName = fileSearchWindowsServiceName
	manager, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return status
	}
	defer windows.CloseServiceHandle(manager)

	serviceName, err := windows.UTF16PtrFromString(fileSearchWindowsServiceName)
	if err != nil {
		return status
	}
	service, err := windows.OpenService(manager, serviceName, windows.SERVICE_QUERY_STATUS)
	if err != nil {
		if !errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			status.ServiceState = "unknown"
		}
		return status
	}
	defer windows.CloseServiceHandle(service)
	status.ServiceInstalled = true

	var serviceStatus windows.SERVICE_STATUS
	if err := windows.QueryServiceStatus(service, &serviceStatus); err != nil {
		status.ServiceState = "unknown"
		return status
	}
	state := svc.State(serviceStatus.CurrentState)
	status.ServiceState = serviceStateLabel(state)
	status.ServiceRunning = state == svc.Running || state == svc.StartPending
	return status
}

func serviceStateLabel(state svc.State) string {
	switch state {
	case svc.Stopped:
		return "stopped"
	case svc.StartPending:
		return "start_pending"
	case svc.StopPending:
		return "stop_pending"
	case svc.Running:
		return "running"
	case svc.ContinuePending:
		return "continue_pending"
	case svc.PausePending:
		return "pause_pending"
	case svc.Paused:
		return "paused"
	default:
		return "unknown"
	}
}
