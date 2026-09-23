package engine

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/apache261/Byakugan-core/domain"
)

func TestIdempotencyRetainsOriginalAcrossConflict(t *testing.T) {
	engine := New(nil)
	request := validRequest()
	first := engine.Check(request)

	conflicting := request
	conflicting.Payload.InstructedAmount.Amount++
	conflict := engine.Check(conflicting)
	if conflict.Decision != domain.DecisionReview {
		t.Fatalf("conflicting payload decision = %q, want review", conflict.Decision)
	}
	conflictReplay := engine.Check(conflicting)
	if conflictReplay.ID != conflict.ID || conflictReplay.IdempotencyFingerprint != conflict.IdempotencyFingerprint {
		t.Fatalf("conflict replay changed: first=%+v replay=%+v", conflict, conflictReplay)
	}

	replay := engine.Check(request)
	if replay.ID != first.ID || replay.IdempotencyFingerprint != first.IdempotencyFingerprint {
		t.Fatalf("original replay changed: first=%+v replay=%+v", first, replay)
	}
}

func TestDefaultHighAmountRule(t *testing.T) {
	request := validRequest()
	request.Payload.InstructedAmount.Amount = 100000
	check := New(DefaultRules()).Check(request)
	if check.Decision != domain.DecisionReview || len(check.RuleHits) != 1 {
		t.Fatalf("check = %+v, want one review hit", check)
	}
}

func TestNonFiniteAmountIsReviewed(t *testing.T) {
	request := validRequest()
	request.Payload.InstructedAmount.Amount = math.NaN()
	check := New(nil).Check(request)
	if check.Decision != domain.DecisionReview {
		t.Fatalf("decision = %q, want review", check.Decision)
	}
}

func TestInvalidRuleOutcomeCannotAllow(t *testing.T) {
	rule := domain.Rule{ID: "typo", Name: "Typo", Enabled: true, Definition: json.RawMessage(`{"field":"amount","op":"gt","value":1,"outcome":"blok","score":100}`)}
	check := New([]domain.Rule{rule}).Check(validRequest())
	if check.Decision != domain.DecisionReview {
		t.Fatalf("decision = %q, want review", check.Decision)
	}
}

func TestIdempotencyCacheEvictsOldestEntryAtCap(t *testing.T) {
	engine := NewWithOptions(nil, Options{MaxIdempotencyEntries: 2})
	firstRequest := validRequest()
	first := engine.Check(firstRequest)
	secondRequest := validRequest()
	secondRequest.IdempotencyKey = "idem-2"
	engine.Check(secondRequest)
	thirdRequest := validRequest()
	thirdRequest.IdempotencyKey = "idem-3"
	engine.Check(thirdRequest)

	replayedAfterEviction := engine.Check(firstRequest)
	if replayedAfterEviction.ID == first.ID {
		t.Fatal("oldest idempotency entry was not evicted")
	}
	if engine.cacheEntries != 2 {
		t.Fatalf("cache entries = %d, want 2", engine.cacheEntries)
	}
}

func TestContextAppliesBlockedAccountsDisabledRulesAndThresholds(t *testing.T) {
	rule := domain.Rule{ID: "score", Name: "Score", Enabled: true, Definition: json.RawMessage(`{"field":"amount","op":"gte","value":1,"outcome":"allow","score":40}`)}
	decisionEngine := New([]domain.Rule{rule})

	review := decisionEngine.CheckWithContext(validRequest(), Context{ReviewThreshold: 40})
	if review.Decision != domain.DecisionReview {
		t.Fatalf("threshold decision = %q, want review", review.Decision)
	}

	disabledRequest := validRequest()
	disabledRequest.IdempotencyKey = "disabled"
	disabled := decisionEngine.CheckWithContext(disabledRequest, Context{ReviewThreshold: 40, DisabledRuleIDs: []string{"score"}})
	if disabled.Decision != domain.DecisionAllow || disabled.Score != 0 {
		t.Fatalf("disabled rule result = %+v", disabled)
	}

	blockedRequest := validRequest()
	blockedRequest.IdempotencyKey = "blocked"
	blocked := decisionEngine.CheckWithContext(blockedRequest, Context{BlockedAccountIDs: []string{"creditor"}})
	if blocked.Decision != domain.DecisionBlock {
		t.Fatalf("blocked account decision = %q, want block", blocked.Decision)
	}
}

func TestReplaceRulesValidatesAndInstallsAtomically(t *testing.T) {
	decisionEngine := New(nil)
	valid := domain.Rule{ID: "valid", Name: "Valid", Enabled: true, Definition: json.RawMessage(`{"field":"amount","op":"gte","value":1,"outcome":"review"}`)}
	if err := decisionEngine.ReplaceRules([]domain.Rule{valid}); err != nil {
		t.Fatal(err)
	}
	if got := decisionEngine.Check(validRequest()); got.Decision != domain.DecisionReview {
		t.Fatalf("decision after replacement = %q", got.Decision)
	}
	invalid := domain.Rule{ID: "invalid", Definition: json.RawMessage(`{"field":"secret.private_field","op":"eq","value":true,"outcome":"block"}`)}
	if err := decisionEngine.ReplaceRules([]domain.Rule{invalid}); err == nil {
		t.Fatal("invalid replacement accepted")
	}
	if installed := decisionEngine.Rules(); len(installed) != 1 || installed[0].ID != "valid" {
		t.Fatalf("installed rules changed after invalid replacement: %+v", installed)
	}
}

func TestEngineRetainsBoundedHistoryForVelocityRules(t *testing.T) {
	rule := domain.Rule{ID: "velocity", Name: "Velocity", Enabled: true, Definition: json.RawMessage(`{"field":"velocity.debtor_transfer_count","op":"gte","value":2,"window_hours":1,"outcome":"review","score":25}`)}
	decisionEngine := NewWithOptions([]domain.Rule{rule}, Options{MaxHistoryEntries: 2})
	for i := 1; i <= 3; i++ {
		request := validRequest()
		request.IdempotencyKey = ""
		request.Payload.MessageID = fmt.Sprintf("message-%d", i)
		request.Payload.PaymentIdentification.EndToEndID = fmt.Sprintf("e2e-%d", i)
		check := decisionEngine.Check(request)
		if i < 3 && check.Decision != domain.DecisionAllow {
			t.Fatalf("decision %d = %q, want allow", i, check.Decision)
		}
		if i == 3 && check.Decision != domain.DecisionReview {
			t.Fatalf("decision %d = %q, want review", i, check.Decision)
		}
	}
	if history := decisionEngine.History(0); len(history) != 2 || history[0].Request.Payload.MessageID != "message-3" {
		t.Fatalf("unexpected bounded history: %+v", history)
	}
}

func validRequest() domain.TransferCheckRequest {
	return domain.TransferCheckRequest{IdempotencyKey: "idem-1", MessageType: "pacs.008", Payload: domain.Pacs008Transfer{
		MessageID: "msg-1", CreationDateTime: time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC),
		PaymentIdentification: domain.PaymentIdentification{EndToEndID: "e2e-1"},
		InstructedAmount:      domain.Money{Amount: 50, Currency: "PHP"},
		DebtorAccount:         domain.AccountRef{ID: "debtor"}, CreditorAccount: domain.AccountRef{ID: "creditor"},
		DebtorAgent: domain.Agent{BICFI: "DEUTPHMM"}, CreditorAgent: domain.Agent{BICFI: "BOPIPHMM"},
	}}
}
