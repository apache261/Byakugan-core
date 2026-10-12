package engine

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/apache261/Byakugan-core/domain"
)

func custom(id string, priority int, outcome domain.Decision, score float64) Registration {
	return Registration{ID: id, Priority: priority, Evaluator: EvaluatorFunc(func(context.Context, domain.TransferCheckRequest, Context) ([]domain.RuleHit, error) {
		return []domain.RuleHit{{RuleID: id, Outcome: outcome, Score: score, Reason: id}}, nil
	})}
}
func portable(id string, priority int, outcome domain.Decision, score float64) domain.Rule {
	raw, _ := json.Marshal(map[string]any{"field": "message_type", "op": "eq", "value": "pacs.008", "outcome": outcome, "score": score, "reason": id})
	return domain.Rule{ID: id, Name: id, Priority: priority, Enabled: true, Definition: raw}
}
func framework(t *testing.T, ruleSet []domain.Rule, registrations []Registration, policy Policy) *Framework {
	t.Helper()
	f, err := NewFramework(ruleSet, registrations, policy)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestFrameworkDeterministicOrderAndThresholds(t *testing.T) {
	f := framework(t, []domain.Rule{portable("second", 20, domain.DecisionAllow, 10), portable("first", 10, domain.DecisionReview, 5)}, []Registration{custom("third", 20, domain.DecisionAllow, 15)}, DefaultPolicy())
	result, err := f.Evaluate(context.Background(), validRequest(), Context{BlockThreshold: 30})
	ids := []string{}
	for _, hit := range result.RuleHits {
		ids = append(ids, hit.RuleID)
	}
	if err != nil || result.Decision != domain.DecisionBlock || result.Score != 30 || !reflect.DeepEqual(ids, []string{"first", "second", "third"}) || !reflect.DeepEqual(result.Reasons, []string{"first", "second", "third", "score reached block threshold"}) {
		t.Fatalf("%+v %v", result, err)
	}
	for _, tc := range []struct {
		score float64
		want  domain.Decision
	}{{49, domain.DecisionAllow}, {50, domain.DecisionReview}, {90, domain.DecisionBlock}} {
		result, err := framework(t, nil, []Registration{custom("score", 0, domain.DecisionAllow, tc.score)}, DefaultPolicy()).Evaluate(context.Background(), validRequest(), Context{ReviewThreshold: 50, BlockThreshold: 90})
		if err != nil || result.Decision != tc.want {
			t.Fatalf("score %v: %+v %v", tc.score, result, err)
		}
	}
	f = framework(t, nil, []Registration{custom("block", 0, domain.DecisionBlock, 0), custom("allow", 1, domain.DecisionAllow, 0)}, DefaultPolicy())
	result, err = f.Evaluate(context.Background(), validRequest(), Context{})
	if err != nil || result.Decision != domain.DecisionBlock || len(result.RuleHits) != 2 {
		t.Fatalf("allow downgraded block: %+v %v", result, err)
	}
}

func TestFrameworkRejectsAmbiguousRegistryAndInvalidReplacement(t *testing.T) {
	for _, registrations := range [][]Registration{{{ID: "", Evaluator: custom("ok", 0, domain.DecisionAllow, 0).Evaluator}}, {custom("same", 0, domain.DecisionAllow, 0), custom("same", 1, domain.DecisionReview, 0)}, {{ID: "typed-nil", Evaluator: EvaluatorFunc(nil)}}} {
		if _, err := NewFramework(nil, registrations, DefaultPolicy()); err == nil {
			t.Fatal("invalid registry accepted")
		}
	}
	f := framework(t, []domain.Rule{portable("same", 0, domain.DecisionReview, 1)}, nil, DefaultPolicy())
	if err := f.Register(custom("same", 1, domain.DecisionAllow, 0)); err == nil {
		t.Fatal("rule/custom ID collision accepted")
	}
	invalid := portable("bad", 0, domain.DecisionBlock, 0)
	invalid.Definition = json.RawMessage(`{"field":"private","op":"eq","value":true,"outcome":"block"}`)
	if err := f.ReplaceRules([]domain.Rule{invalid}); err == nil {
		t.Fatal("invalid replacement accepted")
	}
	got, err := f.Evaluate(context.Background(), validRequest(), Context{})
	if err != nil || got.Decision != domain.DecisionReview || got.RuleHits[0].RuleID != "same" {
		t.Fatalf("replacement changed registry: %+v %v", got, err)
	}
	if _, err := NewFramework(nil, nil, Policy{Failure: "allow"}); err == nil {
		t.Fatal("allow failure policy accepted")
	}
}

func TestFrameworkErrorsAreReturnedAndNeverAllow(t *testing.T) {
	unavailable := errors.New("risk context unavailable")
	bad := Registration{ID: "failure", Priority: 1, Evaluator: EvaluatorFunc(func(context.Context, domain.TransferCheckRequest, Context) ([]domain.RuleHit, error) {
		return []domain.RuleHit{{RuleID: "ignore", Outcome: domain.DecisionAllow, Score: 999}}, unavailable
	})}
	for _, tc := range []struct {
		policy Policy
		want   domain.Decision
	}{{DefaultPolicy(), domain.DecisionReview}, {Policy{Failure: FailureBlock}, domain.DecisionBlock}, {FullCompatibilityPolicy(), domain.DecisionReview}} {
		got, err := framework(t, nil, []Registration{bad, custom("later", 2, domain.DecisionAllow, 1)}, tc.policy).Evaluate(context.Background(), validRequest(), Context{ReviewThreshold: 50, BlockThreshold: 90})
		if !errors.Is(err, unavailable) || got.Decision != tc.want || got.Score > 1 {
			t.Fatalf("failure silently accepted: %+v %v", got, err)
		}
	}
	got, err := framework(t, nil, []Registration{custom("block", 0, domain.DecisionBlock, 0), bad}, DefaultPolicy()).Evaluate(context.Background(), validRequest(), Context{})
	if !errors.Is(err, unavailable) || got.Decision != domain.DecisionBlock {
		t.Fatalf("error downgraded block: %+v %v", got, err)
	}
	for _, hit := range []domain.RuleHit{{RuleID: "bad", Outcome: "typo"}, {Outcome: domain.DecisionAllow}, {RuleID: "bad", Outcome: domain.DecisionAllow, Score: math.NaN()}, {RuleID: "bad", Outcome: domain.DecisionAllow, Score: math.Inf(1)}} {
		invalid := Registration{ID: "bad", Evaluator: EvaluatorFunc(func(context.Context, domain.TransferCheckRequest, Context) ([]domain.RuleHit, error) {
			return []domain.RuleHit{hit}, nil
		})}
		got, err := framework(t, nil, []Registration{invalid}, DefaultPolicy()).Evaluate(context.Background(), validRequest(), Context{})
		if err == nil || got.Decision == domain.DecisionAllow || len(got.RuleHits) != 0 {
			t.Fatalf("invalid hit accepted: %+v %v", got, err)
		}
	}
	invalidReq := validRequest()
	invalidReq.Payload.InstructedAmount.Amount = "0"
	got, err = framework(t, nil, nil, DefaultPolicy()).Evaluate(context.Background(), invalidReq, Context{ValidationFailureDecision: domain.DecisionAllow})
	if err == nil || got.Decision == domain.DecisionAllow {
		t.Fatalf("invalid transfer allowed: %+v %v", got, err)
	}
}

func TestFullCompatibilityStopsAndReplacesErrorReasons(t *testing.T) {
	f := framework(t, []domain.Rule{portable("review", 1, domain.DecisionReview, 10), portable("block", 2, domain.DecisionBlock, 20), portable("later", 3, domain.DecisionReview, 40)}, nil, FullCompatibilityPolicy())
	got, err := f.Evaluate(context.Background(), validRequest(), Context{ReviewThreshold: 50, BlockThreshold: 90})
	if err != nil || got.Decision != domain.DecisionBlock || got.Score != 30 || !reflect.DeepEqual(got.Reasons, []string{"review", "block"}) || len(got.RuleHits) != 2 {
		t.Fatalf("Full early exit mismatch: %+v %v", got, err)
	}
	rule := domain.Rule{ID: "runtime-error", Priority: 2, Enabled: true, Definition: json.RawMessage(`{"field":"amount","op":"gte","value":"0.001","outcome":"block"}`)}
	f = framework(t, []domain.Rule{portable("review", 1, domain.DecisionReview, 10), rule}, nil, FullCompatibilityPolicy())
	got, err = f.Evaluate(context.Background(), validRequest(), Context{ReviewThreshold: 50, BlockThreshold: 90})
	if err == nil || got.Decision != domain.DecisionReview || got.Score != 10 || !reflect.DeepEqual(got.Reasons, []string{"amount exceeds currency scale"}) {
		t.Fatalf("Full error mismatch: %+v %v", got, err)
	}
	got, err = framework(t, nil, nil, FullCompatibilityPolicy()).Evaluate(context.Background(), validRequest(), Context{})
	if err != nil || got.Decision != domain.DecisionBlock {
		t.Fatalf("Full zero threshold semantics changed: %+v %v", got, err)
	}
}

func TestFrameworkCancellationBeforeAndDuringCallbacks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := framework(t, nil, nil, DefaultPolicy()).Evaluate(ctx, validRequest(), Context{})
	if !errors.Is(err, context.Canceled) || got.Decision == domain.DecisionAllow {
		t.Fatalf("cancellation accepted: %+v %v", got, err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	callback := Registration{ID: "cancel", Evaluator: EvaluatorFunc(func(context.Context, domain.TransferCheckRequest, Context) ([]domain.RuleHit, error) {
		cancel()
		return []domain.RuleHit{{RuleID: "cancel", Outcome: domain.DecisionAllow}}, nil
	})}
	got, err = framework(t, nil, []Registration{callback}, DefaultPolicy()).Evaluate(ctx, validRequest(), Context{})
	if !errors.Is(err, context.Canceled) || got.Decision == domain.DecisionAllow || len(got.RuleHits) != 0 {
		t.Fatalf("callback cancellation accepted: %+v %v", got, err)
	}
}

func TestFrameworkSnapshotReplacementAndCallbacksOutsideLocks(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	gate := Registration{ID: "gate", Evaluator: EvaluatorFunc(func(ctx context.Context, _ domain.TransferCheckRequest, _ Context) ([]domain.RuleHit, error) {
		once.Do(func() { close(entered) })
		select {
		case <-release:
			return nil, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})}
	f := framework(t, []domain.Rule{portable("old", 10, domain.DecisionReview, 1)}, []Registration{gate}, DefaultPolicy())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan Result, 1)
	go func() { got, _ := f.Evaluate(ctx, validRequest(), Context{}); done <- got }()
	<-entered
	replaced := make(chan error, 1)
	go func() { replaced <- f.ReplaceRules([]domain.Rule{portable("new", 10, domain.DecisionBlock, 2)}) }()
	select {
	case err := <-replaced:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("callback held registry lock")
	}
	close(release)
	got := <-done
	if len(got.RuleHits) != 1 || got.RuleHits[0].RuleID != "old" {
		t.Fatalf("in-flight snapshot changed: %+v", got)
	}
	got, err := f.Evaluate(context.Background(), validRequest(), Context{})
	if err != nil || got.RuleHits[0].RuleID != "new" {
		t.Fatalf("replacement not visible: %+v %v", got, err)
	}
}

func TestFrameworkDetachesRulesAndCallbackContextWithoutReplay(t *testing.T) {
	rule := portable("original", 10, domain.DecisionReview, 1)
	var calls int
	callback := Registration{ID: "custom", Evaluator: EvaluatorFunc(func(_ context.Context, _ domain.TransferCheckRequest, data Context) ([]domain.RuleHit, error) {
		calls++
		data.Facts["risk"][0] = 'f'
		data.DisabledRuleIDs[0] = "original"
		return nil, nil
	})}
	f := framework(t, []domain.Rule{rule}, []Registration{callback}, DefaultPolicy())
	for i := range rule.Definition {
		rule.Definition[i] = 'x'
	}
	snapshot := f.Rules()
	for i := range snapshot[0].Definition {
		snapshot[0].Definition[i] = 'x'
	}
	data := Context{Facts: map[string]json.RawMessage{"risk": json.RawMessage(`true`)}, DisabledRuleIDs: []string{"unused"}}
	for i := 0; i < 2; i++ {
		got, err := f.Evaluate(context.Background(), validRequest(), data)
		if err != nil || len(got.RuleHits) != 1 {
			t.Fatalf("callback mutated subsequent input: %+v %v", got, err)
		}
	}
	if calls != 2 || string(data.Facts["risk"]) != "true" || data.DisabledRuleIDs[0] != "unused" {
		t.Fatalf("replay or caller mutation: %d %+v", calls, data)
	}
}

func TestFrameworkConcurrentEvaluationAndReplacement(t *testing.T) {
	f := framework(t, []domain.Rule{portable("rule", 0, domain.DecisionReview, 1)}, nil, DefaultPolicy())
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				if i%3 == 0 {
					if err := f.ReplaceRules([]domain.Rule{portable("rule", 0, domain.DecisionReview, 1)}); err != nil {
						t.Error(err)
					}
				}
				got, err := f.Evaluate(context.Background(), validRequest(), Context{})
				if err != nil || len(got.RuleHits) != 1 || got.Decision != domain.DecisionReview {
					t.Errorf("concurrent result %+v %v", got, err)
				}
			}
		}()
	}
	wg.Wait()
}
