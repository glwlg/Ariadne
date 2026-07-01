//go:build windows

package filesearch

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"time"

	"golang.org/x/sys/windows/svc"
)

const WindowsServiceName = "AriadneFileSearch"

type windowsIndexerService struct{}

func RunWindowsService(args []string) error {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return err
	}
	if isService {
		return svc.Run(WindowsServiceName, windowsIndexerService{})
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return runSharedIndexer(ctx)
}

func (windowsIndexerService) Execute(args []string, requests <-chan svc.ChangeRequest, statuses chan<- svc.Status) (bool, uint32) {
	statuses <- svc.Status{State: svc.StartPending}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- runSharedIndexer(ctx)
	}()
	statuses <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	for {
		select {
		case request := <-requests:
			switch request.Cmd {
			case svc.Interrogate:
				statuses <- request.CurrentStatus
			case svc.Stop, svc.Shutdown:
				statuses <- svc.Status{State: svc.StopPending}
				cancel()
				<-done
				return false, 0
			default:
				statuses <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
			}
		case err := <-done:
			cancel()
			if err != nil && !errors.Is(err, context.Canceled) {
				return false, 1
			}
			return false, 0
		}
	}
}

func runSharedIndexer(ctx context.Context) error {
	service := NewServiceWithIndexer(newSharedIndexBuilder())
	defer service.Close()
	service.StartIndexing()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			status := service.Status()
			if !status.Indexing && !status.Ready {
				service.StartIndexing()
			}
		}
	}
}
