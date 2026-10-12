package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/apache261/Byakugan-core/domain"
	"github.com/apache261/Byakugan-core/engine"
	"os"
)

func main() {
	var req domain.TransferCheckRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		panic(err)
	}
	evaluator := engine.EvaluatorFunc(func(ctx context.Context, req domain.TransferCheckRequest, data engine.Context) ([]domain.RuleHit, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var flag bool
		if err := json.Unmarshal(data.Facts["institution.local_risk"], &flag); err != nil {
			return nil, err
		}
		if !flag {
			return nil, nil
		}
		return []domain.RuleHit{{RuleID: "institution-risk", RuleName: "Institution risk", Outcome: domain.DecisionReview, Score: 60, Reason: "caller-supplied local risk flag"}}, nil
	})
	detector, err := engine.NewFramework(engine.DefaultRules(), []engine.Registration{{ID: "institution-risk", Priority: 10, Evaluator: evaluator}}, engine.DefaultPolicy())
	if err != nil {
		panic(err)
	}
	result, err := detector.Evaluate(context.Background(), req, engine.Context{Facts: map[string]json.RawMessage{"institution.local_risk": json.RawMessage(`true`)}})
	if err != nil {
		panic(err)
	}
	fmt.Println(result.Decision, result.Reasons)
}
