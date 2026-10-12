package scenarios

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/apache261/Byakugan-core/aml/domain"
)

func TestInitialScenarioFamilies(t *testing.T) {
	now := time.Now().UTC()
	base := domain.ScenarioVersion{ScenarioID: "scenario", Version: 1, SchemaVersion: 1, Typology: "test", ExecutionModes: []string{"simulation"}, SubjectBasis: domain.SubjectCustomer, WindowSeconds: 3600, LateDataBehavior: "reconcile", ReasonTemplate: "{{left}} {{operator}} {{right}}", Severity: "medium", RiskScore: domain.MustDecimal("50"), Author: "test", CreatedAt: now}
	tests := []struct {
		name                  string
		node                  domain.ScenarioNode
		input                 Input
		matched, insufficient bool
	}{
		{"velocity boundary", metric("count", "3"), Input{Metrics: map[string]domain.Decimal{"count:customer:3600": domain.MustDecimal("3")}}, true, false},
		{"velocity negative", metric("count", "3"), Input{Metrics: map[string]domain.Decimal{"count:customer:3600": domain.MustDecimal("2")}}, false, false},
		{"repeated values", metric("repeated_value_count", "2"), Input{Metrics: map[string]domain.Decimal{"repeated_value_count:customer:3600": domain.MustDecimal("4")}}, true, false},
		{"profile deviation", field("profile.expected_count", "gt", `"10"`), Input{Facts: map[string]json.RawMessage{"profile.expected_count": json.RawMessage(`"12"`)}}, true, false},
		{"high risk combination", domain.ScenarioNode{All: []domain.ScenarioNode{field("customer.risk_rating", "eq", `"high"`), field("transaction.destination_country", "eq", `"XY"`)}}, Input{Facts: map[string]json.RawMessage{"customer.risk_rating": json.RawMessage(`"high"`)}, Transaction: domain.Transaction{DestinationCountry: "XY"}}, true, false},
		{"rapid movement", network("pass_through", "1"), Input{Metrics: map[string]domain.Decimal{"pass_through:customer:3600": domain.MustDecimal("1")}}, true, false},
		{"fan out", network("fan_out", "5"), Input{Metrics: map[string]domain.Decimal{"fan_out:customer:3600": domain.MustDecimal("6")}}, true, false},
		{"cold start", metric("sum", "100"), Input{}, false, true},
		{"multi currency missing conversion", domain.ScenarioNode{Field: "transaction.amount", Operator: "gte", PolicyRef: "converted.PHP.threshold"}, Input{Transaction: domain.Transaction{Money: domain.Money{Amount: domain.MustDecimal("100"), AssetCode: "USD"}}}, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version := base
			version.Condition = tt.node
			observation, err := Evaluate(version, tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if observation.Matched != tt.matched || observation.InsufficientData != tt.insufficient {
				t.Fatalf("observation=%#v", observation)
			}
		})
	}
}

func TestScenarioValidationRejectsExecutableAndUnboundedDocuments(t *testing.T) {
	now := time.Now().UTC()
	version := domain.ScenarioVersion{ScenarioID: "scenario", Version: 1, SchemaVersion: 1, Typology: "unsafe", ExecutionModes: []string{"simulation"}, SubjectBasis: domain.SubjectCustomer, WindowSeconds: 3600, LateDataBehavior: "reconcile", Condition: domain.ScenarioNode{Field: "transaction.amount; DROP TABLE", Operator: "gte", Value: json.RawMessage(`"1"`)}, ReasonTemplate: "unsafe", Severity: "high", RiskScore: domain.MustDecimal("90"), Author: "test", CreatedAt: now}
	if _, err := domain.PrepareScenarioVersion(version); err == nil {
		t.Fatal("expected unknown/executable field to be rejected")
	}
}

