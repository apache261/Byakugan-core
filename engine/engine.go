package engine

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/apache261/Byakugan-core/domain"
	"github.com/apache261/Byakugan-core/rules"
)

type Engine struct {
	mu              sync.RWMutex
	rules           []domain.Rule
	checks          map[string]map[string]domain.TransferCheck
	cacheOrder      []cacheEntry
	cacheEntries    int
	maxCacheEntries int
	history         []domain.TransferCheck
	maxHistory      int
}

type Options struct {
	MaxIdempotencyEntries int
	MaxHistoryEntries     int
}

// Context configures one decision without coupling the library to a database,
// account service, or configuration system. Zero thresholds disable score-based
// escalation; explicit rule outcomes still apply.
type Context struct {
	Rules                     rules.Context
	ReviewThreshold           float64
	BlockThreshold            float64
	ValidationFailureDecision domain.Decision
	BlockedAccountIDs         []string
	DisabledRuleIDs           []string
}

type cacheEntry struct {
	key         string
	fingerprint string
}

const DefaultMaxIdempotencyEntries = 10000
const DefaultMaxHistoryEntries = 10000

var fallbackIDSequence atomic.Uint64

func New(ruleSet []domain.Rule) *Engine {
	return NewWithOptions(ruleSet, Options{})
}

func NewWithOptions(ruleSet []domain.Rule, options Options) *Engine {
	cloned := append([]domain.Rule(nil), ruleSet...)
	sort.SliceStable(cloned, func(i, j int) bool { return cloned[i].Priority < cloned[j].Priority })
	if options.MaxIdempotencyEntries <= 0 {
		options.MaxIdempotencyEntries = DefaultMaxIdempotencyEntries
	}
	if options.MaxHistoryEntries <= 0 {
		options.MaxHistoryEntries = DefaultMaxHistoryEntries
	}
	return &Engine{
		rules: cloned, checks: make(map[string]map[string]domain.TransferCheck),
		maxCacheEntries: options.MaxIdempotencyEntries,
		maxHistory:      options.MaxHistoryEntries,
	}
}

func DefaultRules() []domain.Rule {
	return []domain.Rule{
		{
			ID: "community-rapid-debtor-volume", Name: "Rapid debtor transfer volume", Priority: 50, Enabled: true,
			Definition: json.RawMessage(`{"field":"velocity.debtor_transfer_count","op":"gte","value":5,"window_minutes":15,"outcome":"review","score":40,"reason":"debtor has at least five prior transfers in fifteen minutes"}`),
		},
		{
			ID: "community-high-amount", Name: "High amount review", Priority: 100, Enabled: true,
			Definition: json.RawMessage(`{"field":"amount","op":"gte","value":100000,"outcome":"review","score":50,"reason":"amount meets the community review threshold"}`),
		},
	}
}

func (e *Engine) Check(req domain.TransferCheckRequest) domain.TransferCheck {
	return e.CheckWithContext(req, Context{})
}

// CheckWithContext evaluates a transfer using optional account, history, and
// policy data supplied by the embedding application.
func (e *Engine) CheckWithContext(req domain.TransferCheckRequest, context Context) domain.TransferCheck {
	now := time.Now().UTC()
	fingerprint := fingerprint(req)
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(context.Rules.History) == 0 && !context.Rules.Metrics.Loaded && len(context.Rules.MetricsByWindow) == 0 {
		context.Rules.History = e.history
	}
	if req.IdempotencyKey != "" {
		if priorResults, ok := e.checks[req.IdempotencyKey]; ok {
			if prior, matched := priorResults[fingerprint]; matched {
				return prior
			}
			check := e.evaluate(req, fingerprint, now, true, context)
			e.record(check)
			e.store(req.IdempotencyKey, fingerprint, check)
			return check
		}
	}
	check := e.evaluate(req, fingerprint, now, false, context)
	e.record(check)
	if req.IdempotencyKey != "" {
		e.store(req.IdempotencyKey, fingerprint, check)
	}
	return check
}

// History returns up to limit recent non-replayed decisions, newest first.
// A non-positive limit returns all retained entries.
func (e *Engine) History(limit int) []domain.TransferCheck {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if limit <= 0 || limit > len(e.history) {
		limit = len(e.history)
	}
	result := make([]domain.TransferCheck, 0, limit)
	for i := len(e.history) - 1; i >= len(e.history)-limit; i-- {
		result = append(result, e.history[i])
	}
	return result
}

