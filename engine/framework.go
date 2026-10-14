package engine

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/apache261/Byakugan-core/domain"
	"github.com/apache261/Byakugan-core/rules"
)

// Evaluator consumes caller-supplied data. Implementations must be safe for
// concurrent calls and cooperate with cancellation. Core performs no I/O for them.
type Evaluator interface {
	Evaluate(context.Context, domain.TransferCheckRequest, Context) ([]domain.RuleHit, error)
}

type EvaluatorFunc func(context.Context, domain.TransferCheckRequest, Context) ([]domain.RuleHit, error)

func (f EvaluatorFunc) Evaluate(ctx context.Context, req domain.TransferCheckRequest, data Context) ([]domain.RuleHit, error) {
	return f(ctx, req, data)
}

type Registration struct {
	ID        string
	Priority  int
	Evaluator Evaluator
}

type FailurePolicy string

const (
	FailureReview FailurePolicy = "review"
	FailureBlock  FailurePolicy = "block"
)

// Policy controls aggregation only; it confers no operational authority.
// Every evaluator failure is also returned as an error, regardless of policy.
type Policy struct {
	Failure               FailurePolicy
	StopOnBlock           bool
	StopOnError           bool
	ReplaceReasonsOnError bool
	IncludeZeroThresholds bool
	AllowEmptyOutcome     bool // Legacy Full rules may contribute score without an outcome.
}

func DefaultPolicy() Policy { return Policy{Failure: FailureReview, StopOnError: true} }

// FullCompatibilityPolicy matches Full's ordered, first-block/first-error rule
// stage. Full supplies prefiltered rules and configured thresholds and performs
// account admission, durable replay and data retrieval before calling Core.
func FullCompatibilityPolicy() Policy {
	return Policy{Failure: FailureReview, StopOnBlock: true, StopOnError: true, ReplaceReasonsOnError: true, IncludeZeroThresholds: true, AllowEmptyOutcome: true}
}

// Result deliberately has no generated identity, replay key, history or storage.
type Result struct {
	PolicyVersionID string           `json:"policy_version_id,omitempty"`
	Decision        domain.Decision  `json:"decision"`
	Score           float64          `json:"score"`
	Reasons         []string         `json:"reasons"`
	RuleHits        []domain.RuleHit `json:"rule_hits"`
}

// Framework holds immutable registry snapshots, not transfer state. Its zero
// value is usable with the conservative review-on-error policy.
type Framework struct {
	mu            sync.RWMutex
	ruleSet       []domain.Rule
	registrations []Registration
	policy        Policy
}

func NewFramework(ruleSet []domain.Rule, registrations []Registration, policy Policy) (*Framework, error) {
	if policy.Failure == "" {
		policy.Failure = FailureReview
	}
	if policy.Failure != FailureReview && policy.Failure != FailureBlock {
		return nil, fmt.Errorf("unsupported failure policy %q", policy.Failure)
	}
	if err := validateRegistry(ruleSet, registrations); err != nil {
		return nil, err
	}
	return &Framework{ruleSet: cloneRules(ruleSet), registrations: append([]Registration(nil), registrations...), policy: policy}, nil
}

func validID(id string) bool {
	if id == "" || len(id) > 200 || strings.TrimSpace(id) != id {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("._:-", r)) {
			return false
		}
	}
	return true
}
func nilEvaluator(value Evaluator) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	}
	return false
}
func validateRegistry(ruleSet []domain.Rule, registrations []Registration) error {
	seen := map[string]bool{}
	for _, rule := range ruleSet {
		if !validID(rule.ID) || seen[rule.ID] {
			return fmt.Errorf("invalid or duplicate evaluator ID %q", rule.ID)
		}
		seen[rule.ID] = true
		if err := rules.Validate(rule); err != nil {
			return fmt.Errorf("rule %s: %w", rule.ID, err)
		}
	}
	for _, registration := range registrations {
		if !validID(registration.ID) || seen[registration.ID] || nilEvaluator(registration.Evaluator) {
			return fmt.Errorf("invalid or duplicate evaluator %q", registration.ID)
		}
		seen[registration.ID] = true
	}
	return nil
}

func (f *Framework) Register(registration Registration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	next := append(append([]Registration(nil), f.registrations...), registration)
	if err := validateRegistry(f.ruleSet, next); err != nil {
		return err
	}
	f.registrations = next
	return nil
}
func (f *Framework) ReplaceRules(ruleSet []domain.Rule) error {
	next := cloneRules(ruleSet)
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := validateRegistry(next, f.registrations); err != nil {
		return err
	}
	f.ruleSet = next
	return nil
}
func (f *Framework) Rules() []domain.Rule {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return cloneRules(f.ruleSet)
}
func cloneRules(ruleSet []domain.Rule) []domain.Rule {
	cloned := append([]domain.Rule(nil), ruleSet...)
	for i := range cloned {
		cloned[i].Definition = append([]byte(nil), cloned[i].Definition...)
	}
	return cloned
}

type evaluationStep struct {
	id        string
	priority  int
	rule      *domain.Rule
	evaluator Evaluator
}

func (f *Framework) snapshot() ([]evaluationStep, Policy) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	steps := make([]evaluationStep, 0, len(f.ruleSet)+len(f.registrations))
	ruleSet := cloneRules(f.ruleSet)
	for i := range ruleSet {
		steps = append(steps, evaluationStep{id: ruleSet[i].ID, priority: ruleSet[i].Priority, rule: &ruleSet[i]})
	}
	for _, registration := range f.registrations {
		steps = append(steps, evaluationStep{id: registration.ID, priority: registration.Priority, evaluator: registration.Evaluator})
	}
	sort.SliceStable(steps, func(i, j int) bool { return steps[i].priority < steps[j].priority })
	return steps, f.policy
}

