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
}

type Options struct {
	MaxIdempotencyEntries int
}

type cacheEntry struct {
	key         string
	fingerprint string
}

const DefaultMaxIdempotencyEntries = 10000

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
	return &Engine{rules: cloned, checks: make(map[string]map[string]domain.TransferCheck), maxCacheEntries: options.MaxIdempotencyEntries}
}

func DefaultRules() []domain.Rule {
	return []domain.Rule{{
		ID: "community-high-amount", Name: "High amount review", Priority: 100, Enabled: true,
		Definition: json.RawMessage(`{"field":"amount","op":"gte","value":100000,"outcome":"review","score":50,"reason":"amount meets the community review threshold"}`),
	}}
}

func (e *Engine) Check(req domain.TransferCheckRequest) domain.TransferCheck {
	now := time.Now().UTC()
	fingerprint := fingerprint(req)
	e.mu.Lock()
	defer e.mu.Unlock()
	if req.IdempotencyKey != "" {
		if priorResults, ok := e.checks[req.IdempotencyKey]; ok {
			if prior, matched := priorResults[fingerprint]; matched {
				return prior
			}
			check := e.evaluate(req, fingerprint, now, true)
			e.store(req.IdempotencyKey, fingerprint, check)
			return check
		}
	}
	check := e.evaluate(req, fingerprint, now, false)
	if req.IdempotencyKey != "" {
		e.store(req.IdempotencyKey, fingerprint, check)
	}
	return check
}

func (e *Engine) evaluate(req domain.TransferCheckRequest, fingerprint string, now time.Time, idempotencyConflict bool) domain.TransferCheck {
	check := domain.TransferCheck{
		ID: newID(now), Version: 1, IdempotencyKey: req.IdempotencyKey,
		IdempotencyFingerprint: fingerprint, Request: req, Decision: domain.DecisionAllow,
		Reasons: []string{"no fraud signal detected"}, RuleHits: []domain.RuleHit{}, CreatedAt: now,
	}
	if validationErrors := domain.ValidateTransfer(req); len(validationErrors) > 0 {
		check.Decision = domain.DecisionReview
		check.Reasons = []string{domain.ValidationReason(validationErrors)}
	}
	if idempotencyConflict {
		check.Decision = domain.DecisionReview
		check.Reasons = append(check.Reasons, "idempotency key reused with a different payload")
	}
	for _, rule := range e.rules {
		if !rule.Enabled {
			continue
		}
		hit, matched, err := rules.Evaluate(rule, req)
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
	if len(check.RuleHits) > 0 || check.Decision != domain.DecisionAllow {
		check.Reasons = remove(check.Reasons, "no fraud signal detected")
	}
	return check
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

func remove(values []string, target string) []string {
	out := values[:0]
	for _, value := range values {
		if value != target {
			out = append(out, value)
		}
	}
	return out
}
