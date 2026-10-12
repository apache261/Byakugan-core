// Package scenarios implements the pure, bounded AML scenario DSL. It has no
// persistence, network, fraud-decision, or alert-writing capability.
package scenarios

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"sort"
	"strings"

	"github.com/apache261/Byakugan-core/aml/domain"
)

type Input struct {
	Transaction  domain.Transaction         `json:"transaction"`
	Facts        map[string]json.RawMessage `json:"facts,omitempty"`
	Metrics      map[string]domain.Decimal  `json:"metrics,omitempty"`
	PolicyValues map[string]json.RawMessage `json:"policy_values,omitempty"`
	Screening    map[string]bool            `json:"screening,omitempty"`
	EvidenceIDs  []string                   `json:"evidence_ids,omitempty"`
	LateData     bool                       `json:"late_data,omitempty"`
}

type Observation struct {
	Matched          bool              `json:"matched"`
	InsufficientData bool              `json:"insufficient_data"`
	ScenarioID       string            `json:"scenario_id"`
	ScenarioVersion  int               `json:"scenario_version"`
	Typology         string            `json:"typology"`
	Severity         string            `json:"severity"`
	RiskScore        domain.Decimal    `json:"risk_score"`
	Reasons          []string          `json:"reasons"`
	EvidenceIDs      []string          `json:"evidence_transaction_ids,omitempty"`
	Values           map[string]string `json:"values,omitempty"`
}

func Evaluate(version domain.ScenarioVersion, input Input) (Observation, error) {
	return EvaluateContext(context.Background(), version, input)
}

// EvaluateContext is pure and cooperatively checks cancellation at every DSL node.
func EvaluateContext(ctx context.Context, version domain.ScenarioVersion, input Input) (Observation, error) {
	if err := ctx.Err(); err != nil {
		return Observation{}, err
	}
	prepared, err := domain.PrepareScenarioVersion(version)
	if err != nil {
		return Observation{}, err
	}
	if err := ctx.Err(); err != nil {
		return Observation{}, err
	}
	state := evaluation{ctx: ctx, input: input, values: map[string]string{}}
	if input.LateData && prepared.LateDataBehavior == "ignore" {
		return Observation{Matched: false, ScenarioID: prepared.ScenarioID, ScenarioVersion: prepared.Version, Typology: prepared.Typology, Severity: prepared.Severity, RiskScore: prepared.RiskScore, Reasons: []string{"late data ignored by scenario version"}, Values: map[string]string{"late_data": "true"}}, nil
	}
	matched, known, err := state.node(prepared.Condition)
	if cancelErr := ctx.Err(); cancelErr != nil {
		return Observation{}, cancelErr
	}
	if err != nil {
		return Observation{}, err
	}
	reasons := []string{}
	if !known {
		reasons = append(reasons, "required scenario input is unavailable")
	} else if matched {
		reasons = append(reasons, renderReason(prepared.ReasonTemplate, state.values))
	}
	evidence := append([]string(nil), input.EvidenceIDs...)
	sort.Strings(evidence)
	return Observation{Matched: known && matched, InsufficientData: !known, ScenarioID: prepared.ScenarioID, ScenarioVersion: prepared.Version, Typology: prepared.Typology, Severity: prepared.Severity, RiskScore: prepared.RiskScore, Reasons: reasons, EvidenceIDs: evidence, Values: state.values}, nil
}

type evaluation struct {
	ctx    context.Context
	input  Input
	values map[string]string
}

func (e *evaluation) node(node domain.ScenarioNode) (bool, bool, error) {
	if err := e.ctx.Err(); err != nil {
		return false, false, err
	}
	if node.All != nil {
		for _, child := range node.All {
			matched, known, err := e.node(child)
			if err != nil || !known || !matched {
				return matched, known, err
			}
		}
		return true, true, nil
	}
	if node.Any != nil {
		knownOne := false
		for _, child := range node.Any {
			matched, known, err := e.node(child)
			if err != nil {
				return false, false, err
			}
			knownOne = knownOne || known
			if known && matched {
				return true, true, nil
			}
		}
		return false, knownOne, nil
	}
	if node.Not != nil {
		matched, known, err := e.node(*node.Not)
		return !matched, known, err
	}
	if node.Screening != nil {
		key := string(node.Screening.SubjectBasis) + ":" + node.Screening.Kind + ":" + node.Screening.DatasetRef + ":" + node.Screening.Disposition
		value, ok := e.input.Screening[key]
		e.values["screening"] = key
		return value, ok, nil
	}
	left, ok, err := e.resolveLeft(node)
	if err != nil {
		return false, false, err
	}
	if !ok {
		return false, false, nil
	}
	right := node.Value
	if node.PolicyRef != "" {
		right, ok = e.input.PolicyValues[node.PolicyRef]
		if !ok {
			return false, false, nil
		}
	}
	e.values["left"] = rawString(left)
	e.values["operator"] = node.Operator
	e.values["right"] = rawString(right)
	return compare(left, node.Operator, right)
}

