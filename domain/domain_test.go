package domain

import (
	"math"
	"reflect"
	"testing"
)

func TestValidateTransferReportsCanonicalFieldErrors(t *testing.T) {
	request := TransferCheckRequest{}
	request.Payload.InstructedAmount.Amount = math.NaN()
	want := []string{
		"message_type is required",
		"payload.message_id is required",
		"payload.creation_date_time is required",
		"payload.payment_identification.end_to_end_id is required",
		"payload.instructed_amount.amount must be positive",
		"payload.instructed_amount.currency is required",
		"payload.debtor_account.id is required",
		"payload.creditor_account.id is required",
		"payload.debtor_agent.bicfi is required",
		"payload.creditor_agent.bicfi is required",
	}
	if got := ValidateTransfer(request); !reflect.DeepEqual(got, want) {
		t.Fatalf("validation errors = %#v, want %#v", got, want)
	}
}

func TestValidationReason(t *testing.T) {
	want := "validation failed: first; second"
	if got := ValidationReason([]string{"first", "second"}); got != want {
		t.Fatalf("reason = %q, want %q", got, want)
	}
}
