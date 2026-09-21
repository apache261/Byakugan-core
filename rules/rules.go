package rules

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/apache261/Byakugan-core/domain"
)

// Definition is the intentionally small, portable Community rule DSL.
// Enterprise-only signals and aggregate analytics are not part of this package.
type Definition struct {
	All     []Definition    `json:"all,omitempty"`
	Any     []Definition    `json:"any,omitempty"`
	Field   string          `json:"field,omitempty"`
	Op      string          `json:"op,omitempty"`
	Value   any             `json:"value,omitempty"`
	Outcome domain.Decision `json:"outcome,omitempty"`
	Score   float64         `json:"score,omitempty"`
	Reason  string          `json:"reason,omitempty"`
}

func Evaluate(rule domain.Rule, req domain.TransferCheckRequest) (domain.RuleHit, bool, error) {
	var definition Definition
	if err := json.Unmarshal(rule.Definition, &definition); err != nil {
		return domain.RuleHit{}, false, fmt.Errorf("invalid rule definition: %w", err)
	}
	if err := validateDefinition(definition); err != nil {
		return domain.RuleHit{}, false, fmt.Errorf("invalid rule definition: %w", err)
	}
	matched, err := evaluate(definition, req)
	if err != nil || !matched {
		return domain.RuleHit{}, matched, err
	}
	reason := definition.Reason
	if reason == "" {
		reason = rule.Name
	}
	return domain.RuleHit{RuleID: rule.ID, RuleName: rule.Name, Outcome: definition.Outcome, Score: definition.Score, Reason: reason}, true, nil
}

// Validate checks a rule before it is installed in a long-running process.
func Validate(rule domain.Rule) error {
	var definition Definition
	if err := json.Unmarshal(rule.Definition, &definition); err != nil {
		return fmt.Errorf("invalid rule definition: %w", err)
	}
	if err := validateDefinition(definition); err != nil {
		return fmt.Errorf("invalid rule definition: %w", err)
	}
	return nil
}

func validateDefinition(def Definition) error {
	switch def.Outcome {
	case domain.DecisionAllow, domain.DecisionReview, domain.DecisionBlock:
	default:
		return fmt.Errorf("unsupported outcome %q", def.Outcome)
	}
	condition := def
	condition.Outcome = ""
	return validateCondition(condition)
}

func validateCondition(def Definition) error {
	if len(def.All) > 0 && len(def.Any) > 0 {
		return fmt.Errorf("all and any cannot be combined")
	}
	children := def.All
	if len(children) == 0 {
		children = def.Any
	}
	if len(children) > 0 {
		for _, child := range children {
			if err := validateCondition(child); err != nil {
				return err
			}
		}
		return nil
	}
	actual, ok := fieldValue(def.Field, domain.TransferCheckRequest{})
	if !ok {
		return fmt.Errorf("unsupported rule field %q", def.Field)
	}
	if _, err := compare(actual, def.Op, def.Value); err != nil {
		return err
	}
	return nil
}

func evaluate(def Definition, req domain.TransferCheckRequest) (bool, error) {
	if len(def.All) > 0 {
		for _, child := range def.All {
			matched, err := evaluate(child, req)
			if err != nil || !matched {
				return matched, err
			}
		}
		return true, nil
	}
	if len(def.Any) > 0 {
		for _, child := range def.Any {
			matched, err := evaluate(child, req)
			if err != nil {
				return false, err
			}
			if matched {
				return true, nil
			}
		}
		return false, nil
	}
	actual, ok := fieldValue(def.Field, req)
	if !ok {
		return false, fmt.Errorf("unsupported rule field %q", def.Field)
	}
	return compare(actual, def.Op, def.Value)
}

func fieldValue(field string, req domain.TransferCheckRequest) (any, bool) {
	p := req.Payload
	switch strings.ToLower(field) {
	case "message_type":
		return req.MessageType, true
	case "amount", "instructed_amount.amount":
		return p.InstructedAmount.Amount, true
	case "currency", "instructed_amount.currency":
		return p.InstructedAmount.Currency, true
	case "debtor_account.id":
		return p.DebtorAccount.ID, true
	case "creditor_account.id":
		return p.CreditorAccount.ID, true
	case "debtor_agent.bicfi":
		return p.DebtorAgent.BICFI, true
	case "creditor_agent.bicfi":
		return p.CreditorAgent.BICFI, true
	case "remittance_information":
		return p.RemittanceInformation, true
	default:
		return nil, false
	}
}

func compare(actual any, op string, expected any) (bool, error) {
	switch value := actual.(type) {
	case float64:
		want, err := strconv.ParseFloat(fmt.Sprint(expected), 64)
		if err != nil {
			return false, fmt.Errorf("expected numeric value: %w", err)
		}
		switch op {
		case "gt":
			return value > want, nil
		case "gte":
			return value >= want, nil
		case "lt":
			return value < want, nil
		case "lte":
			return value <= want, nil
		case "eq":
			return value == want, nil
		default:
			return false, fmt.Errorf("unsupported number op %q", op)
		}
	case string:
		want := fmt.Sprint(expected)
		switch op {
		case "eq":
			return strings.EqualFold(value, want), nil
		case "neq":
			return !strings.EqualFold(value, want), nil
		case "contains":
			return strings.Contains(strings.ToLower(value), strings.ToLower(want)), nil
		case "not_contains":
			return !strings.Contains(strings.ToLower(value), strings.ToLower(want)), nil
		case "in", "not_in":
			matched := false
			if items, ok := expected.([]any); ok {
				for _, item := range items {
					matched = matched || strings.EqualFold(value, fmt.Sprint(item))
				}
			} else {
				matched = strings.EqualFold(value, want)
			}
			if op == "not_in" {
				return !matched, nil
			}
			return matched, nil
		default:
			return false, fmt.Errorf("unsupported string op %q", op)
		}
	default:
		return false, fmt.Errorf("unsupported value type %T", actual)
	}
}
