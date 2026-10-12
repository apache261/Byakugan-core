package main

import (
	"encoding/json"
	"fmt"
	"github.com/apache261/Byakugan-core/domain"
	"github.com/apache261/Byakugan-core/engine"
	"github.com/apache261/Byakugan-core/rules"
	"os"
)

// Run from the module root: go run ./examples/supplied-aggregates < examples/transfer.json
func main() {
	var req domain.TransferCheckRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		panic(err)
	}
	rule := domain.Rule{ID: "exact-sum", Name: "Exact accumulated amount", Enabled: true, Definition: json.RawMessage(`{"field":"velocity.debtor_amount_sum_including_current","op":"gte","value":"1250.6","outcome":"review","reason":"exact sum meets threshold"}`)}
	check := engine.New([]domain.Rule{rule}).CheckWithContext(req, engine.Context{Rules: rules.Context{Metrics: rules.Metrics{Loaded: true, Currency: req.Payload.InstructedAmount.Currency, DebtorAmountSum: "0.1"}}})
	fmt.Println(check.Decision, check.Reasons)
}
