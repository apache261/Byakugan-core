package rules

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/apache261/Byakugan-core/domain"
)

// Definition is the portable Byakugan rule DSL. Window fields apply to
// velocity, beneficiary-history, and account-change signals.
type Definition struct {
	All           []Definition    `json:"all,omitempty"`
	Any           []Definition    `json:"any,omitempty"`
	Field         string          `json:"field,omitempty"`
	Op            string          `json:"op,omitempty"`
	Value         any             `json:"value,omitempty"`
	Outcome       domain.Decision `json:"outcome,omitempty"`
	Score         float64         `json:"score,omitempty"`
	Reason        string          `json:"reason,omitempty"`
	WindowMinutes int             `json:"window_minutes,omitempty"`
	WindowHours   int             `json:"window_hours,omitempty"`
}

// AccountContext supplies portable account attributes without prescribing an
// account database or storage model.
type AccountContext struct {
	RiskTier  string
	UpdatedAt time.Time
}

// Metrics contains pre-aggregated history for high-volume integrations.
// Loaded distinguishes a real zero from metrics that were not supplied.
type Metrics struct {
	Loaded                    bool
	DebtorTransferCount       int
	DebtorAmountSum           float64
	CreditorAmountSum         float64
	DebtorUniqueCreditorCount int
	DebtorRepeatedAmountCount int
	HasPreviousCreditor       bool
}

// Context supplies optional historical and account data to contextual rules.
// Callers can provide History for simple deployments or MetricsByWindow for
// database-backed deployments; no persistence implementation is required.
type Context struct {
	History         []domain.TransferCheck
	Metrics         Metrics
	MetricsByWindow map[time.Duration]Metrics
	DebtorAccount   AccountContext
	CreditorAccount AccountContext
	Now             time.Time
}

func Evaluate(rule domain.Rule, req domain.TransferCheckRequest) (domain.RuleHit, bool, error) {
	return EvaluateWithContext(rule, req, Context{})
}

// EvaluateWithContext evaluates scalar and history-aware portable rules.
func EvaluateWithContext(rule domain.Rule, req domain.TransferCheckRequest, context Context) (domain.RuleHit, bool, error) {
	definition, err := decodeDefinition(rule.Definition)
	if err != nil {
		return domain.RuleHit{}, false, fmt.Errorf("invalid rule definition: %w", err)
	}
	if err := validateDefinition(definition); err != nil {
		return domain.RuleHit{}, false, fmt.Errorf("invalid rule definition: %w", err)
	}
	if context.Now.IsZero() {
		context.Now = time.Now().UTC()
	}
	matched, err := evaluate(definition, req, context)
	if err != nil || !matched {
		return domain.RuleHit{}, matched, err
	}
	reason := definition.Reason
	if reason == "" {
		reason = rule.Name
	}
	return domain.RuleHit{RuleID: rule.ID, RuleName: rule.Name, Outcome: definition.Outcome, Score: definition.Score, Reason: reason}, true, nil
}

// MetricWindowsFromRules returns the distinct aggregate windows required by a
// rule set. Integrations can use it to issue bounded database queries.
func MetricWindowsFromRules(ruleList []domain.Rule) []time.Duration {
	seen := map[time.Duration]bool{}
	for _, rule := range ruleList {
		definition, err := decodeDefinition(rule.Definition)
		if err != nil {
			continue
		}
		collectMetricWindows(definition, seen)
	}
	windows := make([]time.Duration, 0, len(seen))
	for window := range seen {
		windows = append(windows, window)
	}
	sort.Slice(windows, func(i, j int) bool { return windows[i] < windows[j] })
	return windows
}

// Validate checks a rule before it is installed in a long-running process.
func Validate(rule domain.Rule) error {
	definition, err := decodeDefinition(rule.Definition)
	if err != nil {
		return fmt.Errorf("invalid rule definition: %w", err)
	}
	if err := validateDefinition(definition); err != nil {
		return fmt.Errorf("invalid rule definition: %w", err)
	}
	return nil
}

func decodeDefinition(raw json.RawMessage) (Definition, error) {
	var definition Definition
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&definition); err != nil {
		return Definition{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return Definition{}, fmt.Errorf("multiple JSON values")
		}
		return Definition{}, err
	}
	return definition, nil
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
	if def.WindowMinutes < 0 || def.WindowHours < 0 {
		return fmt.Errorf("rule windows cannot be negative")
	}
	if def.WindowMinutes > 0 && def.WindowHours > 0 {
		return fmt.Errorf("window_minutes and window_hours cannot be combined")
	}
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
	actual, ok := fieldValue(def, domain.TransferCheckRequest{}, Context{})
	if !ok {
		return fmt.Errorf("unsupported rule field %q", def.Field)
	}
	if _, err := compare(actual, def.Op, def.Value); err != nil {
		return err
	}
	return nil
}