func (e *evaluation) resolveLeft(node domain.ScenarioNode) (json.RawMessage, bool, error) {
	key := node.Field
	if node.Metric != "" {
		key = node.Metric + ":" + string(node.SubjectBasis) + ":" + fmt.Sprint(node.WindowSeconds)
	}
	if node.NetworkMetric != "" {
		key = node.NetworkMetric + ":" + string(node.SubjectBasis) + ":" + fmt.Sprint(node.WindowSeconds)
	}
	if metric, ok := e.input.Metrics[key]; ok {
		if err := metric.Validate(); err != nil {
			return nil, false, err
		}
		raw, err := json.Marshal(metric.String())
		return raw, true, err
	}
	if fact, ok := e.input.Facts[key]; ok {
		return fact, true, nil
	}
	var value any
	switch node.Field {
	case "transaction.kind":
		value = e.input.Transaction.Kind
	case "transaction.direction":
		value = e.input.Transaction.Direction
	case "transaction.status":
		value = e.input.Transaction.Status
	case "transaction.amount":
		if e.input.Transaction.Money.Amount == "" {
			return nil, false, nil
		}
		if err := e.input.Transaction.Money.Amount.Validate(); err != nil {
			return nil, false, err
		}
		value = e.input.Transaction.Money.Amount.String()
	case "transaction.asset_code":
		value = e.input.Transaction.Money.AssetCode
	case "transaction.channel":
		value = e.input.Transaction.Channel
	case "transaction.origin_country":
		value = e.input.Transaction.OriginCountry
	case "transaction.destination_country":
		value = e.input.Transaction.DestinationCountry
	default:
		return nil, false, nil
	}
	raw, _ := json.Marshal(value)
	return raw, true, nil
}

func compare(left json.RawMessage, operator string, right json.RawMessage) (bool, bool, error) {
	if operator == "exists" {
		var value any
		if err := decode(left, &value); err != nil {
			return false, false, err
		}
		return value != nil, true, nil
	}
	var l, r any
	if err := decode(left, &l); err != nil {
		return false, false, err
	}
	if err := decode(right, &r); err != nil {
		return false, false, err
	}
	ls := fmt.Sprint(l)
	switch operator {
	case "eq", "neq":
		result := canonical(l) == canonical(r)
		if operator == "neq" {
			result = !result
		}
		return result, true, nil
	case "contains":
		return strings.Contains(ls, fmt.Sprint(r)), true, nil
	case "starts_with":
		return strings.HasPrefix(ls, fmt.Sprint(r)), true, nil
	case "in", "not_in":
		items, ok := r.([]any)
		if !ok {
			return false, false, fmt.Errorf("in operator requires an array")
		}
		found := false
		for _, item := range items {
			if canonical(l) == canonical(item) {
				found = true
				break
			}
		}
		if operator == "not_in" {
			found = !found
		}
		return found, true, nil
	case "gt", "gte", "lt", "lte":
		leftDecimal, leftOK := l.(string)
		rightDecimal, rightOK := r.(string)
		if !leftOK || !rightOK {
			return false, false, fmt.Errorf("numeric operands must be exact decimal strings")
		}
		cmp, err := compareExact(leftDecimal, rightDecimal)
		if err != nil {
			return false, false, err
		}
		switch operator {
		case "gt":
			return cmp > 0, true, nil
		case "gte":
			return cmp >= 0, true, nil
		case "lt":
			return cmp < 0, true, nil
		default:
			return cmp <= 0, true, nil
		}
	default:
		return false, false, fmt.Errorf("unsupported operator")
	}
}

func decode(raw []byte, value *any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return fmt.Errorf("scenario value must contain exactly one JSON value")
	}
	return nil
}
func canonical(value any) string { raw, _ := json.Marshal(value); return string(raw) }
func rawString(raw []byte) string {
	var value any
	if decode(raw, &value) != nil {
		return ""
	}
	return fmt.Sprint(value)
}
func compareExact(left, right string) (int, error) {
	leftDecimal, err := domain.ParseDecimal(left)
	if err != nil {
		return 0, fmt.Errorf("left operand is not an exact decimal: %w", err)
	}
	rightDecimal, err := domain.ParseDecimal(right)
	if err != nil {
		return 0, fmt.Errorf("right operand is not an exact decimal: %w", err)
	}
	l, ok := new(big.Rat).SetString(leftDecimal.String())
	if !ok {
		return 0, fmt.Errorf("left operand is not an exact decimal")
	}
	r, ok := new(big.Rat).SetString(rightDecimal.String())
	if !ok {
		return 0, fmt.Errorf("right operand is not an exact decimal")
	}
	return l.Cmp(r), nil
}
func renderReason(template string, values map[string]string) string {
	if strings.TrimSpace(template) == "" {
		return "scenario conditions matched"
	}
	out := template
	for _, key := range []string{"left", "operator", "right", "screening"} {
		out = strings.ReplaceAll(out, "{{"+key+"}}", values[key])
	}
	return out
}