// Evaluate never consults previous calls, even when idempotency keys repeat.
// Inputs are detached before callbacks; callers must not mutate inputs while
// Evaluate is taking its snapshot. Cancellation is cooperative, not preemptive.
func (f *Framework) Evaluate(ctx context.Context, req domain.TransferCheckRequest, data Context) (Result, error) {
	result := Result{PolicyVersionID: data.PolicyVersionID, Decision: domain.DecisionAllow, Reasons: []string{"no fraud signal detected"}, RuleHits: []domain.RuleHit{}}
	steps, policy := f.snapshot()
	var failures []error
	fail := func(err error) {
		failures = append(failures, err)
		decision := domain.DecisionReview
		if policy.Failure == FailureBlock {
			decision = domain.DecisionBlock
		}
		if result.Decision != domain.DecisionBlock {
			result.Decision = decision
		}
		if policy.ReplaceReasonsOnError {
			result.Reasons = []string{err.Error()}
		} else {
			result.Reasons = appendReason(result.Reasons, err.Error())
		}
	}
	if err := ctx.Err(); err != nil {
		fail(err)
		return result, err
	}
	if errs := domain.ValidateTransfer(req); len(errs) > 0 {
		err := errors.New(domain.ValidationReason(errs))
		fail(err)
		if data.ValidationFailureDecision == domain.DecisionBlock {
			result.Decision = domain.DecisionBlock
		}
		return result, err
	}
	for _, threshold := range []float64{data.ReviewThreshold, data.BlockThreshold} {
		if threshold < 0 || math.IsNaN(threshold) || math.IsInf(threshold, 0) {
			err := errors.New("thresholds must be finite and nonnegative")
			fail(err)
			return result, err
		}
	}
	if contains(data.BlockedAccountIDs, req.Payload.DebtorAccount.ID) || contains(data.BlockedAccountIDs, req.Payload.CreditorAccount.ID) {
		result.Decision = domain.DecisionBlock
		result.Reasons = []string{"debtor or creditor account is blocked"}
		return result, nil
	}
	data = cloneEvaluationContext(data)
	for _, step := range steps {
		if err := ctx.Err(); err != nil {
			fail(err)
			return result, errors.Join(failures...)
		}
		if contains(data.DisabledRuleIDs, step.id) || step.rule != nil && !step.rule.Enabled {
			continue
		}
		var hits []domain.RuleHit
		var err error
		if step.rule != nil {
			var hit domain.RuleHit
			var matched bool
			hit, matched, err = rules.EvaluateWithContext(*step.rule, req, data.Rules)
			if matched && err == nil {
				hits = []domain.RuleHit{hit}
			}
		} else {
			hits, err = step.evaluator.Evaluate(ctx, req, cloneEvaluationContext(data))
		}
		if cancelErr := ctx.Err(); cancelErr != nil {
			fail(cancelErr)
			return result, errors.Join(failures...)
		}
		if err == nil {
			err = validateHits(hits, result.Score, policy.AllowEmptyOutcome)
		}
		if err != nil {
			// Full compatibility exposes the original rule error as its sole reason.
			reason := fmt.Errorf("evaluator %s: %w", step.id, err)
			if policy.ReplaceReasonsOnError {
				fail(err)
			} else {
				fail(reason)
			}
			if policy.StopOnError {
				return result, errors.Join(failures...)
			}
			continue
		}
		for _, hit := range hits {
			result.RuleHits = append(result.RuleHits, hit)
			result.Score += hit.Score
			result.Reasons = appendReason(result.Reasons, hit.Reason)
			if hit.Outcome == domain.DecisionBlock || hit.Outcome == domain.DecisionReview && result.Decision == domain.DecisionAllow {
				result.Decision = hit.Outcome
			}
			if policy.StopOnBlock && result.Decision == domain.DecisionBlock {
				return result, errors.Join(failures...)
			}
		}
	}
	if err := ctx.Err(); err != nil {
		fail(err)
		return result, errors.Join(failures...)
	}
	if result.Decision != domain.DecisionBlock {
		if (data.BlockThreshold > 0 || policy.IncludeZeroThresholds) && result.Score >= data.BlockThreshold {
			result.Decision = domain.DecisionBlock
			result.Reasons = appendReason(result.Reasons, "score reached block threshold")
		} else if (data.ReviewThreshold > 0 || policy.IncludeZeroThresholds) && result.Score >= data.ReviewThreshold {
			result.Decision = domain.DecisionReview
			result.Reasons = appendReason(result.Reasons, "score reached review threshold")
		}
	}
	return result, errors.Join(failures...)
}

func appendReason(reasons []string, reason string) []string {
	if len(reasons) == 1 && reasons[0] == "no fraud signal detected" {
		return []string{reason}
	}
	return append(reasons, reason)
}
func validateHits(hits []domain.RuleHit, initial float64, allowEmptyOutcome bool) error {
	score := initial
	for _, hit := range hits {
		if !validID(hit.RuleID) {
			return errors.New("rule hit requires an explicit ID")
		}
		if hit.Outcome == "" && allowEmptyOutcome {
			// Score-only legacy rule; escalation is applied by configured thresholds.
		} else {
			switch hit.Outcome {
			case domain.DecisionAllow, domain.DecisionReview, domain.DecisionBlock:
			default:
				return fmt.Errorf("invalid hit outcome %q", hit.Outcome)
			}
		}
		score += hit.Score
		if math.IsNaN(score) || math.IsInf(score, 0) {
			return errors.New("hit scores must have a finite aggregate")
		}
	}
	return nil
}
