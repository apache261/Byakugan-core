package rules

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/apache261/Byakugan-core/domain"
)

func TestValidateAcceptsPortableRule(t *testing.T) {
	rule := domain.Rule{Definition: json.RawMessage(`{"field":"amount","op":"gte","value":100,"outcome":"review"}`)}
	if err := Validate(rule); err != nil {
		t.Fatalf("portable rule rejected: %v", err)
	}
}

func TestEvaluateWithContextSupportsVelocityAndRouteSignals(t *testing.T) {
	now := time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC)
	request := contextualRequest(1500)
	request.Payload.RequestedExecutionDate = "2026-09-26"
	history := []domain.TransferCheck{
		{Request: contextualRequest(1500), CreatedAt: now.Add(-10 * time.Minute)},
		{Request: requestWithCreditor("creditor-2", 2000), CreatedAt: now.Add(-20 * time.Minute)},
		{Request: requestWithCreditor("creditor-old", 9000), CreatedAt: now.Add(-2 * time.Hour)},
	}
	context := Context{
		History: history,
		Now:     now,
		DebtorAccount: AccountContext{
			RiskTier:  "high",
			UpdatedAt: now.Add(-time.Hour),
		},
	}
	tests := []Definition{
		{Field: "velocity.debtor_transfer_count", Op: "gte", Value: 2, WindowMinutes: 30, Outcome: domain.DecisionReview},
		{Field: "velocity.debtor_amount_sum_including_current", Op: "gte", Value: 5000, WindowMinutes: 30, Outcome: domain.DecisionReview},
		{Field: "velocity.debtor_unique_creditor_count", Op: "gte", Value: 2, WindowMinutes: 30, Outcome: domain.DecisionReview},
		{Field: "velocity.debtor_repeated_amount_count", Op: "gte", Value: 1, WindowMinutes: 30, Outcome: domain.DecisionReview},
		{Field: "beneficiary.first_time_creditor", Op: "eq", Value: false, Outcome: domain.DecisionReview},
		{Field: "route.country_pair", Op: "eq", Value: "PH-PH", Outcome: domain.DecisionReview},
		{Field: "requested_execution_days_ahead", Op: "gte", Value: 3, Outcome: domain.DecisionReview},
		{Field: "debtor_account.risk_tier", Op: "eq", Value: "high", Outcome: domain.DecisionReview},
		{Field: "account.debtor_metadata_recently_changed", Op: "eq", Value: true, WindowHours: 2, Outcome: domain.DecisionReview},
	}
	for _, definition := range tests {
		raw, err := json.Marshal(definition)
		if err != nil {
			t.Fatal(err)
		}
		_, matched, err := EvaluateWithContext(domain.Rule{ID: definition.Field, Name: definition.Field, Definition: raw}, request, context)
		if err != nil {
			t.Fatalf("%s: %v", definition.Field, err)
		}
		if !matched {
			t.Fatalf("expected %s to match", definition.Field)
		}
	}
}

func TestEvaluateWithContextUsesPrecomputedWindowMetrics(t *testing.T) {
	raw := json.RawMessage(`{"field":"velocity.debtor_transfer_count","op":"gte","value":3,"window_minutes":15,"outcome":"review"}`)
	_, matched, err := EvaluateWithContext(domain.Rule{ID: "velocity", Name: "Velocity", Definition: raw}, contextualRequest(100), Context{
		MetricsByWindow: map[time.Duration]Metrics{
			15 * time.Minute: {Loaded: true, DebtorTransferCount: 2},
			24 * time.Hour:   {Loaded: true, DebtorTransferCount: 99},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if matched {
		t.Fatal("expected the matching 15-minute metric to be used")
	}
}

func TestMetricWindowsFromRules(t *testing.T) {
	rules := []domain.Rule{
		{Definition: json.RawMessage(`{"field":"velocity.debtor_transfer_count","op":"gte","value":3,"window_minutes":15,"outcome":"review"}`)},
		{Definition: json.RawMessage(`{"field":"velocity.debtor_amount_sum","op":"gte","value":1000,"window_hours":24,"outcome":"review"}`)},
	}
	windows := MetricWindowsFromRules(rules)
	if len(windows) != 2 {
		t.Fatalf("windows = %v, want two distinct windows", windows)
	}
}

func contextualRequest(amount float64) domain.TransferCheckRequest {
	return domain.TransferCheckRequest{MessageType: "pacs.008", Payload: domain.Pacs008Transfer{
		MessageID: "message", CreationDateTime: time.Date(2026, 9, 23, 9, 0, 0, 0, time.UTC),
		PaymentIdentification: domain.PaymentIdentification{EndToEndID: "e2e"},
		InstructedAmount:      domain.Money{Amount: amount, Currency: "PHP"},
		DebtorAccount:         domain.AccountRef{ID: "debtor"}, CreditorAccount: domain.AccountRef{ID: "creditor"},
		DebtorAgent: domain.Agent{BICFI: "DEUTPHMM"}, CreditorAgent: domain.Agent{BICFI: "BOPIPHMM"},
	}}
}

func requestWithCreditor(creditor string, amount float64) domain.TransferCheckRequest {
	request := contextualRequest(amount)
	request.Payload.CreditorAccount.ID = creditor
	return request
}

func TestValidateAcceptsPortableWindowFields(t *testing.T) {
	rule := domain.Rule{Definition: json.RawMessage(`{"field":"velocity.debtor_amount_sum","op":"gte","value":100,"window_hours":24,"outcome":"review"}`)}
	if err := Validate(rule); err != nil {
		t.Fatalf("portable window rule rejected: %v", err)
	}
}

func TestValidateRejectsPrivateSignalFields(t *testing.T) {
	rule := domain.Rule{Definition: json.RawMessage(`{"field":"external.ml_score","op":"gte","value":0.8,"outcome":"review"}`)}
	if err := Validate(rule); err == nil {
		t.Fatal("private signal field was accepted")
	}
}

func TestValidateRejectsTrailingJSON(t *testing.T) {
	rule := domain.Rule{Definition: json.RawMessage(`{"field":"amount","op":"gte","value":100,"outcome":"review"} {}`)}
	if err := Validate(rule); err == nil {
		t.Fatal("rule with trailing JSON was accepted")
	}
}
