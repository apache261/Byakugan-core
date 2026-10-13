package rules

import (
	"encoding/json"
	"github.com/apache261/Byakugan-core/domain"
	"testing"
)

func TestCustomLeavesPreservePortableShortCircuitAndExactFacts(t *testing.T) {
	req := domain.TransferCheckRequest{MessageType: "pacs.008", Payload: domain.Pacs008Transfer{InstructedAmount: domain.Money{Amount: "999999999999999999.99", Currency: "USD"}}}
	rule := domain.Rule{ID: "custom", Definition: json.RawMessage(`{"all":[{"field":"amount","op":"eq","value":"999999999999999999.99"},{"field":"custom.exact_sum","op":"gt","value":"999999999999999999.99"}],"outcome":"review","score":5}`)}
	calls := 0
	leaf := func(def Definition) (bool, bool, error) {
		if def.Field != "custom.exact_sum" {
			return false, false, nil
		}
		calls++
		matched, err := CompareValue("1000000000000000000", def.Op, def.Value, "USD")
		return matched, true, err
	}
	hit, matched, err := EvaluateWithLeaves(rule, req, Context{}, leaf)
	if err != nil || !matched || hit.Score != 5 || calls != 1 {
		t.Fatalf("%+v %v %v %d", hit, matched, err, calls)
	}
	req.Payload.InstructedAmount.Amount = "1"
	_, matched, err = EvaluateWithLeaves(rule, req, Context{}, leaf)
	if err != nil || matched || calls != 1 {
		t.Fatalf("short circuit %v %v %d", matched, err, calls)
	}
	if _, err := CompareValue(0.1, "eq", "0.1", "USD"); err == nil {
		t.Fatal("float monetary fact accepted")
	}
	if _, _, err := EvaluateWithLeaves(domain.Rule{Definition: json.RawMessage(`{"field":"message_type"} {}`)}, req, Context{}, leaf); err == nil {
		t.Fatal("trailing JSON accepted")
	}
}
