package rules

import (
	"encoding/json"
	"github.com/apache261/Byakugan-core/domain"
	"testing"
	"time"
)

func TestMonetaryComparisonsUseExactDecimals(t *testing.T) {
	for _, tc := range []struct {
		actual, op, want string
		match            bool
	}{
		{"0.3", "eq", "0.30", true}, {"10", "gt", "2", true},
		{"999999999999999999.99", "gt", "999999999999999999.98", true},
		{"0.29", "gte", "0.30", false}, {"0.31", "lt", "0.30", false},
		{"1", "neq", "1.00", false},
	} {
		raw, _ := json.Marshal(Definition{Field: "amount", Op: tc.op, Value: tc.want, Outcome: domain.DecisionReview})
		_, match, err := Evaluate(domain.Rule{Definition: raw}, contextualRequest(tc.actual))
		if err != nil || match != tc.match {
			t.Fatalf("%+v: match %v err %v", tc, match, err)
		}
	}
	for _, value := range []any{0.1, json.Number("0.1"), "1e2", "-1", "NaN", "1.0001"} {
		raw, _ := json.Marshal(Definition{Field: "amount", Op: "gte", Value: value, Outcome: domain.DecisionReview})
		if err := Validate(domain.Rule{Definition: raw}); err == nil {
			t.Fatalf("invalid monetary threshold accepted: %v", value)
		}
	}
}

func TestExactHistoryAndSuppliedAggregates(t *testing.T) {
	now := time.Now().UTC()
	req := contextualRequest("0.10")
	history := []domain.TransferCheck{{Request: contextualRequest("0.20"), CreatedAt: now}}
	foreign := contextualRequest("999999999999999999.99")
	foreign.Payload.InstructedAmount.Currency = "USD"
	history = append(history, domain.TransferCheck{Request: foreign, CreatedAt: now})
	for _, field := range []string{"velocity.debtor_amount_sum_including_current", "velocity.creditor_amount_sum_including_current"} {
		raw, _ := json.Marshal(Definition{Field: field, Op: "eq", Value: "0.30", WindowMinutes: 15, Outcome: domain.DecisionReview})
		rule := domain.Rule{Definition: raw}
		for _, ctx := range []Context{
			{Now: now, History: history},
			{Metrics: Metrics{Loaded: true, Currency: "PHP", DebtorAmountSum: "0.20", CreditorAmountSum: "0.20"}},
			{MetricsByWindow: map[time.Duration]Metrics{15 * time.Minute: {Loaded: true, Currency: "PHP", DebtorAmountSum: "0.20", CreditorAmountSum: "0.20"}}},
		} {
			_, match, err := EvaluateWithContext(rule, req, ctx)
			if err != nil || !match {
				t.Fatalf("%s: %v %v", field, match, err)
			}
		}
		for _, metrics := range []Metrics{{Loaded: true, Currency: "USD", DebtorAmountSum: "0.2"}, {Loaded: true, Currency: "PHP", DebtorAmountSum: "invalid", CreditorAmountSum: "invalid"}} {
			if _, _, err := EvaluateWithContext(rule, req, Context{Metrics: metrics}); err == nil {
				t.Fatalf("invalid metrics accepted: %+v", metrics)
			}
		}
	}
	badHistory := []domain.TransferCheck{{Request: contextualRequest("invalid"), CreatedAt: now}}
	if _, _, err := EvaluateWithContext(domain.Rule{Definition: json.RawMessage(`{"field":"velocity.debtor_amount_sum","op":"gte","value":"0","outcome":"review"}`)}, req, Context{History: badHistory}); err == nil {
		t.Fatal("invalid historical amount ignored")
	}
}

func TestRepeatedAmountsCompareCanonicalValues(t *testing.T) {
	rule := domain.Rule{Definition: json.RawMessage(`{"field":"velocity.debtor_repeated_amount_count","op":"eq","value":1,"outcome":"review"}`)}
	_, match, err := EvaluateWithContext(rule, contextualRequest("1.20"), Context{History: []domain.TransferCheck{{Request: contextualRequest("01.2")}}})
	if err != nil || !match {
		t.Fatalf("canonical repeat missed: %v %v", match, err)
	}
}