func evaluate(def Definition, req domain.TransferCheckRequest, context Context) (bool, error) {
	if len(def.All) > 0 {
		for _, child := range def.All {
			matched, err := evaluate(child, req, context)
			if err != nil || !matched {
				return matched, err
			}
		}
		return true, nil
	}
	if len(def.Any) > 0 {
		for _, child := range def.Any {
			matched, err := evaluate(child, req, context)
			if err != nil {
				return false, err
			}
			if matched {
				return true, nil
			}
		}
		return false, nil
	}
	actual, ok := fieldValue(def, req, context)
	if !ok {
		return false, fmt.Errorf("unsupported rule field %q", def.Field)
	}
	return compare(actual, def.Op, def.Value)
}

func fieldValue(def Definition, req domain.TransferCheckRequest, context Context) (any, bool) {
	p := req.Payload
	switch strings.ToLower(def.Field) {
	case "message_type":
		return req.MessageType, true
	case "idempotency_key":
		return req.IdempotencyKey, true
	case "payment_identification.instruction_id", "instruction_id":
		return p.PaymentIdentification.InstructionID, true
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
	case "requested_execution_date":
		return p.RequestedExecutionDate, true
	case "requested_execution_days_ahead":
		return requestedExecutionDaysAhead(p.RequestedExecutionDate, context.Now), true
	case "debtor_agent.country":
		return bicCountry(p.DebtorAgent.BICFI), true
	case "creditor_agent.country":
		return bicCountry(p.CreditorAgent.BICFI), true
	case "route.country_pair":
		return bicCountry(p.DebtorAgent.BICFI) + "-" + bicCountry(p.CreditorAgent.BICFI), true
	case "debtor_account.risk_tier":
		return context.DebtorAccount.RiskTier, true
	case "creditor_account.risk_tier":
		return context.CreditorAccount.RiskTier, true
	case "velocity.debtor_transfer_count":
		if metrics, ok := metricsForWindow(context, def); ok {
			return float64(metrics.DebtorTransferCount), true
		}
		return float64(len(historyWindow(req, context, def))), true
	case "velocity.debtor_amount_sum":
		if metrics, ok := metricsForWindow(context, def); ok {
			return metrics.DebtorAmountSum, true
		}
		return amountSum(req, historyWindow(req, context, def)), true
	case "velocity.debtor_amount_sum_including_current":
		if metrics, ok := metricsForWindow(context, def); ok {
			return metrics.DebtorAmountSum + p.InstructedAmount.Amount, true
		}
		return amountSum(req, historyWindow(req, context, def)) + p.InstructedAmount.Amount, true
	case "velocity.creditor_amount_sum_including_current":
		if metrics, ok := metricsForWindow(context, def); ok {
			return metrics.CreditorAmountSum + p.InstructedAmount.Amount, true
		}
		return amountSum(req, creditorHistoryWindow(req, context, def)) + p.InstructedAmount.Amount, true
	case "velocity.debtor_unique_creditor_count":
		if metrics, ok := metricsForWindow(context, def); ok {
			return float64(metrics.DebtorUniqueCreditorCount), true
		}
		return float64(uniqueCreditors(req, historyWindow(req, context, def))), true
	case "velocity.debtor_repeated_amount_count":
		if metrics, ok := metricsForWindow(context, def); ok {
			return float64(metrics.DebtorRepeatedAmountCount), true
		}
		return float64(repeatedAmountCount(req, historyWindow(req, context, def))), true
	case "beneficiary.first_time_creditor", "first_time_creditor":
		if metrics, ok := metricsForWindow(context, def); ok {
			return !metrics.HasPreviousCreditor, true
		}
		if context.Metrics.Loaded {
			return !context.Metrics.HasPreviousCreditor, true
		}
		return !hasPreviousCreditor(req, context.History), true
	case "account.debtor_metadata_recently_changed":
		return recentlyChanged(context.DebtorAccount.UpdatedAt, context.Now, def), true
	case "account.creditor_metadata_recently_changed":
		return recentlyChanged(context.CreditorAccount.UpdatedAt, context.Now, def), true
	default:
		return nil, false
	}
}

