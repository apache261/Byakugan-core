package engine

import (
	"encoding/json"
	"github.com/apache261/Byakugan-core/domain"
	"github.com/apache261/Byakugan-core/rules"
	"time"
)

func cloneRawMap(values map[string]json.RawMessage) map[string]json.RawMessage {
	if values == nil {
		return nil
	}
	result := make(map[string]json.RawMessage, len(values))
	for key, value := range values {
		result[key] = append(json.RawMessage(nil), value...)
	}
	return result
}
func cloneEvaluationContext(data Context) Context {
	data.Facts = cloneRawMap(data.Facts)
	data.PolicyValues = cloneRawMap(data.PolicyValues)
	data.DisabledRuleIDs = append([]string(nil), data.DisabledRuleIDs...)
	data.BlockedAccountIDs = append([]string(nil), data.BlockedAccountIDs...)
	if data.Rules.MetricsByWindow != nil {
		metrics := make(map[time.Duration]rules.Metrics, len(data.Rules.MetricsByWindow))
		for key, value := range data.Rules.MetricsByWindow {
			metrics[key] = value
		}
		data.Rules.MetricsByWindow = metrics
	}
	if data.Rules.History != nil {
		history := append([]domain.TransferCheck(nil), data.Rules.History...)
		for i := range history {
			history[i].Reasons = append([]string(nil), history[i].Reasons...)
			history[i].RuleHits = append([]domain.RuleHit(nil), history[i].RuleHits...)
		}
		data.Rules.History = history
	}
	return data
}
