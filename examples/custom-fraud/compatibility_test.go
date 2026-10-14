package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/apache261/Byakugan-core/domain"
	"github.com/apache261/Byakugan-core/engine"
	"os"
	"testing"
)

// This independent consumer uses only released public APIs. The release gates
// run it with both the pinned module and a temporary source workspace.
func TestPublicConsumerEvaluationAndCancellation(t *testing.T) {
	raw, err := os.ReadFile("../transfer.json")
	if err != nil {
		t.Fatal(err)
	}
	var req domain.TransferCheckRequest
	if err = json.Unmarshal(raw, &req); err != nil {
		t.Fatal(err)
	}
	f, err := engine.NewFramework(nil, []engine.Registration{{ID: "consumer-risk", Evaluator: engine.EvaluatorFunc(func(ctx context.Context, _ domain.TransferCheckRequest, _ engine.Context) ([]domain.RuleHit, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return []domain.RuleHit{{RuleID: "consumer-risk", Outcome: domain.DecisionReview, Score: 60, Reason: "synthetic consumer risk"}}, nil
	})}}, engine.DefaultPolicy())
	if err != nil {
		t.Fatal(err)
	}
	result, err := f.Evaluate(context.Background(), req, engine.Context{})
	if err != nil || result.Decision != domain.DecisionReview || len(result.RuleHits) != 1 {
		t.Fatalf("public contract changed: %+v %v", result, err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	result, err = f.Evaluate(cancelled, req, engine.Context{})
	if !errors.Is(err, context.Canceled) || result.Decision == domain.DecisionAllow {
		t.Fatalf("cancellation lost: %+v %v", result, err)
	}
}
