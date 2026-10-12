package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExactCurrencyAmounts(t *testing.T) {
	tests := []struct{ value, currency, want string }{
		{"0001.20", "PHP", "1.2"}, {"999999999999999999.99", "USD", "999999999999999999.99"},
		{"1", "JPY", "1"}, {"0.001", "KWD", "0.001"}, {"0.01", "PHP", "0.01"},
	}
	for _, tc := range tests {
		got, err := CanonicalAmount(tc.value, tc.currency)
		if err != nil || got != tc.want {
			t.Fatalf("%+v: %q %v", tc, got, err)
		}
	}
	for _, tc := range []struct{ value, currency string }{
		{"0", "PHP"}, {"0.00", "PHP"}, {"-1", "PHP"}, {"+1", "PHP"}, {"1e2", "PHP"},
		{"NaN", "PHP"}, {"Inf", "PHP"}, {" 1", "PHP"}, {"1.", "PHP"}, {".1", "PHP"},
		{"1.001", "PHP"}, {"1.0", "JPY"}, {"1.0001", "KWD"}, {"1", "ZZZ"},
		{"1", "php"}, {"1000000000000000000", "USD"}, {"1,000", "USD"}, {"", "USD"},
	} {
		if _, err := CanonicalAmount(tc.value, tc.currency); err == nil {
			t.Fatalf("accepted %+v", tc)
		}
	}
}

func TestExactAggregationAndComparison(t *testing.T) {
	for _, tc := range []struct {
		values []string
		want   string
	}{
		{[]string{"0.1", "0.2"}, "0.3"},
		{[]string{"999999999999999999.99", "0.01"}, "1000000000000000000"},
		{nil, "0"},
	} {
		got, err := AddAmounts("USD", tc.values...)
		if err != nil || got != tc.want {
			t.Fatalf("sum %v = %s %v", tc.values, got, err)
		}
	}
	if cmp, err := CompareAmounts("999999999999999999.99", "999999999999999999.98", "USD"); err != nil || cmp != 1 {
		t.Fatalf("comparison %d %v", cmp, err)
	}
	if cmp, err := CompareAmounts("1.20", "01.2", "PHP"); err != nil || cmp != 0 {
		t.Fatalf("comparison %d %v", cmp, err)
	}
	if _, err := AddAmounts("USD", strings.Repeat("9", 36), "1"); err == nil {
		t.Fatal("aggregate overflow accepted")
	}
	if _, err := AddAmounts("JPY", "0.01"); err == nil {
		t.Fatal("wrong currency scale accepted")
	}
	if _, err := AddAmounts("USD", "invalid"); err == nil {
		t.Fatal("invalid aggregate accepted")
	}
}

func TestMoneyJSONRetainsPrecisionAndRejectsNumbers(t *testing.T) {
	var money Money
	if err := json.Unmarshal([]byte(`{"amount":"999999999999999999.99","currency":"USD"}`), &money); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(money)
	if err != nil || !strings.Contains(string(raw), `"999999999999999999.99"`) {
		t.Fatalf("precision lost: %s %v", raw, err)
	}
	if err := json.Unmarshal([]byte(`{"amount":0.1,"currency":"USD"}`), &money); err == nil {
		t.Fatal("JSON number accepted")
	}
}
