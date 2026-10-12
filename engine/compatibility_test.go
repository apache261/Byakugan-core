package engine

import (
	"encoding/json"
	"github.com/apache261/Byakugan-core/domain"
	"reflect"
	"testing"
)

// Freeze pre-migration semantics, including the legacy error-after-block review.
func TestCompatibilityBaseline(t *testing.T) {
	rule := func(id string, priority int, outcome string, score int) domain.Rule {
		raw, _ := json.Marshal(map[string]any{"field": "message_type", "op": "eq", "value": "pacs.008", "outcome": outcome, "score": score, "reason": id})
		return domain.Rule{ID: id, Name: id, Priority: priority, Enabled: true, Definition: raw}
	}
	review := rule("review", 10, "review", 10)
	block := rule("block", 20, "block", 20)
	allow := rule("allow", 20, "allow", 5)
	broken := domain.Rule{ID: "broken", Priority: 30, Enabled: true, Definition: json.RawMessage(`{"field":"unknown","op":"eq","value":1,"outcome":"block"}`)}
	tests := []struct {
		name         string
		rules        []domain.Rule
		context      Context
		decision     domain.Decision
		score        float64
		ids, reasons []string
	}{
		{"stable ordering and allow", []domain.Rule{block, allow, review}, Context{}, domain.DecisionBlock, 35, []string{"review", "block", "allow"}, []string{"review", "block", "allow"}},
		{"blocked account early exit", []domain.Rule{review, broken}, Context{BlockedAccountIDs: []string{"debtor"}}, domain.DecisionBlock, 0, []string{}, []string{"debtor or creditor account is blocked"}},
		{"legacy error after block", []domain.Rule{block, broken}, Context{}, domain.DecisionReview, 20, []string{"block"}, []string{"block", "rule broken could not be evaluated"}},
		{"threshold", []domain.Rule{allow}, Context{ReviewThreshold: 5}, domain.DecisionReview, 5, []string{"allow"}, []string{"allow", "score reached review threshold"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := New(tc.rules).CheckWithContext(validRequest(), tc.context)
			ids := make([]string, 0, len(got.RuleHits))
			for _, hit := range got.RuleHits {
				ids = append(ids, hit.RuleID)
			}
			if got.Decision != tc.decision || got.Score != tc.score || !reflect.DeepEqual(ids, tc.ids) || !reflect.DeepEqual(got.Reasons, tc.reasons) {
				t.Fatalf("unexpected baseline: %+v", got)
			}
		})
	}
}
