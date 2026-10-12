package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"
)

type ScenarioVersion struct {
	ID                    string       `json:"id"`
	ScenarioID            string       `json:"scenario_id"`
	Version               int          `json:"version"`
	SchemaVersion         int          `json:"schema_version"`
	Typology              string       `json:"typology"`
	Description           string       `json:"description,omitempty"`
	ExecutionModes        []string     `json:"execution_modes"`
	SubjectBasis          SubjectType  `json:"subject_basis"`
	WindowSeconds         int64        `json:"window_seconds"`
	LateDataBehavior      string       `json:"late_data_behavior"`
	Condition             ScenarioNode `json:"condition"`
	EvidenceLabels        []string     `json:"evidence_labels,omitempty"`
	ReasonTemplate        string       `json:"reason_template"`
	Severity              string       `json:"severity"`
	RiskScore             Decimal      `json:"risk_score"`
	PolicyInputs          []string     `json:"policy_inputs,omitempty"`
	MinimumHistorySeconds int64        `json:"minimum_history_seconds,omitempty"`
	Author                string       `json:"author"`
	ContentHash           string       `json:"content_hash"`
	CreatedAt             time.Time    `json:"created_at"`
}

type ScenarioNode struct {
	All           []ScenarioNode      `json:"all,omitempty"`
	Any           []ScenarioNode      `json:"any,omitempty"`
	Not           *ScenarioNode       `json:"not,omitempty"`
	Field         string              `json:"field,omitempty"`
	Metric        string              `json:"metric,omitempty"`
	NetworkMetric string              `json:"network_metric,omitempty"`
	Operator      string              `json:"operator,omitempty"`
	Value         json.RawMessage     `json:"value,omitempty"`
	PolicyRef     string              `json:"policy_ref,omitempty"`
	SubjectBasis  SubjectType         `json:"subject_basis,omitempty"`
	WindowSeconds int64               `json:"window_seconds,omitempty"`
	Screening     *ScreeningPredicate `json:"screening,omitempty"`
}

type ScreeningPredicate struct {
	SubjectBasis SubjectType `json:"subject_basis"`
	Kind         string      `json:"kind"`
	DatasetRef   string      `json:"dataset_ref,omitempty"`
	Disposition  string      `json:"disposition,omitempty"`
	MinimumScore Decimal     `json:"minimum_score,omitempty"`
}

var hashPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func (s ScenarioVersion) Validate() error {
	if err := requiredBounded("scenario_id", s.ScenarioID, 200); err != nil {
		return err
	}
	if s.Version < 1 || s.SchemaVersion != 1 {
		return fmt.Errorf("unsupported scenario schema/version")
	}
	if err := requiredBounded("typology", s.Typology, 100); err != nil {
		return err
	}
	if !validSubjectType(s.SubjectBasis) {
		return fmt.Errorf("invalid subject_basis")
	}
	if s.WindowSeconds < 1 || s.WindowSeconds > int64((366*24*time.Hour)/time.Second) {
		return fmt.Errorf("window_seconds is out of range")
	}
	if !oneOf(s.LateDataBehavior, "reconcile", "ignore", "observe") {
		return fmt.Errorf("invalid late_data_behavior")
	}
	if len(s.ExecutionModes) == 0 || len(s.ExecutionModes) > 3 {
		return fmt.Errorf("execution_modes must contain 1 to 3 values")
	}
	for _, mode := range s.ExecutionModes {
		if !oneOf(mode, "realtime", "scheduled", "simulation") {
			return fmt.Errorf("invalid execution mode")
		}
	}
	if err := validateScenarioNode(s.Condition, 1, new(int)); err != nil {
		return err
	}
	if scenarioContains(s.ExecutionModes, "realtime") && !s.RealtimeSafe() {
		return fmt.Errorf("realtime scenarios may contain only transaction scalar predicates")
	}
	if !oneOf(s.Severity, "low", "medium", "high", "critical") {
		return fmt.Errorf("invalid severity")
	}
	if err := s.RiskScore.Validate(); err != nil || decimalGreaterThan(s.RiskScore, "100") {
		return fmt.Errorf("risk_score must be an exact value from 0 through 100")
	}
	if err := requiredBounded("author", s.Author, 200); err != nil {
		return err
	}
	if len(s.Description) > 4000 || len(s.ReasonTemplate) > 1000 {
		return fmt.Errorf("scenario description or reason template exceeds limits")
	}
	for _, match := range regexp.MustCompile(`\{\{([^{}]+)\}\}`).FindAllStringSubmatch(s.ReasonTemplate, -1) {
		if !oneOf(match[1], "left", "operator", "right", "screening") {
			return fmt.Errorf("reason template contains unsupported evidence label")
		}
	}
	if s.ContentHash != "" && !hashPattern.MatchString(s.ContentHash) {
		return fmt.Errorf("invalid content_hash")
	}
	return nil
}

func (s ScenarioVersion) RealtimeSafe() bool {
	if validateScenarioNode(s.Condition, 1, new(int)) != nil {
		return false
	}
	return realtimeSafeNode(s.Condition)
}

func realtimeSafeNode(node ScenarioNode) bool {
	if node.Metric != "" || node.NetworkMetric != "" || node.Screening != nil {
		return false
	}
	for _, child := range node.All {
		if !realtimeSafeNode(child) {
			return false
		}
	}
	for _, child := range node.Any {
		if !realtimeSafeNode(child) {
			return false
		}
	}
	return node.Not == nil || realtimeSafeNode(*node.Not)
}

func scenarioContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func validateScenarioNode(node ScenarioNode, depth int, count *int) error {
	*count++
	if depth > 12 || *count > 200 {
		return fmt.Errorf("scenario condition exceeds complexity limits")
	}
	kinds := 0
	if node.All != nil {
		kinds++
	}
	if node.Any != nil {
		kinds++
	}
	if node.Not != nil {
		kinds++
	}
	if node.Field != "" {
		kinds++
	}
	if node.Metric != "" {
		kinds++
	}
	if node.NetworkMetric != "" {
		kinds++
	}
	if node.Screening != nil {
		kinds++
	}
	if kinds != 1 {
		return fmt.Errorf("each scenario node must contain exactly one node family")
	}
	children := node.All
	if node.Any != nil {
		children = node.Any
	}
	if node.All != nil || node.Any != nil {
		if len(children) == 0 || len(children) > 50 {
			return fmt.Errorf("boolean node must contain 1 to 50 children")
		}
		for i := range children {
			if err := validateScenarioNode(children[i], depth+1, count); err != nil {
				return err
			}
		}
		return nil
	}
	if node.Not != nil {
		return validateScenarioNode(*node.Not, depth+1, count)
	}
	if node.Screening != nil {
		if !validSubjectType(node.Screening.SubjectBasis) || !oneOf(node.Screening.Kind, "sanctions", "pep", "adverse_media", "internal_watchlist") {
			return fmt.Errorf("invalid screening predicate")
		}

		if node.Screening.MinimumScore != "" {
			if err := node.Screening.MinimumScore.Validate(); err != nil || decimalGreaterThan(node.Screening.MinimumScore, "100") {
				return fmt.Errorf("screening minimum_score must be an exact value from 0 through 100")
			}
		}
		return nil
	}
	if !oneOf(node.Operator, "eq", "neq", "in", "not_in", "contains", "starts_with", "exists", "gt", "gte", "lt", "lte") {
		return fmt.Errorf("unsupported operator")
	}
	if (len(node.Value) == 0) == (node.PolicyRef == "") && node.Operator != "exists" {
		return fmt.Errorf("predicate requires exactly one value or policy_ref")
	}
	if len(node.Value) > 0 {
		var value any
		decoder := json.NewDecoder(bytes.NewReader(node.Value))
		decoder.UseNumber()
		if err := decoder.Decode(&value); err != nil || !validScenarioValue(value) {
			return fmt.Errorf("predicate value must be a bounded scalar or scalar array")
		}
		var trailing any
		if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
			return fmt.Errorf("predicate value must contain exactly one JSON value")
		}
		if oneOf(node.Operator, "gt", "gte", "lt", "lte") {
			decimal, ok := value.(string)
			if !ok {
				return fmt.Errorf("numeric operand must be an exact decimal string")
			}
			if _, err := ParseDecimal(decimal); err != nil {
				return err
			}
		}
	}
	if node.Field != "" && !oneOf(node.Field, "transaction.kind", "transaction.direction", "transaction.status", "transaction.amount", "transaction.asset_code", "transaction.channel", "transaction.origin_country", "transaction.destination_country", "customer.risk_rating", "customer.industry", "profile.expected_amount", "profile.expected_count") {
		return fmt.Errorf("unsupported field %q", node.Field)
	}
	if node.Field != "" {
		numeric := oneOf(node.Field, "transaction.amount", "profile.expected_amount", "profile.expected_count")
		if numeric && !oneOf(node.Operator, "eq", "neq", "exists", "gt", "gte", "lt", "lte") {
			return fmt.Errorf("operator is not valid for numeric field")
		}
		if !numeric && !oneOf(node.Operator, "eq", "neq", "in", "not_in", "contains", "starts_with", "exists") {
			return fmt.Errorf("operator is not valid for string field")
		}
	}
	if node.Metric != "" && !oneOf(node.Metric, "count", "sum", "distinct_count", "repeated_value_count", "first_seen_age", "active_span") {
		return fmt.Errorf("unsupported aggregate metric")
	}
	if node.NetworkMetric != "" && !oneOf(node.NetworkMetric, "fan_in", "fan_out", "shared_counterparty", "circular_flow", "pass_through") {
		return fmt.Errorf("unsupported network metric")
	}
	if node.Metric != "" || node.NetworkMetric != "" {
		if !validSubjectType(node.SubjectBasis) || node.WindowSeconds < 1 || node.WindowSeconds > int64((366*24*time.Hour)/time.Second) {
			return fmt.Errorf("aggregate predicate requires valid subject_basis and bounded window_seconds")
		}
	}
	return nil
}

func PrepareScenarioVersion(value ScenarioVersion) (ScenarioVersion, error) {
	value.CreatedAt = value.CreatedAt.UTC()
	value.PolicyInputs = uniqueSorted(value.PolicyInputs)
	value.EvidenceLabels = uniqueSorted(value.EvidenceLabels)
	value.ContentHash = ""
	if err := value.Validate(); err != nil {
		return value, err
	}
	value.ContentHash = fingerprint(value)
	return value, nil
}

func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			seen[value] = true
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func validScenarioValue(value any) bool {
	switch typed := value.(type) {
	case string:
		return len(typed) <= 1000
	case bool, nil:
		return true
	case []any:
		if len(typed) > 100 {
			return false
		}
		for _, item := range typed {
			if !validScenarioValue(item) {
				return false
			}
		}
		return true
	default:
		return false
	}
}