func compare(actual any, op string, expected any) (bool, error) {
	switch value := actual.(type) {
	case bool:
		want, err := boolFrom(expected)
		if err != nil {
			return false, err
		}
		switch op {
		case "eq":
			return value == want, nil
		case "neq":
			return value != want, nil
		default:
			return false, fmt.Errorf("unsupported bool op %q", op)
		}
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

func boolFrom(value any) (bool, error) {
	switch typed := value.(type) {
	case bool:
		return typed, nil
	case string:
		return strconv.ParseBool(strings.ToLower(strings.TrimSpace(typed)))
	default:
		return false, fmt.Errorf("expected boolean value, got %T", value)
	}
}

func collectMetricWindows(def Definition, seen map[time.Duration]bool) {
	for _, child := range def.All {
		collectMetricWindows(child, seen)
	}
	for _, child := range def.Any {
		collectMetricWindows(child, seen)
	}
	field := strings.ToLower(def.Field)
	if strings.HasPrefix(field, "velocity.") || field == "beneficiary.first_time_creditor" || field == "first_time_creditor" {
		seen[metricWindow(def)] = true
	}
}

func metricsForWindow(context Context, def Definition) (Metrics, bool) {
	if len(context.MetricsByWindow) > 0 {
		metrics, ok := context.MetricsByWindow[metricWindow(def)]
		if ok {
			return metrics, true
		}
	}
	if context.Metrics.Loaded {
		return context.Metrics, true
	}
	return Metrics{}, false
}

func metricWindow(def Definition) time.Duration {
	if def.WindowMinutes > 0 {
		return time.Duration(def.WindowMinutes) * time.Minute
	}
	if def.WindowHours > 0 {
		return time.Duration(def.WindowHours) * time.Hour
	}
	return 24 * time.Hour
}

func historyWindow(req domain.TransferCheckRequest, context Context, def Definition) []domain.TransferCheck {
	cutoff := windowCutoff(context.Now, def)
	result := make([]domain.TransferCheck, 0, len(context.History))
	for _, check := range context.History {
		if !cutoff.IsZero() && check.CreatedAt.Before(cutoff) {
			continue
		}
		if check.Request.Payload.DebtorAccount.ID == req.Payload.DebtorAccount.ID {
			result = append(result, check)
		}
	}
	return result
}

func creditorHistoryWindow(req domain.TransferCheckRequest, context Context, def Definition) []domain.TransferCheck {
	cutoff := windowCutoff(context.Now, def)
	result := make([]domain.TransferCheck, 0, len(context.History))
	for _, check := range context.History {
		if !cutoff.IsZero() && check.CreatedAt.Before(cutoff) {
			continue
		}
		if check.Request.Payload.CreditorAccount.ID == req.Payload.CreditorAccount.ID {
			result = append(result, check)
		}
	}
	return result
}

func windowCutoff(now time.Time, def Definition) time.Time {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if def.WindowMinutes > 0 {
		return now.Add(-time.Duration(def.WindowMinutes) * time.Minute)
	}
	if def.WindowHours > 0 {
		return now.Add(-time.Duration(def.WindowHours) * time.Hour)
	}
	return time.Time{}
}

func amountSum(req domain.TransferCheckRequest, checks []domain.TransferCheck) float64 {
	var total float64
	for _, check := range checks {
		if strings.EqualFold(check.Request.Payload.InstructedAmount.Currency, req.Payload.InstructedAmount.Currency) {
			total += check.Request.Payload.InstructedAmount.Amount
		}
	}
	return total
}

func uniqueCreditors(req domain.TransferCheckRequest, checks []domain.TransferCheck) int {
	seen := map[string]bool{}
	for _, check := range checks {
		if creditor := strings.TrimSpace(check.Request.Payload.CreditorAccount.ID); creditor != "" {
			seen[creditor] = true
		}
	}
	if creditor := strings.TrimSpace(req.Payload.CreditorAccount.ID); creditor != "" {
		seen[creditor] = true
	}
	return len(seen)
}

func repeatedAmountCount(req domain.TransferCheckRequest, checks []domain.TransferCheck) int {
	count := 0
	for _, check := range checks {
		amount := check.Request.Payload.InstructedAmount
		if amount.Amount == req.Payload.InstructedAmount.Amount && strings.EqualFold(amount.Currency, req.Payload.InstructedAmount.Currency) {
			count++
		}
	}
	return count
}

func hasPreviousCreditor(req domain.TransferCheckRequest, checks []domain.TransferCheck) bool {
	for _, check := range checks {
		payload := check.Request.Payload
		if payload.DebtorAccount.ID == req.Payload.DebtorAccount.ID && payload.CreditorAccount.ID == req.Payload.CreditorAccount.ID {
			return true
		}
	}
	return false
}

func recentlyChanged(updatedAt, now time.Time, def Definition) bool {
	if updatedAt.IsZero() {
		return false
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	cutoff := windowCutoff(now, def)
	if cutoff.IsZero() {
		cutoff = now.Add(-24 * time.Hour)
	}
	return updatedAt.After(cutoff)
}

func bicCountry(bic string) string {
	value := strings.ToUpper(strings.TrimSpace(bic))
	if len(value) < 6 {
		return ""
	}
	return value[4:6]
}

func requestedExecutionDaysAhead(value string, now time.Time) float64 {
	if strings.TrimSpace(value) == "" {
		return 0
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	base := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return parsed.Sub(base).Hours() / 24
}