func (e *Engine) evaluate(req domain.TransferCheckRequest, fingerprint string, now time.Time, idempotencyConflict bool, context Context) domain.TransferCheck {
	check := domain.TransferCheck{
		ID: newID(now), Version: 1, IdempotencyKey: req.IdempotencyKey,
		IdempotencyFingerprint: fingerprint, Request: req, Decision: domain.DecisionAllow,
		Reasons: []string{"no fraud signal detected"}, RuleHits: []domain.RuleHit{}, CreatedAt: now,
	}
	if contains(context.BlockedAccountIDs, req.Payload.DebtorAccount.ID) || contains(context.BlockedAccountIDs, req.Payload.CreditorAccount.ID) {
		check.Decision = domain.DecisionBlock
		check.Reasons = []string{"debtor or creditor account is blocked"}
		return check
	}
	if validationErrors := domain.ValidateTransfer(req); len(validationErrors) > 0 {
		switch context.ValidationFailureDecision {
		case domain.DecisionAllow, domain.DecisionReview, domain.DecisionBlock:
			check.Decision = context.ValidationFailureDecision
		default:
			check.Decision = domain.DecisionReview
		}
		check.Reasons = []string{domain.ValidationReason(validationErrors)}
	}
	if idempotencyConflict {
		check.Decision = domain.DecisionReview
		check.Reasons = append(check.Reasons, "idempotency key reused with a different payload")
	}
	for _, rule := range e.rules {
		if !rule.Enabled || contains(context.DisabledRuleIDs, rule.ID) {
			continue
		}
		hit, matched, err := rules.EvaluateWithContext(rule, req, context.Rules)
		if err != nil {
			check.Decision = domain.DecisionReview
			check.Reasons = append(check.Reasons, fmt.Sprintf("rule %s could not be evaluated", rule.ID))
			continue
		}
		if !matched {
			continue
		}
		check.RuleHits = append(check.RuleHits, hit)
		check.Score += hit.Score
		check.Reasons = append(check.Reasons, hit.Reason)
		if hit.Outcome == domain.DecisionBlock || hit.Outcome == domain.DecisionReview && check.Decision == domain.DecisionAllow {
			check.Decision = hit.Outcome
		}
	}
	if check.Decision != domain.DecisionBlock {
		if context.BlockThreshold > 0 && check.Score >= context.BlockThreshold {
			check.Decision = domain.DecisionBlock
			check.Reasons = append(check.Reasons, "score reached block threshold")
		} else if context.ReviewThreshold > 0 && check.Score >= context.ReviewThreshold {
			check.Decision = domain.DecisionReview
			check.Reasons = append(check.Reasons, "score reached review threshold")
		}
	}
	if len(check.RuleHits) > 0 || check.Decision != domain.DecisionAllow {
		check.Reasons = remove(check.Reasons, "no fraud signal detected")
	}
	return check
}

// Rules returns a detached, priority-ordered snapshot of the installed rules.
func (e *Engine) Rules() []domain.Rule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return append([]domain.Rule(nil), e.rules...)
}

// ReplaceRules validates and atomically installs a new rule set.
func (e *Engine) ReplaceRules(ruleSet []domain.Rule) error {
	for _, rule := range ruleSet {
		if err := rules.Validate(rule); err != nil {
			return fmt.Errorf("validate rule %q: %w", rule.ID, err)
		}
	}
	cloned := append([]domain.Rule(nil), ruleSet...)
	sort.SliceStable(cloned, func(i, j int) bool { return cloned[i].Priority < cloned[j].Priority })
	e.mu.Lock()
	e.rules = cloned
	e.mu.Unlock()
	return nil
}

func fingerprint(req domain.TransferCheckRequest) string {
	req.IdempotencyKey = ""
	raw, _ := json.Marshal(req)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func newID(now time.Time) string {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err == nil {
		return hex.EncodeToString(random)
	}
	sequence := fallbackIDSequence.Add(1)
	digest := sha256.Sum256([]byte(fmt.Sprintf("%d:%d", now.UnixNano(), sequence)))
	return hex.EncodeToString(digest[:16])
}

func (e *Engine) store(key, fingerprint string, check domain.TransferCheck) {
	results := e.checks[key]
	if results != nil {
		if _, exists := results[fingerprint]; exists {
			results[fingerprint] = check
			return
		}
	}
	for e.cacheEntries >= e.maxCacheEntries {
		oldest := e.cacheOrder[0]
		e.cacheOrder = e.cacheOrder[1:]
		versions := e.checks[oldest.key]
		if _, exists := versions[oldest.fingerprint]; !exists {
			continue
		}
		delete(versions, oldest.fingerprint)
		e.cacheEntries--
		if len(versions) == 0 {
			delete(e.checks, oldest.key)
		}
	}
	results = e.checks[key]
	if results == nil {
		results = make(map[string]domain.TransferCheck)
		e.checks[key] = results
	}
	if _, exists := results[fingerprint]; exists {
		results[fingerprint] = check
		return
	}
	results[fingerprint] = check
	e.cacheOrder = append(e.cacheOrder, cacheEntry{key: key, fingerprint: fingerprint})
	e.cacheEntries++
}

func (e *Engine) record(check domain.TransferCheck) {
	if len(e.history) >= e.maxHistory {
		copy(e.history, e.history[len(e.history)-e.maxHistory+1:])
		e.history = e.history[:e.maxHistory-1]
	}
	e.history = append(e.history, check)
}

func remove(values []string, target string) []string {
	out := values[:0]
	for _, value := range values {
		if value != target {
			out = append(out, value)
		}
	}
	return out
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
