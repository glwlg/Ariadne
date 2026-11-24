package search

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"fancun/internal/contracts"
)

type providerFunc func(string) []contracts.SearchResult

func (f providerFunc) Search(query string) []contracts.SearchResult { return f(query) }

func TestProvidersRunConcurrentlyWithDeterministicDeduplication(t *testing.T) {
	release := make(chan struct{})
	secondStarted := make(chan struct{})
	first := providerFunc(func(string) []contracts.SearchResult {
		<-release
		return []contracts.SearchResult{scoredResult("duplicate", "First provider", 10), scoredResult("first", "First", 10)}
	})
	second := providerFunc(func(string) []contracts.SearchResult {
		close(secondStarted)
		return []contracts.SearchResult{scoredResult("duplicate", "Second provider", 100), scoredResult("second", "Second", 10)}
	})
	service := NewServiceWithState(NewStateStore(""), first, second)
	done := make(chan contracts.SearchResponse, 1)
	go func() { done <- service.Search(context.Background(), "provider-order-test") }()
	select {
	case <-secondStarted:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("second provider was blocked by first provider")
	}
	close(release)
	response := <-done
	if len(response.Results) != 3 || response.Results[0].Title != "First provider" || response.Results[1].ID != "first" || response.Results[2].ID != "second" {
		t.Fatalf("provider completion changed deduplication or score ties: %#v", response.Results)
	}
}

func TestCancellationReturnsWithoutWaitingForLegacyProviderAndBoundsCalls(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	started := make(chan struct{})
	var calls atomic.Int32
	provider := providerFunc(func(string) []contracts.SearchResult {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-release
		return nil
	})
	service := NewServiceWithState(NewStateStore(""), provider)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { service.Search(ctx, "old"); close(done) }()
	<-started
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cancellation waited for a legacy provider")
	}
	// New queries may wait for a busy provider, but never start another uncancellable call.
	for i := 0; i < 20; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		service.Search(ctx, "new")
		cancel()
	}
	if calls.Load() != 1 {
		t.Fatalf("legacy provider calls grew with cancelled requests: %d", calls.Load())
	}
}

func TestProgressiveSearchPublishesFastResultsBeforeSlowProviderCompletes(t *testing.T) {
	release := make(chan struct{})
	fastResult := make(chan Progress, 4)
	service := NewServiceWithState(NewStateStore(""),
		providerFunc(func(string) []contracts.SearchResult {
			<-release
			return []contracts.SearchResult{scoredResult("slow", "Slow", 20)}
		}),
		providerFunc(func(string) []contracts.SearchResult {
			return []contracts.SearchResult{scoredResult("fast", "Fast", 10)}
		}),
	)
	RegisterProgressHandler(service, func(progress Progress) {
		if len(progress.Response.Results) > 0 {
			fastResult <- progress
		}
	})
	done := make(chan contracts.SearchResponse, 1)
	go func() { done <- service.SearchProgressively(context.Background(), "progress-test", "request-1") }()
	select {
	case progress := <-fastResult:
		if progress.RequestID != "request-1" || len(progress.Response.Results) != 1 || progress.Response.Results[0].ID != "fast" {
			t.Fatalf("wrong partial result: %#v", progress)
		}
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("fast results waited for slow provider")
	}
	select {
	case <-done:
		t.Fatal("search finished before slow provider returned")
	default:
	}
	close(release)
	response := <-done
	if len(response.Results) != 2 || response.Results[0].ID != "slow" {
		t.Fatalf("wrong final result: %#v", response)
	}
	if service.PerformanceStatus().SampleCount != 1 {
		t.Fatal("partial results must not add performance samples")
	}
}

func TestUsageDecorationDoesNotMutateProviderSlices(t *testing.T) {
	tags := []string{"base", "untouched"}
	actions := []contracts.PreviewAction{{ID: "open"}, {ID: "untouched"}}
	result := scoredResult("favorite", "Favorite", 10)
	result.Tags, result.Actions = tags[:1], actions[:1]
	state := NewStateStore("")
	state.SetFavorite("favorite", true)
	service := NewServiceWithState(state, fakeProvider{results: []contracts.SearchResult{result}})
	response := service.Search(context.Background(), "decoration-test")
	if len(response.Results[0].Tags) != 2 || len(response.Results[0].Actions) != 2 {
		t.Fatal("missing usage decorations")
	}
	if tags[1] != "untouched" || actions[1].ID != "untouched" {
		t.Fatal("search mutated provider backing arrays")
	}
}

func TestProgressiveSearchCoalescesRapidProviderCompletions(t *testing.T) {
	slowRelease := make(chan struct{})
	var releaseOnce sync.Once
	releaseSlow := func() { releaseOnce.Do(func() { close(slowRelease) }) }
	defer releaseSlow()

	var progressCount atomic.Int32
	coalesced := make(chan Progress, 16)

	providers := make([]ResultProvider, 0, 7)
	providers = append(providers, providerFunc(func(string) []contracts.SearchResult {
		<-slowRelease
		return []contracts.SearchResult{scoredResult("slow-final", "Slow Final", 100)}
	}))
	for i := 0; i < 6; i++ {
		id := "fast-" + strconv.Itoa(i)
		score := float64(50 - i)
		providers = append(providers, providerFunc(func(string) []contracts.SearchResult {
			return []contracts.SearchResult{scoredResult(id, id, score)}
		}))
	}

	service := NewServiceWithState(NewStateStore(""), providers...)
	RegisterProgressHandler(service, func(progress Progress) {
		progressCount.Add(1)
		coalesced <- progress
	})

	done := make(chan contracts.SearchResponse, 1)
	go func() {
		done <- service.SearchProgressively(context.Background(), "burst-query", "req-burst")
	}()

	deadline := time.After(2 * time.Second)
	var lastProgress Progress
	for {
		select {
		case p := <-coalesced:
			lastProgress = p
			if len(p.Response.Results) == 6 {
				goto flushed
			}
		case <-deadline:
			t.Fatalf("expected all 6 fast providers in coalesced progress, got count=%d last=%d", progressCount.Load(), len(lastProgress.Response.Results))
		}
	}
flushed:
	if got := progressCount.Load(); got >= 6 {
		t.Fatalf("expected rapid provider completions to be coalesced (<6 progress emits), got %d", got)
	}
	releaseSlow()
	final := <-done
	if len(final.Results) != 7 || final.Results[0].ID != "slow-final" {
		t.Fatalf("expected 7 final results with slow-final first, got %#v", final.Results)
	}
}

func TestMergeProviderResultsDeduplicatesInOrderAtScale(t *testing.T) {
	seed := []contracts.SearchResult{scoredResult("seed-1", "Seed 1", 10)}
	batch1 := make([]contracts.SearchResult, 0, 500)
	batch2 := make([]contracts.SearchResult, 0, 500)
	for i := 0; i < 500; i++ {
		id := "item-" + strconv.Itoa(i)
		batch1 = append(batch1, scoredResult(id, "B1-"+id, 50))
		batch2 = append(batch2, scoredResult(id, "B2-"+id, 90))
	}
	batch2 = append(batch2, scoredResult("unique-2", "Unique 2", 20))

	merged := mergeProviderResults(seed, [][]contracts.SearchResult{batch1, batch2})
	if len(merged) != 502 {
		t.Fatalf("expected 502 deduplicated results, got %d", len(merged))
	}
	if merged[1].Title != "B1-item-0" {
		t.Fatalf("expected earlier provider to win deduplication, got %#v", merged[1])
	}
}
