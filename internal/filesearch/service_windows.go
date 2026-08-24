//go:build windows

package filesearch

import (
	"ariadne/internal/networkmonitor"
	"context"
	"errors"
	"os"
	"os/signal"
	"reflect"
	"time"

	"golang.org/x/sys/windows/svc"
)

const WindowsServiceName = "AriadneFileSearch"

type windowsIndexerService struct {
	policyProvider PolicyProvider
}

func RunWindowsService(args []string) error {
	return RunWindowsServiceWithPolicy(args, DefaultFileSearchPolicy())
}

func RunWindowsServiceWithPolicy(args []string, policy FileSearchPolicy) error {
	return RunWindowsServiceWithPolicyProvider(args, func() FileSearchPolicy {
		return policy
	})
}

func RunWindowsServiceWithPolicyProvider(args []string, policyProvider PolicyProvider) error {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return err
	}
	if isService {
		return svc.Run(WindowsServiceName, windowsIndexerService{policyProvider: policyProvider})
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return runSharedServices(ctx, policyProvider)
}

func (service windowsIndexerService) Execute(args []string, requests <-chan svc.ChangeRequest, statuses chan<- svc.Status) (bool, uint32) {
	statuses <- svc.Status{State: svc.StartPending}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- runSharedServices(ctx, service.policyProvider)
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

func runSharedServices(ctx context.Context, policyProvider PolicyProvider) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 2)
	go func() { done <- runSharedIndexer(ctx, policyProvider) }()
	go func() { done <- networkmonitor.RunCollector(ctx) }()
	first := <-done
	cancel()
	second := <-done
	for _, err := range []error{first, second} {
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
	}
	return nil
}

func runSharedIndexer(ctx context.Context, policyProvider PolicyProvider) error {
	service := NewServiceWithIndexer(newSharedIndexBuilder())
	defer service.Close()
	if policyProvider == nil {
		policyProvider = func() FileSearchPolicy {
			return DefaultFileSearchPolicy()
		}
	}
	policy := NormalizeFileSearchPolicy(policyProvider())
	service.ApplyPolicy(policy)
	service.StartIndexing()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	pendingRebuild := false
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			nextPolicy := NormalizeFileSearchPolicy(policyProvider())
			if !reflect.DeepEqual(nextPolicy, policy) {
				policy = nextPolicy
				service.ApplyPolicy(policy)
				status := service.Status()
				if status.Indexing {
					pendingRebuild = true
					continue
				}
				pendingRebuild = false
				service.RebuildIndex()
				continue
			}
			status := service.Status()
			if pendingRebuild && !status.Indexing {
				pendingRebuild = false
				service.RebuildIndex()
				continue
			}
			if !status.Indexing && !status.Ready {
				service.StartIndexing()
			}
		}
	}
}
