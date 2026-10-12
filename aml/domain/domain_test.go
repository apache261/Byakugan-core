package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestAMLDecimalPolicyRemainsIndependent(t *testing.T) {
	for _, value := range []string{"0", "0.000000000000000001", strings.Repeat("9", 38), "001.2300"} {
		parsed, err := ParseDecimal(value)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(parsed)
		if err != nil {
			t.Fatal(err)
		}
		var roundTrip Decimal
		if err := json.Unmarshal(raw, &roundTrip); err != nil || roundTrip != parsed {
			t.Fatalf("decimal round trip %s %v", raw, err)
		}
	}
	for _, value := range []string{"-1", "+1", "1e2", "1/2", "NaN", " 1", "1.", strings.Repeat("9", 39), "0." + strings.Repeat("1", 19)} {
		if _, err := ParseDecimal(value); err == nil {
			t.Fatalf("invalid decimal accepted: %q", value)
		}
	}
	for _, raw := range []string{`0.1`, `null`} {
		var value Decimal
		if err := json.Unmarshal([]byte(raw), &value); err == nil {
			t.Fatalf("numeric/null money accepted: %s", raw)
		}
	}
	// AML assets do not consult fraud's fiat currency catalog or two-digit scales.
	if err := (Money{Amount: MustDecimal("0.000000000000000001"), AssetCode: "SYNTHETIC", AssetType: AssetOther}).Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestTransactionEvaluationValidationHasNoPersistenceFields(t *testing.T) {
	now := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	transaction := Transaction{ID: "synthetic", Source: SourceIdentity{SourceSystem: "synthetic", ExternalID: "external", SourceEventAt: now}, EventAt: now, Kind: TransactionPayment, Direction: DirectionOutbound, Status: TransactionPosted, Money: Money{Amount: MustDecimal("0.00001"), AssetCode: "USD"}, Subjects: []SubjectReference{{Type: SubjectCustomer, ID: "synthetic-customer"}}}
	if err := transaction.Validate(); err != nil {
		t.Fatal(err)
	}
	transaction.Subjects = append(transaction.Subjects, transaction.Subjects[0])
	if err := transaction.Validate(); err == nil {
		t.Fatal("duplicate subject accepted")
	}
	transaction.Subjects = transaction.Subjects[:1]
	transaction.Money.Amount = Decimal("1.00")
	if err := transaction.Validate(); err == nil {
		t.Fatal("noncanonical amount accepted")
	}
}

func TestScenarioValidationBoundsCompiledCycles(t *testing.T) {
	node := ScenarioNode{}
	node.Not = &node
	version := ScenarioVersion{ScenarioID: "cycle", Version: 1, SchemaVersion: 1, Typology: "test", ExecutionModes: []string{"realtime"}, SubjectBasis: SubjectCustomer, WindowSeconds: 1, LateDataBehavior: "observe", Condition: node, Severity: "low", RiskScore: MustDecimal("1"), Author: "test"}
	if _, err := PrepareScenarioVersion(version); err == nil {
		t.Fatal("cyclic Go DSL accepted")
	}
	if version.RealtimeSafe() {
		t.Fatal("cyclic Go DSL is realtime safe")
	}
}
