package engine

import (
	"encoding/json"
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

func validRequest() domain.TransferCheckRequest {
	return domain.TransferCheckRequest{IdempotencyKey: "idem-1", MessageType: "pacs.008", Payload: domain.Pacs008Transfer{
		MessageID: "msg-1", CreationDateTime: time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC),
		PaymentIdentification: domain.PaymentIdentification{EndToEndID: "e2e-1"},
		InstructedAmount:      domain.Money{Amount: 50, Currency: "PHP"},
		DebtorAccount:         domain.AccountRef{ID: "debtor"}, CreditorAccount: domain.AccountRef{ID: "creditor"},
		DebtorAgent: domain.Agent{BICFI: "DEUTPHMM"}, CreditorAgent: domain.Agent{BICFI: "BOPIPHMM"},
	}}
}