func TestScenarioFamilyBoundaryColdLateAndCrossAssetFixtures(t *testing.T) {
	now := time.Now().UTC()
	families := []struct {
		name    string
		node    domain.ScenarioNode
		leftKey string
		fact    bool
	}{
		{"velocity", domain.ScenarioNode{Metric: "count", Operator: "gte", PolicyRef: "threshold.PHP", SubjectBasis: domain.SubjectCustomer, WindowSeconds: 3600}, "count:customer:3600", false},
		{"repeated_value", domain.ScenarioNode{Metric: "repeated_value_count", Operator: "gte", PolicyRef: "threshold.PHP", SubjectBasis: domain.SubjectCustomer, WindowSeconds: 3600}, "repeated_value_count:customer:3600", false},
		{"profile_deviation", domain.ScenarioNode{Field: "profile.expected_count", Operator: "gte", PolicyRef: "threshold.PHP"}, "profile.expected_count", true},
		{"high_risk_combination", domain.ScenarioNode{Field: "profile.expected_amount", Operator: "gte", PolicyRef: "threshold.PHP"}, "profile.expected_amount", true},
		{"rapid_movement", domain.ScenarioNode{Metric: "active_span", Operator: "gte", PolicyRef: "threshold.PHP", SubjectBasis: domain.SubjectCustomer, WindowSeconds: 3600}, "active_span:customer:3600", false},
		{"pass_through", domain.ScenarioNode{NetworkMetric: "pass_through", Operator: "gte", PolicyRef: "threshold.PHP", SubjectBasis: domain.SubjectCustomer, WindowSeconds: 3600}, "pass_through:customer:3600", false},
		{"network", domain.ScenarioNode{NetworkMetric: "fan_out", Operator: "gte", PolicyRef: "threshold.PHP", SubjectBasis: domain.SubjectCustomer, WindowSeconds: 3600}, "fan_out:customer:3600", false},
	}
	for _, family := range families {
		t.Run(family.name, func(t *testing.T) {
			version := domain.ScenarioVersion{ScenarioID: family.name, Version: 1, SchemaVersion: 1, Typology: family.name, ExecutionModes: []string{"simulation"}, SubjectBasis: domain.SubjectCustomer, WindowSeconds: 3600, LateDataBehavior: "ignore", Condition: family.node, ReasonTemplate: "matched", Severity: "medium", RiskScore: domain.MustDecimal("50"), Author: "test", CreatedAt: now}
			makeInput := func(value string) Input {
				input := Input{PolicyValues: map[string]json.RawMessage{"threshold.PHP": json.RawMessage(`"3"`)}}
				if family.fact {
					input.Facts = map[string]json.RawMessage{family.leftKey: json.RawMessage(`"` + value + `"`)}
				} else {
					input.Metrics = map[string]domain.Decimal{family.leftKey: domain.MustDecimal(value)}
				}
				return input
			}
			for name, input := range map[string]Input{"positive": makeInput("4"), "boundary": makeInput("3"), "negative": makeInput("2")} {
				observation, err := Evaluate(version, input)
				if err != nil {
					t.Fatal(err)
				}
				want := name != "negative"
				if observation.Matched != want {
					t.Fatalf("%s matched=%v", name, observation.Matched)
				}
			}
			cold := makeInput("3")
			cold.Facts = nil
			cold.Metrics = nil
			observation, err := Evaluate(version, cold)
			if err != nil || !observation.InsufficientData {
				t.Fatalf("cold-start=%#v err=%v", observation, err)
			}
			late := makeInput("4")
			late.LateData = true
			observation, err = Evaluate(version, late)
			if err != nil || observation.Matched {
				t.Fatalf("late-data=%#v err=%v", observation, err)
			}
			crossAsset := makeInput("4")
			delete(crossAsset.PolicyValues, "threshold.PHP")
			observation, err = Evaluate(version, crossAsset)
			if err != nil || !observation.InsufficientData {
				t.Fatalf("cross-asset missing policy=%#v err=%v", observation, err)
			}
		})
	}
}

func metric(name, threshold string) domain.ScenarioNode {
	return domain.ScenarioNode{Metric: name, Operator: "gte", Value: json.RawMessage(`"` + threshold + `"`), SubjectBasis: domain.SubjectCustomer, WindowSeconds: 3600}
}
func network(name, threshold string) domain.ScenarioNode {
	return domain.ScenarioNode{NetworkMetric: name, Operator: "gte", Value: json.RawMessage(`"` + threshold + `"`), SubjectBasis: domain.SubjectCustomer, WindowSeconds: 3600}
}
func field(name, operator, value string) domain.ScenarioNode {
	return domain.ScenarioNode{Field: name, Operator: operator, Value: json.RawMessage(value)}
}
