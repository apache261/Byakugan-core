package rules

import (
	"encoding/json"
	"testing"

	"github.com/apache261/Byakugan-core/domain"
)

func TestValidateAcceptsPortableRule(t *testing.T) {
	rule := domain.Rule{Definition: json.RawMessage(`{"field":"amount","op":"gte","value":100,"outcome":"review"}`)}
	if err := Validate(rule); err != nil {
		t.Fatalf("portable rule rejected: %v", err)
	}
}

func TestValidateRejectsPrivateExtensionFields(t *testing.T) {
	rule := domain.Rule{Definition: json.RawMessage(`{"field":"amount","op":"gte","value":100,"window_hours":24,"outcome":"review"}`)}
	if err := Validate(rule); err == nil {
		t.Fatal("rule with a private extension field was accepted")
	}
}

func TestValidateRejectsTrailingJSON(t *testing.T) {
	rule := domain.Rule{Definition: json.RawMessage(`{"field":"amount","op":"gte","value":100,"outcome":"review"} {}`)}
	if err := Validate(rule); err == nil {
		t.Fatal("rule with trailing JSON was accepted")
	}
}
