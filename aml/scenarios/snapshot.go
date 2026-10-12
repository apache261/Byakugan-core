package scenarios

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/apache261/Byakugan-core/aml/domain"
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
func cloneInput(input Input) (Input, error) {
	input.Facts = cloneRawMap(input.Facts)
	input.PolicyValues = cloneRawMap(input.PolicyValues)
	input.EvidenceIDs = append([]string(nil), input.EvidenceIDs...)
	if input.Metrics != nil {
		metrics := make(map[string]domain.Decimal, len(input.Metrics))
		for key, value := range input.Metrics {
			metrics[key] = value
		}
		input.Metrics = metrics
	}
	if input.Screening != nil {
		screening := make(map[string]bool, len(input.Screening))
		for key, value := range input.Screening {
			screening[key] = value
		}
		input.Screening = screening
	}
	input.Transaction.Subjects = append([]domain.SubjectReference(nil), input.Transaction.Subjects...)
	if input.Transaction.RelatedTransaction != nil {
		link := *input.Transaction.RelatedTransaction
		input.Transaction.RelatedTransaction = &link
	}
	if input.Transaction.Metadata != nil {
		raw, err := json.Marshal(input.Transaction.Metadata)
		if err != nil {
			return Input{}, fmt.Errorf("invalid transaction metadata: %w", err)
		}
		var metadata map[string]any
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if err := decoder.Decode(&metadata); err != nil {
			return Input{}, err
		}
		input.Transaction.Metadata = metadata
	}
	return input, nil
}
