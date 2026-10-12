package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/apache261/Byakugan-core/aml/domain"
	"github.com/apache261/Byakugan-core/aml/scenarios"
)

func main() {
	evaluator := scenarios.EvaluatorFunc(func(ctx context.Context, input scenarios.Input) (scenarios.Observation, error) {
		if err := ctx.Err(); err != nil {
			return scenarios.Observation{}, err
		}
		result := scenarios.Observation{ScenarioID: "institution-observation", ScenarioVersion: 1, Typology: "local-context", Severity: "low", RiskScore: domain.MustDecimal("20"), EvidenceIDs: input.EvidenceIDs}
		raw, known := input.Facts["institution.local_risk"]
		if !known {
			result.InsufficientData = true
			result.Reasons = []string{"local risk context unavailable"}
			return result, nil
		}
		if err := json.Unmarshal(raw, &result.Matched); err != nil {
			return result, err
		}
		result.Reasons = []string{"caller-supplied local risk context"}
		return result, nil
	})
	version := domain.ScenarioVersion{ScenarioID: "exact-amount", Version: 1, SchemaVersion: 1, Typology: "supplied-threshold", ExecutionModes: []string{"simulation"}, SubjectBasis: domain.SubjectCustomer, WindowSeconds: 3600, LateDataBehavior: "observe", Condition: domain.ScenarioNode{Field: "transaction.amount", Operator: "gte", PolicyRef: "institution.threshold"}, ReasonTemplate: "{{left}} {{operator}} {{right}}", Severity: "medium", RiskScore: domain.MustDecimal("30"), Author: "synthetic-example"}
	detector, err := scenarios.NewFramework([]domain.ScenarioVersion{version}, []scenarios.Registration{{ID: "institution-observation", Evaluator: evaluator}})
	if err != nil {
		panic(err)
	}
	result, err := detector.Evaluate(context.Background(), scenarios.Input{Transaction: domain.Transaction{Money: domain.Money{Amount: domain.MustDecimal("1250.5"), AssetCode: "PHP"}}, Facts: map[string]json.RawMessage{"institution.local_risk": json.RawMessage(`true`)}, PolicyValues: map[string]json.RawMessage{"institution.threshold": json.RawMessage(`"1250.50"`)}, EvidenceIDs: []string{"synthetic-tx-b", "synthetic-tx-a"}})
	if err != nil {
		panic(err)
	}
	for _, observation := range result.Observations {
		fmt.Println(observation.ScenarioID, observation.Matched, observation.EvidenceIDs)
	}
}
