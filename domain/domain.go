package domain

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

type Decision string

const (
	DecisionAllow  Decision = "allow"
	DecisionReview Decision = "review"
	DecisionBlock  Decision = "block"
)

type Agent struct {
	BICFI string `json:"bicfi"`
	Name  string `json:"name,omitempty"`
}

type Party struct {
	Name string `json:"name,omitempty"`
}

type AccountRef struct {
	ID   string `json:"id"`
	IBAN string `json:"iban,omitempty"`
	BBAN string `json:"bban,omitempty"`
}

type Money struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type PaymentIdentification struct {
	InstructionID           string `json:"instruction_id,omitempty"`
	EndToEndID              string `json:"end_to_end_id"`
	TransactionID           string `json:"transaction_id,omitempty"`
	ClearingSystemReference string `json:"clearing_system_reference,omitempty"`
}

type Pacs008Transfer struct {
	MessageDefinitionID     string                `json:"message_definition_id,omitempty"`
	SchemaNamespace         string                `json:"schema_namespace,omitempty"`
	MessageID               string                `json:"message_id"`
	CreationDateTime        time.Time             `json:"creation_date_time"`
	PaymentIdentification   PaymentIdentification `json:"payment_identification"`
	InstructedAmount        Money                 `json:"instructed_amount"`
	Debtor                  Party                 `json:"debtor"`
	DebtorAccount           AccountRef            `json:"debtor_account"`
	DebtorAgent             Agent                 `json:"debtor_agent"`
	Creditor                Party                 `json:"creditor"`
	CreditorAccount         AccountRef            `json:"creditor_account"`
	CreditorAgent           Agent                 `json:"creditor_agent"`
	RemittanceInformation   string                `json:"remittance_information,omitempty"`
	InterbankSettlementDate string                `json:"interbank_settlement_date,omitempty"`
}

type TransferCheckRequest struct {
	IdempotencyKey string          `json:"idempotency_key,omitempty"`
	MessageType    string          `json:"message_type"`
	Payload        Pacs008Transfer `json:"payload"`
}

type Rule struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Priority   int             `json:"priority"`
	Enabled    bool            `json:"enabled"`
	Definition json.RawMessage `json:"definition"`
}

type RuleHit struct {
	RuleID   string   `json:"rule_id"`
	RuleName string   `json:"rule_name"`
	Outcome  Decision `json:"outcome,omitempty"`
	Score    float64  `json:"score_delta"`
	Reason   string   `json:"reason"`
}

type TransferCheck struct {
	ID                     string               `json:"id"`
	Version                int                  `json:"version"`
	IdempotencyKey         string               `json:"idempotency_key,omitempty"`
	IdempotencyFingerprint string               `json:"idempotency_fingerprint,omitempty"`
	Request                TransferCheckRequest `json:"request"`
	Decision               Decision             `json:"decision"`
	Score                  float64              `json:"score"`
	Reasons                []string             `json:"reasons"`
	RuleHits               []RuleHit            `json:"rule_hits"`
	CreatedAt              time.Time            `json:"created_at"`
}

func ValidateTransfer(req TransferCheckRequest) []string {
	var errs []string
	if req.MessageType != "pacs.008" {
		errs = append(errs, "message_type must be pacs.008")
	}
	p := req.Payload
	if strings.TrimSpace(p.MessageID) == "" {
		errs = append(errs, "payload.message_id is required")
	}
	if p.CreationDateTime.IsZero() {
		errs = append(errs, "payload.creation_date_time is required")
	}
	if strings.TrimSpace(p.PaymentIdentification.EndToEndID) == "" {
		errs = append(errs, "payload.payment_identification.end_to_end_id is required")
	}
	if p.InstructedAmount.Amount <= 0 || math.IsNaN(p.InstructedAmount.Amount) || math.IsInf(p.InstructedAmount.Amount, 0) {
		errs = append(errs, "payload.instructed_amount.amount must be positive")
	}
	if strings.TrimSpace(p.InstructedAmount.Currency) == "" {
		errs = append(errs, "payload.instructed_amount.currency is required")
	}
	if strings.TrimSpace(p.DebtorAccount.ID) == "" || strings.TrimSpace(p.CreditorAccount.ID) == "" {
		errs = append(errs, "debtor and creditor account ids are required")
	}
	if p.DebtorAccount.ID != "" && p.DebtorAccount.ID == p.CreditorAccount.ID {
		errs = append(errs, "debtor and creditor account cannot be the same")
	}
	if strings.TrimSpace(p.DebtorAgent.BICFI) == "" || strings.TrimSpace(p.CreditorAgent.BICFI) == "" {
		errs = append(errs, "debtor and creditor agent BICs are required")
	}
	return errs
}

func ValidationReason(errs []string) string {
	return fmt.Sprintf("validation failed: %s", strings.Join(errs, "; "))
}
