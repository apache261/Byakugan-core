package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/apache261/Byakugan-core/domain"
	"github.com/apache261/Byakugan-core/rules"
	"testing"
	"time"
)

func TestFrameworkPreservesCallerPolicyIdentity(t *testing.T) {
	result, err := framework(t, nil, nil, DefaultPolicy()).Evaluate(context.Background(), validRequest(), Context{PolicyVersionID: "synthetic-policy-v1"})
	if err != nil || result.PolicyVersionID != "synthetic-policy-v1" {
		t.Fatalf("policy provenance lost: %+v %v", result, err)
	}
}
func BenchmarkFramework(b *testing.B) {
	for _, count := range []int{1, 10, 100, 1000} {
		for _, aggregates := range []bool{false, true} {
			b.Run(fmt.Sprintf("rules=%d/aggregates=%t", count, aggregates), func(b *testing.B) {
				list := make([]domain.Rule, count)
				for i := range list {
					definition := `{"field":"amount","op":"gte","value":"100000","outcome":"review"}`
					if aggregates {
						definition = `{"field":"velocity.debtor_transfer_count","op":"gte","value":100000,"window_minutes":15,"outcome":"review"}`
					}
					list[i] = domain.Rule{ID: fmt.Sprintf("bench-%d", i), Enabled: true, Priority: i, Definition: json.RawMessage(definition)}
				}
				f, err := NewFramework(list, nil, DefaultPolicy())
				if err != nil {
					b.Fatal(err)
				}
				data := Context{PolicyVersionID: "benchmark"}
				if aggregates {
					data.Rules.MetricsByWindow = map[time.Duration]rules.Metrics{15 * time.Minute: {DebtorTransferCount: 10}}
				}
				req := validRequest()
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if _, err := f.Evaluate(context.Background(), req, data); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
