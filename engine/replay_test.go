package engine

import (
	"context"
	"errors"
	"github.com/apache261/Byakugan-core/domain"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestReplayEngineCoalescesWithoutCallbackLocks(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	var replay *ReplayEngine
	f := framework(t, nil, []Registration{{ID: "gate", Evaluator: EvaluatorFunc(func(ctx context.Context, _ domain.TransferCheckRequest, _ Context) ([]domain.RuleHit, error) {
		if calls.Add(1) == 1 {
			close(entered)
		}
		replay.History(1)
		select {
		case <-release:
			return nil, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})}}, DefaultPolicy())
	var err error
	replay, err = NewReplayEngine(f, Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	firstDone := make(chan domain.TransferCheck, 1)
	go func() {
		got, err := replay.Check(ctx, validRequest(), Context{})
		if err != nil {
			t.Error(err)
		}
		firstDone <- got
	}()
	<-entered
	canceled, cancelWait := context.WithCancel(ctx)
	cancelWait()
	if got, err := replay.Check(canceled, validRequest(), Context{}); !errors.Is(err, context.Canceled) || got.Decision == domain.DecisionAllow {
		t.Fatalf("waiter cancellation accepted: %+v %v", got, err)
	}
	const concurrent = 8
	results := make(chan domain.TransferCheck, concurrent)
	var wg sync.WaitGroup
	for i := 0; i < concurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := replay.Check(ctx, validRequest(), Context{})
			if err != nil {
				t.Error(err)
			}
			results <- got
		}()
	}
	close(release)
	first := <-firstDone
	wg.Wait()
	close(results)
	for got := range results {
		if got.ID != first.ID {
			t.Fatalf("same request evaluated twice: %s %s", first.ID, got.ID)
		}
	}
	if calls.Load() != 1 || len(replay.History(0)) != 1 {
		t.Fatal("replay invoked callbacks or duplicated history")
	}
	first.Reasons[0] = "modified"
	got, err := replay.Check(ctx, validRequest(), Context{})
	if err != nil || got.Reasons[0] == "modified" {
		t.Fatal("caller mutated replay cache")
	}
}

func TestReplayEngineBoundsAndDoesNotCacheFailure(t *testing.T) {
	calls := 0
	f := framework(t, nil, []Registration{{ID: "transient", Evaluator: EvaluatorFunc(func(context.Context, domain.TransferCheckRequest, Context) ([]domain.RuleHit, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("transient")
		}
		return nil, nil
	})}}, DefaultPolicy())
	replay, err := NewReplayEngine(f, Options{MaxIdempotencyEntries: 2, MaxHistoryEntries: 2})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := replay.Check(context.Background(), validRequest(), Context{}); err == nil || got.Decision == domain.DecisionAllow {
		t.Fatal("failed evaluation allowed")
	}
	if len(replay.History(0)) != 0 {
		t.Fatal("failed evaluation recorded")
	}
	original, err := replay.Check(context.Background(), validRequest(), Context{})
	if err != nil {
		t.Fatal(err)
	}
	conflict := validRequest()
	conflict.Payload.InstructedAmount.Amount = "51"
	got, err := replay.Check(context.Background(), conflict, Context{})
	if err != nil || got.Decision != domain.DecisionReview {
		t.Fatalf("conflict not reviewed: %+v %v", got, err)
	}
	got, err = replay.Check(context.Background(), validRequest(), Context{})
	if err != nil || got.ID != original.ID {
		t.Fatal("conflict replaced original")
	}
	other := validRequest()
	other.IdempotencyKey = "other"
	if _, err := replay.Check(context.Background(), other, Context{}); err != nil {
		t.Fatal(err)
	}
	got, err = replay.Check(context.Background(), validRequest(), Context{})
	if err != nil || got.ID == original.ID || len(replay.History(0)) != 2 || replay.entries != 2 {
		t.Fatal("history/cache caps not enforced")
	}
}

func TestReplayEngineRejectsFingerprintSerializationFailure(t *testing.T) {
	replay, err := NewReplayEngine(framework(t, nil, nil, DefaultPolicy()), Options{})
	if err != nil {
		t.Fatal(err)
	}
	request := validRequest()
	request.Payload.CreationDateTime = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
	got, err := replay.Check(context.Background(), request, Context{})
	if err == nil || got.Decision == domain.DecisionAllow || len(replay.History(0)) != 0 || replay.entries != 0 {
		t.Fatalf("unserializable payload entered replay cache: %+v %v", got, err)
	}
}
