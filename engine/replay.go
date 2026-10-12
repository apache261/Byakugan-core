package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/apache261/Byakugan-core/domain"
)

// ReplayEngine adds explicitly selected, bounded in-memory history/replay to a
// Framework. It has no durable or source-scoped operational idempotency.
// Failed evaluations are not recorded or cached.
type ReplayEngine struct {
	framework  *Framework
	mu         sync.Mutex
	checks     map[string]map[string]domain.TransferCheck
	pending    map[string]chan struct{}
	order      []cacheEntry
	entries    int
	maxEntries int
	history    []domain.TransferCheck
	maxHistory int
}

func NewReplayEngine(framework *Framework, options Options) (*ReplayEngine, error) {
	if framework == nil {
		return nil, errors.New("framework is required")
	}
	if options.MaxIdempotencyEntries <= 0 {
		options.MaxIdempotencyEntries = DefaultMaxIdempotencyEntries
	}
	if options.MaxHistoryEntries <= 0 {
		options.MaxHistoryEntries = DefaultMaxHistoryEntries
	}
	return &ReplayEngine{framework: framework, checks: map[string]map[string]domain.TransferCheck{}, pending: map[string]chan struct{}{}, maxEntries: options.MaxIdempotencyEntries, maxHistory: options.MaxHistoryEntries}, nil
}

// Check coalesces successful concurrent requests for the same key. Callbacks
// execute outside locks; waiting callers can cancel independently. A callback
// must not recursively Check the same key it is currently evaluating.
func (m *ReplayEngine) Check(ctx context.Context, req domain.TransferCheckRequest, data Context) (domain.TransferCheck, error) {
	if err := ctx.Err(); err != nil {
		return failedReplay(req, err), err
	}
	fingerprintRequest := req
	fingerprintRequest.IdempotencyKey = ""
	raw, err := json.Marshal(fingerprintRequest)
	if err != nil {
		return failedReplay(req, err), err
	}
	digest := sha256.Sum256(raw)
	fp := hex.EncodeToString(digest[:])
	conflict := false
	for {
		if err := ctx.Err(); err != nil {
			return failedReplay(req, err), err
		}
		m.mu.Lock()
		if req.IdempotencyKey != "" {
			if prior, ok := m.checks[req.IdempotencyKey][fp]; ok {
				m.mu.Unlock()
				return cloneCheck(prior), nil
			}
			if pending, ok := m.pending[req.IdempotencyKey]; ok {
				m.mu.Unlock()
				select {
				case <-pending:
					continue
				case <-ctx.Done():
					return failedReplay(req, ctx.Err()), ctx.Err()
				}
			}
			m.pending[req.IdempotencyKey] = make(chan struct{})
			conflict = len(m.checks[req.IdempotencyKey]) > 0
		}
		if len(data.Rules.History) == 0 && !data.Rules.Metrics.Loaded && len(data.Rules.MetricsByWindow) == 0 {
			data.Rules.History = cloneHistory(m.history)
		}
		m.mu.Unlock()
		break
	}
	if req.IdempotencyKey != "" {
		defer func() {
			m.mu.Lock()
			close(m.pending[req.IdempotencyKey])
			delete(m.pending, req.IdempotencyKey)
			m.mu.Unlock()
		}()
	}
	now := time.Now().UTC()
	if data.Rules.Now.IsZero() {
		data.Rules.Now = now
	}
	result, err := m.framework.Evaluate(ctx, req, data)
	check := domain.TransferCheck{ID: newID(now), Version: 1, IdempotencyKey: req.IdempotencyKey, IdempotencyFingerprint: fp, Request: req, Decision: result.Decision, Score: result.Score, Reasons: result.Reasons, RuleHits: result.RuleHits, CreatedAt: now}
	if err != nil {
		return check, err
	}
	if err := ctx.Err(); err != nil {
		if check.Decision == domain.DecisionAllow {
			check.Decision = domain.DecisionReview
		}
		check.Reasons = appendReason(check.Reasons, err.Error())
		return check, err
	}
	if conflict {
		if check.Decision != domain.DecisionBlock {
			check.Decision = domain.DecisionReview
		}
		check.Reasons = appendReason(check.Reasons, "idempotency key reused with a different payload")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		if check.Decision == domain.DecisionAllow {
			check.Decision = domain.DecisionReview
		}
		check.Reasons = appendReason(check.Reasons, err.Error())
		return check, err
	}
	if len(m.history) >= m.maxHistory {
		copy(m.history, m.history[1:])
		m.history = m.history[:m.maxHistory-1]
	}
	m.history = append(m.history, cloneCheck(check))
	if req.IdempotencyKey != "" {
		for m.entries >= m.maxEntries {
			oldest := m.order[0]
			m.order = m.order[1:]
			delete(m.checks[oldest.key], oldest.fingerprint)
			if len(m.checks[oldest.key]) == 0 {
				delete(m.checks, oldest.key)
			}
			m.entries--
		}
		if m.checks[req.IdempotencyKey] == nil {
			m.checks[req.IdempotencyKey] = map[string]domain.TransferCheck{}
		}
		m.checks[req.IdempotencyKey][fp] = cloneCheck(check)
		m.order = append(m.order, cacheEntry{key: req.IdempotencyKey, fingerprint: fp})
		m.entries++
	}
	return check, nil
}
func (m *ReplayEngine) History(limit int) []domain.TransferCheck {
	m.mu.Lock()
	defer m.mu.Unlock()
	if limit <= 0 || limit > len(m.history) {
		limit = len(m.history)
	}
	result := make([]domain.TransferCheck, 0, limit)
	for i := len(m.history) - 1; i >= len(m.history)-limit; i-- {
		result = append(result, cloneCheck(m.history[i]))
	}
	return result
}
func failedReplay(req domain.TransferCheckRequest, err error) domain.TransferCheck {
	return domain.TransferCheck{Request: req, Decision: domain.DecisionReview, Reasons: []string{err.Error()}}
}
func cloneCheck(check domain.TransferCheck) domain.TransferCheck {
	check.Reasons = append([]string(nil), check.Reasons...)
	check.RuleHits = append([]domain.RuleHit(nil), check.RuleHits...)
	return check
}
func cloneHistory(history []domain.TransferCheck) []domain.TransferCheck {
	result := make([]domain.TransferCheck, len(history))
	for i := range history {
		result[i] = cloneCheck(history[i])
	}
	return result
}
